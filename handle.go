package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/hoisie/mustache"
	"github.com/kballard/go-shellquote"
	"github.com/tidwall/gjson"
)

func handle(upd tgbotapi.Update) {
	if upd.Message != nil {
		handleMessage(upd.Message)
	} else if upd.CallbackQuery != nil {
		handleCallback(upd.CallbackQuery)
	} else if upd.InlineQuery != nil {
		handleInlineQuery(upd.InlineQuery)
	} else if upd.EditedMessage != nil {
	}
}

func handleMessage(message *tgbotapi.Message) {
	u, err := ensureUser(message.From.ID, message.From.UserName)
	if err != nil {
		log.Warn().Err(err).
			Str("username", message.From.UserName).
			Int("id", message.From.ID).
			Msg("failed to ensure user")
		return
	}

	var (
		opts    = make(docopt.Opts)
		proceed = false
		text    = regexp.MustCompile("/([a-z]+)@"+s.ServiceId).ReplaceAllString(message.Text, "/$1")
	)

	log.Debug().Str("t", text).Str("user", u.Username).Msg("got message")

	// when receiving a forwarded invoice (from messages from other people?)
	// or just the full text of a an invoice (shared from a phone wallet?)
	if !strings.HasPrefix(text, "/") {
		if bolt11, ok := searchForInvoice(*message); ok {
			opts, _, _ = parse("/pay " + bolt11)
			goto parsed
		}
	}

	// individual transaction query
	if strings.HasPrefix(text, "/txn") {
		hashfirstchars := text[4:]
		txn, err := u.getTransaction(hashfirstchars)
		if err != nil {
			log.Warn().Err(err).Str("user", u.Username).Str("hash", hashfirstchars).
				Msg("failed to get transaction")
			return
		}

		u.notify(mustache.Render(`
{{Status}} {{#TelegramPeer}}{{PeerActionDescription}}{{/TelegramPeer}} on {{TimeFormat}}
_{{Description}}_
{{^TelegramPeer}}*Hash*: {{Hash}}{{/TelegramPeer}}
{{#HasPreimage}}*Preimage*: {{Preimage}}{{/HasPreimage}}
*Amount*: {{Satoshis}} satoshis
{{#Fees}}*Fee paid*: {{FeeSatoshis}}{{/Fees}}
        `, txn))
		return
	}

	// otherwise parse the slash command
	opts, proceed, err = parse(text)
	if !proceed {
		return
	}
	if err != nil {
		log.Warn().Err(err).Str("command", text).
			Msg("Failed to parse command")
		u.notify("Could not understand the command.")
		return
	}

parsed:
	if opts["paynow"].(bool) {
		opts["pay"] = true
		opts["now"] = true
	}

	switch {
	case opts["start"].(bool):
		// create user
		if message.Chat.Type == "private" {
			u.setChat(message.Chat.ID)
			u.notify("Account created successfully.")
		}

		break
	case opts["receive"].(bool), opts["invoice"].(bool):
		sats, err := opts.Int("<amount>")
		if err != nil {
			u.notify("Invalid amount: " + opts["<amount>"].(string))
			break
		}
		var desc string
		if idesc, ok := opts["<description>"]; ok {
			desc = strings.Join(idesc.([]string), " ")
		}

		label := makeLabel(u.ChatId, message.MessageID)

		bolt11, qrpath, err := makeInvoice(u, label, sats, desc)
		if err != nil {
			notify(message.Chat.ID, "Failed to generate invoice.")
			return
		}

		if qrpath == "" {
			u.notify(bolt11)
		} else {
			defer os.Remove(qrpath)
			photo := tgbotapi.NewPhotoUpload(message.Chat.ID, qrpath)
			photo.Caption = bolt11
			_, err := bot.Send(photo)
			if err != nil {
				log.Warn().Str("user", u.Username).Err(err).
					Msg("error sending photo")

					// send just the bolt11
				notify(message.Chat.ID, bolt11)
			}
		}

		break
	case opts["decode"].(bool):
		// just decode invoice
		bolt11 := opts["<invoice>"].(string)
		handleDecodeInvoice(message.Chat.ID, bolt11, 0)
		break
	case opts["send"].(bool):
		break
	case opts["giveaway"].(bool):
		if message.Chat.Type == "group" {
			sats, err := opts.Int("<satoshis>")
			if err != nil {
				break
			}

			chattable := tgbotapi.NewMessage(
				message.Chat.ID,
				fmt.Sprintf("%s is giving %d satoshis away!", u.Username, sats),
			)
			chattable.BaseChat.ReplyMarkup = giveAwayKeyboard(u, sats)
			bot.Send(chattable)
		} else {
			u.notify("This must be called in a group.")
		}
		break
	case opts["transactions"].(bool):
		// show list of transactions
		txns, err := u.listTransactions()
		if err != nil {
			log.Warn().Err(err).Str("user", u.Username).
				Msg("failed to list transactions")
			break
		}

		u.notify(mustache.Render(`
{{#txns}}
`+"`{{PaddedStatus}}`"+` `+"`{{PaddedSatoshis}}`"+`{{#TelegramPeer}} {{PeerActionDescription}}{{/TelegramPeer}} {{TimeFormatSmall}} /txn{{HashReduced}}
{{/txns}}
        `, map[string][]Transaction{"txns": txns}))
		break
	case opts["balance"].(bool):
		// show balance
		info, err := u.getInfo()
		if err != nil {
			log.Warn().Err(err).Str("user", u.Username).Msg("failed to get info")
			break
		}

		u.notify(fmt.Sprintf(`
*Balance*: %.3f satoshis
*Total received*: %.3f satoshis
*Total sent*: %.3f satoshis
*Total fees paid*: %.3f satoshis
        `, info.Balance, info.TotalReceived, info.TotalSent, info.TotalFees))
		break
	case opts["pay"].(bool):
		// pay invoice
		askConfirmation := true
		if opts["now"].(bool) {
			askConfirmation = false
		}

		var bolt11 string
		// when paying, the invoice could be in the message this is replying to
		if ibolt11, ok := opts["<invoice>"]; !ok || ibolt11 == nil {
			if message.ReplyToMessage != nil {
				bolt11, ok = searchForInvoice(*message.ReplyToMessage)
				if !ok {
					u.notify("Invoice not provided.")
					break
				}
			}

			u.notify("Invoice not provided.")
			break
		} else {
			bolt11 = ibolt11.(string)
		}

		optsats, _ := opts.Int("<satoshis>")
		optmsats := optsats * 1000

		invlabel := makeLabel(u.ChatId, message.MessageID)

		if askConfirmation {
			// decode invoice and show a button for confirmation
			message := handleDecodeInvoice(u.ChatId, bolt11, optmsats)

			rds.Set("payinvoice:"+invlabel, bolt11, s.PayConfirmTimeout)
			rds.Set("payinvoice:"+invlabel+":msats", optmsats, s.PayConfirmTimeout)

			bot.Send(tgbotapi.NewEditMessageText(u.ChatId, message.MessageID,
				message.Text+"\n\nPay the invoice described above?"))
			bot.Send(tgbotapi.NewEditMessageReplyMarkup(u.ChatId, message.MessageID,
				tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("Cancel", "cancel"),
						tgbotapi.NewInlineKeyboardButtonData("Yes", "pay="+invlabel),
					),
				),
			))
		} else {
			handlePayInvoice(u, bolt11, invlabel, optmsats)
		}
	case opts["help"].(bool):
		notify(message.Chat.ID, strings.Replace(s.Usage, "  c ", "  /", -1))
	}
}

func handleCallback(cb *tgbotapi.CallbackQuery) {
	switch {
	case cb.Data == "noop":
		goto answerEmpty
	case cb.Data == "cancel":
		removeKeyboardButtons(cb)
		goto answerEmpty
	case strings.HasPrefix(cb.Data, "pay="):
		u, err := ensureUser(cb.From.ID, cb.From.UserName)
		if err != nil {
			log.Warn().Err(err).
				Str("username", cb.From.UserName).
				Int("id", cb.From.ID).
				Msg("failed to ensure user")
			goto answerEmpty
		}

		invlabel := cb.Data[4:]
		bolt11, err := rds.Get("payinvoice:" + invlabel).Result()
		if err != nil {
			bot.AnswerCallbackQuery(
				tgbotapi.NewCallback(
					cb.ID,
					"The payment confirmation button has expired.",
				),
			)
			goto answerEmpty
		}

		bot.AnswerCallbackQuery(
			tgbotapi.NewCallback(cb.ID, "Sending payment."),
		)

		optmsats, _ := rds.Get("payinvoice:" + invlabel + ":msats").Int64()
		handlePayInvoice(u, bolt11, invlabel, int(optmsats))
		removeKeyboardButtons(cb)
		appendTextToMessage(cb, " Paid.")
		return
	case strings.HasPrefix(cb.Data, "send="):
		params := strings.Split(cb.Data[5:], "-")
		if len(params) != 3 {
			goto answerEmpty
		}

		fromid, err1 := strconv.Atoi(params[0])
		sats, err2 := strconv.Atoi(params[1])
		toname := params[2]
		if err1 != nil || err2 != nil || toname == "" || toname == "0" {
			goto answerEmpty
		}

		u, err := loadUser(fromid, 0)
		if err != nil {
			log.Warn().Err(err).
				Int("id", fromid).
				Msg("failed to load user")
			goto answerEmpty
		}

		var target User
		if toid, err := strconv.Atoi(toname); err == nil {
			target, err = ensureTelegramId(toid)
		} else {
			target, err = ensureUsername(toname)
		}
		if err != nil {
			log.Warn().Err(err).
				Msg("failed to ensure target user after send/tip.")
			goto answerEmpty
		}

		errMsg, err := u.sendInternally(target, sats*1000, "send", nil)
		if err != nil {
			log.Warn().Err(err).
				Msg("failed to send/tip")
			u.notify("Failed to send: " + errMsg)
			goto answerEmpty
		}
		bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, "Payment sent."))
		removeKeyboardButtons(cb)
		target.notify(fmt.Sprintf("%s has sent you %d satoshis.", u.Username, sats))

		return
	case strings.HasPrefix(cb.Data, "give="):
		params := strings.Split(cb.Data[5:], "-")
		if len(params) != 3 {
			goto answerEmpty
		}

		buttonData := rds.Get("giveaway:" + params[2]).Val()
		if buttonData != cb.Data {
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, " Giveaway expired.")
			goto answerEmpty
		}
		if err = rds.Del("giveaway:" + params[2]).Err(); err != nil {
			log.Warn().Err(err).Str("id", params[2]).
				Msg("error deleting giveaway check from redis")
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, " Giveaway error.")
			goto answerEmpty
		}

		fromid, err1 := strconv.Atoi(params[0])
		sats, err2 := strconv.Atoi(params[1])
		if err1 != nil || err2 != nil {
			goto answerEmpty
		}

		u, err := loadUser(fromid, 0)
		if err != nil {
			log.Warn().Err(err).
				Int("id", fromid).
				Msg("failed to load user")
			goto answerEmpty
		}

		claimer, err := ensureUser(cb.From.ID, cb.From.UserName)
		if err != nil {
			log.Warn().Err(err).
				Msg("failed to ensure claimer user on giveaway.")
			goto answerEmpty
		}

		errMsg, err := u.sendInternally(claimer, sats*1000, "giveaway", nil)
		if err != nil {
			log.Warn().Err(err).Msg("failed to give away")
			claimer.notify("Failed to claim giveaway: " + errMsg)
			goto answerEmpty
		}
		bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, "Payment sent."))
		removeKeyboardButtons(cb)
		claimer.notify(fmt.Sprintf("%s has sent you %d satoshis.", u.Username, sats))
		appendTextToMessage(cb, " Given successfully.")
		return
	}

answerEmpty:
	bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, ""))
}

func handleInlineQuery(q *tgbotapi.InlineQuery) {
	var (
		u    User
		err  error
		resp tgbotapi.APIResponse
		argv []string
		text string
	)

	u, err = loadUser(0, int(q.From.ID))
	if err != nil {
		log.Debug().Err(err).
			Str("username", q.From.UserName).
			Int("id", q.From.ID).
			Msg("unregistered user trying to use inline query")

		resp, err = bot.AnswerInlineQuery(tgbotapi.InlineConfig{
			InlineQueryID:     q.ID,
			Results:           []interface{}{},
			SwitchPMText:      "Create an account first",
			SwitchPMParameter: "1",
		})
		goto responded
	}

	text = strings.TrimSpace(q.Query)
	argv, err = shellquote.Split(text)
	if err != nil || len(argv) < 2 {
		goto answerEmpty
	}

	switch argv[0] {
	case "invoice", "receive":
		label := makeLabel(u.ChatId, q.ID)

		sats, err := strconv.Atoi(argv[1])
		if err != nil {
			goto answerEmpty
		}

		bolt11, qrpath, err := makeInvoice(u, label, sats, "inline-"+q.ID)
		if err != nil {
			log.Warn().Err(err).Msg("error making invoice on inline query.")
			goto answerEmpty
		}

		qrurl := s.ServiceURL + "/qr/" + qrpath

		result := tgbotapi.NewInlineQueryResultPhoto("inv-"+argv[1], qrurl)
		result.Title = argv[1] + " satoshis"
		result.Description = "Payment request for " + argv[1] + " satoshis"
		result.ThumbURL = qrurl
		result.Caption = bolt11

		resp, err = bot.AnswerInlineQuery(tgbotapi.InlineConfig{
			InlineQueryID: q.ID,
			Results:       []interface{}{result},
			IsPersonal:    true,
		})

		go func(qrpath string) {
			time.Sleep(30 * time.Second)
			os.Remove(qrpath)
		}(qrpath)
		goto responded
	case "giveaway":
		if len(argv) != 2 {
			goto answerEmpty
		}

		if sats, err := strconv.Atoi(argv[1]); err == nil {
			result := tgbotapi.NewInlineQueryResultArticle(
				fmt.Sprintf("give-%d-%d", u.Id, sats),
				fmt.Sprintf("Giving %d away", sats),
				fmt.Sprintf("%s is giving %d satoshis away!", u.Username, sats),
			)

			keyboard := giveAwayKeyboard(u, sats)
			result.ReplyMarkup = &keyboard

			resp, err = bot.AnswerInlineQuery(tgbotapi.InlineConfig{
				InlineQueryID: q.ID,
				Results:       []interface{}{result},
				IsPersonal:    true,
			})
		}
	case "tip", "send":
		var (
			sats        int
			username    string
			userid      int
			displayname string
		)

		switch len(argv) {
		case 3:
			if argv[1][0] != '@' {
				goto answerEmpty
			}

			sats, err = strconv.Atoi(argv[2])
			if err != nil {
				goto answerEmpty
			}
			username = argv[1][1:]
			displayname = username
		case 4:
			if argv[1][0] != '@' {
				goto answerEmpty
			}
			userid, err = strconv.Atoi(argv[1][1:])
			if err != nil {
				goto answerEmpty
			}

			if strings.HasPrefix(argv[2], "(") && strings.HasSuffix(argv[2], ")") {
				displayname = argv[2][1 : len(argv[2])-1]
			} else {
				goto answerEmpty
			}

			sats, err = strconv.Atoi(argv[3])
			if err != nil {
				goto answerEmpty
			}

		default:
			goto answerEmpty
		}

		result := tgbotapi.NewInlineQueryResultArticle(
			fmt.Sprintf("pay-%d-%s-%d", sats, username, userid),
			fmt.Sprintf("Send %d to %s", sats, displayname),
			fmt.Sprintf("Sending %d from %s to %s.", sats, u.Username, displayname),
		)

		buttonData := fmt.Sprintf("send=%d-%d-", u.Id, sats)
		if username != "" {
			buttonData += username
		} else {
			buttonData += strconv.Itoa(userid)
		}
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Cancel", "cancel"),
				tgbotapi.NewInlineKeyboardButtonData("Confirm", buttonData),
			),
		)
		result.ReplyMarkup = &keyboard

		resp, err = bot.AnswerInlineQuery(tgbotapi.InlineConfig{
			InlineQueryID: q.ID,
			Results:       []interface{}{result},
			IsPersonal:    true,
		})

		goto responded
	default:
		goto answerEmpty
	}

responded:
	if err != nil || !resp.Ok {
		log.Warn().Err(err).
			Str("resp", resp.Description).
			Msg("error answering inline query")
	}
	return

answerEmpty:
	bot.AnswerInlineQuery(tgbotapi.InlineConfig{
		InlineQueryID: q.ID,
		Results:       []interface{}{},
	})
}

func handleDecodeInvoice(chatId int64, bolt11 string, optmsats int) (_ tgbotapi.Message) {
	inv, err := decodeInvoice(bolt11)
	if err != nil {
		notify(chatId, "Invalid bolt11: "+err.Error())
		return
	}

	amount := int(inv.Get("msatoshi").Int())
	if amount == 0 {
		amount = optmsats
	}

	return notify(chatId,
		fmt.Sprintf("[%s] \n%d satoshis. \nhash: %s.",
			inv.Get("description").String(),
			amount/1000,
			inv.Get("payment_hash").String()),
	)
}

func handlePayInvoice(u User, bolt11, invlabel string, optmsats int) {
	// check if this is an internal invoice (it will have a different label)
	if label, err := rds.Get("recinvoice.internal:" + bolt11).Result(); err == nil && label != "" {
		// this is an internal invoice. do not pay.
		// delete it and just transfer balance.
		rds.Del("recinvoice.internal:" + bolt11)
		ln.Call("delinvoice", label, "unpaid")

		targetId, err := rds.Get("recinvoice:" + label + ":creator").Int64()
		if err != nil {
			log.Warn().Err(err).
				Str("label", label).
				Msg("failed to get internal invoice target from redis")
			u.notify("Failed to find invoice payee")
			return
		}
		target, err := loadUser(int(targetId), 0)
		if err != nil {
			log.Warn().Err(err).
				Str("label", label).
				Int64("id", targetId).
				Msg("failed to get load internal invoice target from postgres")
			u.notify("Failed to find invoice payee")
			return
		}

		amount, hash, errMsg, err := u.payInternally(
			target, bolt11, label, optmsats)
		if err != nil {
			log.Warn().Err(err).
				Str("label", label).
				Msg("failed to pay pay internally")
			u.notify("Failed to pay: " + errMsg)
			return
		}

		// internal payment succeeded
		target.notifyAsReply(
			fmt.Sprintf("Payment received: %d. \n\nhash: %s.", amount/1000, hash),
			messageIdFromLabel(label),
		)

		return
	}

	pay, errMsg, err := u.payInvoice(bolt11, invlabel, optmsats)
	if err != nil {
		log.Warn().Err(err).
			Str("user", u.Username).
			Str("bolt11", bolt11).
			Str("label", invlabel).
			Msg("couldn't pay invoice")
		u.notify("Failed to pay: " + errMsg)
		return
	}

	log.Print("paid: " + pay.String())

	u.notify(fmt.Sprintf(
		"Paid with %d satoshis (+ %.3f fee). \n\nHash: %s\n\nProof: %s\n\n/txn%s",
		int(pay.Get("msatoshi").Float()/1000),
		(pay.Get("msatoshi_sent").Float()-pay.Get("msatoshi").Float())/1000,
		pay.Get("payment_hash").String(),
		pay.Get("payment_preimage").String(),
		pay.Get("payment_hash").String()[:5],
	))
}

func handleGiveAway(u User, sats int) {

}

func handleInvoicePaid(res gjson.Result) {
	index := res.Get("pay_index").Int()
	rds.Set("lastinvoiceindex", index, 0)

	label := res.Get("label").String()

	// use the label to get the user that created this invoice
	userId, _ := rds.Get("recinvoice:" + label + ":creator").Int64()
	u, err := loadUser(int(userId), 0)
	if err != nil {
		log.Warn().Err(err).
			Int64("userid", userId).Str("label", label).Int64("index", index).
			Msg("couldn't load user who created this invoice.")
		return
	}

	msats := res.Get("msatoshi_received").Int()
	desc := res.Get("description").String()
	hash := res.Get("payment_hash").String()

	err = u.paymentReceived(
		int(msats),
		desc,
		hash,
		label,
	)
	if err != nil {
		u.notify(
			"Payment received, but failed to save on database. Please report this issue: " + label + ".",
		)
	}

	u.notifyAsReply(
		fmt.Sprintf("Payment received: %d. \n\nhash: %s.", msats/1000, hash),
		messageIdFromLabel(label),
	)
}

func removeKeyboardButtons(cb *tgbotapi.CallbackQuery) {
	baseEdit := getBaseEdit(cb)

	baseEdit.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			[]tgbotapi.InlineKeyboardButton{},
		},
	}

	bot.Send(tgbotapi.EditMessageReplyMarkupConfig{
		BaseEdit: baseEdit,
	})
}

func appendTextToMessage(cb *tgbotapi.CallbackQuery, text string) {
	baseEdit := getBaseEdit(cb)

	bot.Send(tgbotapi.EditMessageTextConfig{
		BaseEdit: baseEdit,
		Text:     cb.Message.Text + text,
	})
}
