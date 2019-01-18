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

	// after ensuring the user we should always enable him to
	// receive payment notifications and so on, as not all people will
	// remember to call /start
	if message.Chat.Type == "private" {
		u.setChat(message.Chat.ID)
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
	if strings.HasPrefix(text, "/tx") {
		hashfirstchars := text[3:]
		txn, err := u.getTransaction(hashfirstchars)
		if err != nil {
			log.Warn().Err(err).Str("user", u.Username).Str("hash", hashfirstchars).
				Msg("failed to get transaction")
			return
		}

		u.notify(mustache.Render(`
`+"`{{Status}}`"+` {{#TelegramPeer}}{{PeerActionDescription}}{{/TelegramPeer}} on {{TimeFormat}}
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
			u.notify("Account created.")
			handleHelp(message)
		}

		break
	case opts["receive"].(bool), opts["invoice"].(bool):
		sats, err := opts.Int("<satoshis>")
		if err != nil {
			u.notify("Invalid amount: " + opts["<satoshis>"].(string))
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
		decodeNotifyBolt11(message.Chat.ID, bolt11, 0)
		break
	case opts["send"].(bool), opts["tip"].(bool):
		sats, err := opts.Int("<satoshis>")
		if err != nil {
			u.notify("Invalid amount: " + opts["<satoshis>"].(string))
			break
		}

		var (
			todisplayname string
			receiver      User
		)
		if opts["<username>"] != nil {
			toname := opts["<username>"].(string)
			if toname[0] != '@' {
				u.notify("Target user name should begin with a `@`.")
				break
			}
			toname = toname[1:]
			todisplayname = toname
			receiver, err = ensureUsername(toname)
		} else if opts["<userid>"] != nil && opts["<displayname>"] != nil {
			todisplayname = opts["<displayname>"].(string)
			toid := opts["<username>"].(string)
			if toid[0] != '@' {
				u.notify("Target user id should begin with a `@`.")
				break
			}
			toid = toid[1:]
			toidint, err := strconv.Atoi(toid)
			if err != nil {
				u.notify("Target user id should be a number, not `" + toid + "`.")
				break
			}
			receiver, err = ensureTelegramId(toidint)
		} else {
			break
		}

		if err != nil {
			log.Warn().Err(err).
				Msg("failed to ensure target user on send/tip.")
			u.notify("Failed to save receiver.")
			break
		}

		errMsg, err := u.sendInternally(receiver, sats*1000, nil, nil)
		if err != nil {
			log.Warn().Err(err).
				Str("from", u.Username).
				Str("to", todisplayname).
				Msg("failed to send/tip")
			u.notify("Failed to send: " + errMsg)
			break
		}

		receiver.notify(fmt.Sprintf("%s has sent you %d satoshis.", u.AtName(), sats))

		if message.Chat.Type == "private" {
			u.notifyAsReply(
				fmt.Sprintf("%d satoshis sent to %s.", sats, todisplayname),
				message.MessageID,
			)
		} else {
			u.notify(fmt.Sprintf("%d satoshis sent to %s.", sats, todisplayname))
		}

		break
	case opts["giveaway"].(bool):
		if message.Chat.Type == "group" {
			sats, err := opts.Int("<satoshis>")
			if err != nil {
				u.notify("Invalid amount: " + opts["<satoshis>"].(string))
				break
			}

			chattable := tgbotapi.NewMessage(
				message.Chat.ID,
				fmt.Sprintf("%s is giving %d satoshis away!", u.AtName(), sats),
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

		u.notify(mustache.Render(`*Latest transactions*
{{#txns}}
`+"{{StatusSmall}}"+` `+"`{{PaddedSatoshis}}`"+`{{#TelegramPeer}} {{PeerActionDescription}}{{/TelegramPeer}} _{{TimeFormatSmall}}_ /tx{{HashReduced}}
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
			message := decodeNotifyBolt11(u.ChatId, bolt11, optmsats)

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
			payInvoice(u, bolt11, invlabel, optmsats)
		}
		break
	case opts["help"].(bool):
		handleHelp(message)
		break
	}
}

func handleCallback(cb *tgbotapi.CallbackQuery) {
	switch {
	case cb.Data == "noop":
		goto answerEmpty
	case cb.Data == "cancel":
		removeKeyboardButtons(cb)
		appendTextToMessage(cb, "Canceled.")
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
		paid, mayRetry := payInvoice(u, bolt11, invlabel, int(optmsats))
		if paid {
			appendTextToMessage(cb, "Paid.")
			removeKeyboardButtons(cb)
		} else if mayRetry {
			removeKeyboardButtons(cb)
		}
		return
	case strings.HasPrefix(cb.Data, "give="):
		params := strings.Split(cb.Data[5:], "-")
		if len(params) != 3 {
			goto answerEmpty
		}

		buttonData := rds.Get("giveaway:" + params[2]).Val()
		if buttonData != cb.Data {
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, "Giveaway expired.")
			goto answerEmpty
		}
		if err = rds.Del("giveaway:" + params[2]).Err(); err != nil {
			log.Warn().Err(err).Str("id", params[2]).
				Msg("error deleting giveaway check from redis")
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, "Giveaway error.")
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
		claimer.notify(fmt.Sprintf("%s has sent you %d satoshis.", u.AtName(), sats))
		appendTextToMessage(cb, fmt.Sprintf("%d satoshis given from %s to %s.", sats, u.AtName(), claimer.AtName()))
		return
	}

answerEmpty:
	bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, ""))
}

func handleHelp(message *tgbotapi.Message) {
	notify(message.Chat.ID, strings.Replace(s.Usage, "  c ", "  /", -1))
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

		goto answerEmpty
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
				fmt.Sprintf("%s is giving %d satoshis away!", u.AtName(), sats),
			)

			keyboard := giveAwayKeyboard(u, sats)
			result.ReplyMarkup = &keyboard

			resp, err = bot.AnswerInlineQuery(tgbotapi.InlineConfig{
				InlineQueryID: q.ID,
				Results:       []interface{}{result},
				IsPersonal:    true,
			})
		}
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

func decodeNotifyBolt11(chatId int64, bolt11 string, optmsats int) (_ tgbotapi.Message) {
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

func payInvoice(u User, bolt11, label string, optmsats int) (paid, mayRetry bool) {
	// check if this is an internal invoice (it will have a different label)
	intlabel, err := rds.Get("recinvoice.internal:" + bolt11).Result()
	if err == nil && intlabel != "" {
		// this is an internal invoice. do not pay.
		// delete it and just transfer balance.
		rds.Del("recinvoice.internal:" + bolt11)
		ln.Call("delinvoice", intlabel, "unpaid")

		targetId, err := rds.Get("recinvoice:" + intlabel + ":creator").Int64()
		if err != nil {
			log.Warn().Err(err).
				Str("intlabel", intlabel).
				Msg("failed to get internal invoice target from redis")
			u.notify("Failed to find invoice payee.")
			return false, false
		}
		target, err := loadUser(int(targetId), 0)
		if err != nil {
			log.Warn().Err(err).
				Str("intlabel", intlabel).
				Int64("id", targetId).
				Msg("failed to get load internal invoice target from postgres")
			u.notify("Failed to find invoice payee")
			return false, false
		}

		amount, hash, errMsg, mayRetry, err := u.payInternally(
			target,
			bolt11,
			intlabel,
			optmsats,
		)
		if err != nil {
			log.Warn().Err(err).
				Str("intlabel", intlabel).
				Msg("failed to pay pay internally")
			u.notify("Failed to pay: " + errMsg)

			return false, mayRetry
		}

		// internal payment succeeded
		target.notifyAsReply(
			fmt.Sprintf("Payment received: %d. \n\nhash: %s.", amount/1000, hash),
			messageIdFromLabel(intlabel),
		)

		return true, false
	}

	pay, errMsg, mayRetry, err := u.payInvoice(bolt11, label, optmsats)
	if err != nil {
		log.Warn().Err(err).
			Str("user", u.Username).
			Str("bolt11", bolt11).
			Str("label", label).
			Msg("couldn't pay invoice")
		u.notify("Failed to pay: " + errMsg)

		return false, mayRetry
	}

	u.notify(fmt.Sprintf(
		"Paid with %d satoshis (+ %.3f fee). \n\nHash: %s\n\nProof: %s\n\n/txn%s",
		int(pay.Get("msatoshi").Float()/1000),
		(pay.Get("msatoshi_sent").Float()-pay.Get("msatoshi").Float())/1000,
		pay.Get("payment_hash").String(),
		pay.Get("payment_preimage").String(),
		pay.Get("payment_hash").String()[:5],
	))

	return true, false
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
	if cb.Message != nil {
		text = cb.Message.Text + " " + text
	}

	baseEdit := getBaseEdit(cb)
	bot.Send(tgbotapi.EditMessageTextConfig{
		BaseEdit: baseEdit,
		Text:     text,
	})
}
