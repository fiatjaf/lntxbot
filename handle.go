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
	u, t, err := ensureUser(message.From.ID, message.From.UserName)
	if err != nil {
		log.Warn().Err(err).Int("case", t).
			Str("username", message.From.UserName).
			Int("id", message.From.ID).
			Msg("failed to ensure user")
		return
	}

	if message.Chat.Type == "private" {
		// after ensuring the user we should always enable him to
		// receive payment notifications and so on, as not all people will
		// remember to call /start
		u.setChat(message.Chat.ID)
	} else if message.Entities == nil || len(*message.Entities) == 0 ||
		// unless in the private chat, only messages starting with
		// bot commands will work
		(*message.Entities)[0].Type != "bot_command" ||
		(*message.Entities)[0].Offset != 0 {
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
	if strings.HasPrefix(text, "/tx") {
		hashfirstchars := text[3:]
		txn, err := u.getTransaction(hashfirstchars)
		if err != nil {
			log.Warn().Err(err).Str("user", u.Username).Str("hash", hashfirstchars).
				Msg("failed to get transaction")
			u.notifyAsReply("Couldn't find transaction "+hashfirstchars+".", message.MessageID)
			return
		}

		text := mustache.Render(`
<code>{{Status}}</code> {{#TelegramPeer.Valid}}{{PeerActionDescription}}{{/TelegramPeer.Valid}} on {{TimeFormat}} {{#IsUnclaimed}}(💤 unclaimed){{/IsUnclaimed}}
<i>{{Description}}</i>{{^TelegramPeer.Valid}} 
<b>Hash</b>: {{Hash}}{{/TelegramPeer.Valid}}{{#HasPreimage}} 
<b>Preimage</b>: {{Preimage}}{{/HasPreimage}}
<b>Amount</b>: {{Satoshis}} satoshis
{{^IsReceive}}<b>Fee paid</b>: {{FeeSatoshis}}{{/IsReceive}}
        `, txn)
		id := u.notifyAsReply(text, txn.TriggerMessage).MessageID

		if txn.Status == "PENDING" {
			// allow people to cancel pending if they're old enough
			editWithKeyboard(u.ChatId, id, text+"\n\nRecheck pending payment?",
				tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("Yes", "check="+hashfirstchars),
					),
				),
			)
		}

		if txn.IsUnclaimed() {
			editWithKeyboard(u.ChatId, id, text+"\n\nRetract unclaimed tip?",
				tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("Yes", "remunc="+hashfirstchars),
					),
				),
			)
		}

		return
	}

	// otherwise parse the slash command
	opts, proceed, err = parse(text)
	if !proceed {
		return
	}
	if err != nil {
		if message.Chat.Type == "private" {
			// only tell we don't understand commands when in a private chat
			// because these commands we're not understanding
			// may be targeting other bots in a group, so we're spamming people.
			log.Warn().Err(err).Str("command", text).
				Msg("Failed to parse command")
			u.notify("Could not understand the command. /help")
		}
		return
	}

parsed:
	if opts["paynow"].(bool) {
		opts["pay"] = true
		opts["now"] = true
	}

	switch {
	case opts["start"].(bool):
		if message.Chat.Type == "private" {
			u.setChat(message.Chat.ID)
			u.notify("Your account is created.")
			handleHelp(u)
		}
		break
	case opts["stop"].(bool):
		if message.Chat.Type == "private" {
			u.unsetChat()
			u.notify("Notifications stopped.")
		}
		break
	case opts["receive"].(bool), opts["invoice"].(bool), opts["fund"].(bool):
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

		var preimage string
		if param, ok := opts["--preimage"]; ok {
			preimage, _ = param.(string)
		}

		bolt11, qrpath, err := makeInvoice(u, label, sats, desc, preimage)
		if err != nil {
			log.Warn().Err(err).Msg("failed to generate invoice")
			notify(message.Chat.ID, messageFromError(err, "Failed to generate invoice"))
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
		decodeNotifyBolt11(message.Chat.ID, message.MessageID, bolt11, 0)
		break
	case opts["send"].(bool), opts["tip"].(bool):
		// default notify function to use depending on many things
		defaultNotify := func(m string) { u.notify(m) }
		if message.Chat.Type == "private" {
			defaultNotify = func(m string) { u.notifyAsReply(m, message.MessageID) }
		} else if isSpammy(message.Chat.ID) {
			defaultNotify = func(m string) { notifyAsReply(message.Chat.ID, m, message.MessageID) }
		}

		// sending money to others
		var (
			sats          int
			todisplayname string
			receiver      User
			username      string
		)

		// get quantity
		sats, err := opts.Int("<satoshis>")

		if err != nil || sats <= 0 {
			// maybe the order of arguments is inverted
			if val, ok := opts["<satoshis>"].(string); ok && val[0] == '@' {
				// it seems to be
				if asats, ok := opts["<username>"].([]string); ok && len(asats) == 1 {
					sats, _ = strconv.Atoi(asats[0])
					username = strings.ToLower(val)
					goto gotusername
				}
			}

			defaultNotify("Invalid amount: " + opts["<satoshis>"].(string))
			break
		} else {
			if aval, ok := opts["<username>"].([]string); ok && len(aval) > 0 {
				// got a username
				username = strings.ToLower(strings.Join(aval, " "))
				goto gotusername
			}
		}

	gotusername:
		// check entities for user type
		for _, entity := range *message.Entities {
			if entity.Type == "text_mention" && entity.User != nil {
				// user without username
				toid := entity.User.ID
				todisplayname = strings.TrimSpace(
					entity.User.FirstName + " " + entity.User.LastName,
				)
				receiver, err = ensureTelegramId(toid)
				goto ensured
			}
			if entity.Type == "mention" {
				// user with username
				toname := username[1:]
				todisplayname = toname
				receiver, err = ensureUsername(toname)
				goto ensured
			}
		}

		// no username, this may be a reply-tip
		if message.ReplyToMessage != nil {
			reply := message.ReplyToMessage

			var t int
			receiver, t, err = ensureUser(reply.From.ID, reply.From.UserName)
			if err != nil {
				log.Warn().Err(err).Int("case", t).
					Str("username", reply.From.UserName).
					Int("id", reply.From.ID).
					Msg("failed to ensure user on reply-tip")
				break
			}
			if reply.From.UserName != "" {
				todisplayname = "@" + reply.From.UserName
			} else {
				todisplayname = strings.TrimSpace(
					reply.From.FirstName + " " + reply.From.LastName,
				)
			}
			goto ensured
		}

		// if we ever reach this point then it's because the receiver is missing.
		defaultNotify("Can't send " + opts["<satoshis>"].(string) + ". Missing receiver!")
		break

	ensured:
		if err != nil {
			log.Warn().Err(err).
				Msg("failed to ensure target user on send/tip.")
			defaultNotify("Failed to save receiver. This is probably a bug.")
			break
		}

		errMsg, err := u.sendInternally(message.MessageID, receiver, sats*1000, nil, nil)
		if err != nil {
			log.Warn().Err(err).
				Str("from", u.Username).
				Str("to", todisplayname).
				Msg("failed to send/tip")
			defaultNotify("Failed to send: " + errMsg)
			break
		}

		if receiver.ChatId != 0 {
			receiver.notify(fmt.Sprintf("%s has sent you %d satoshis.", u.AtName(), sats))
		}

		if message.Chat.Type == "private" {
			warning := ""
			if receiver.ChatId == 0 {
				warning = fmt.Sprintf(
					" (couldn't notify %s as they haven't started a conversation with the bot)",
					todisplayname,
				)
			}
			u.notifyAsReply(
				fmt.Sprintf("%d satoshis sent to %s%s.", sats, todisplayname, warning),
				message.MessageID,
			)
			break
		}

		defaultNotify(fmt.Sprintf("%d satoshis sent to %s.", sats, todisplayname))
		break
	case opts["giveaway"].(bool):
		sats, err := opts.Int("<satoshis>")
		if err != nil {
			u.notify("Invalid amount: " + opts["<satoshis>"].(string))
			break
		}
		if info, err := u.getInfo(); err != nil || int(info.Balance) < sats {
			u.notify(fmt.Sprintf("Insufficient balance for the giveaway. Needs %.3f more satoshis",
				float64(sats)-info.Balance))
			break
		}

		chattable := tgbotapi.NewMessage(
			message.Chat.ID,
			fmt.Sprintf("%s is giving %d satoshis away!", u.AtName(), sats),
		)
		chattable.BaseChat.ReplyMarkup = giveAwayKeyboard(u, sats)
		bot.Send(chattable)
		break
	case opts["transactions"].(bool):
		// show list of transactions
		limit := 25
		offset := 0
		if page, err := opts.Int("--page"); err == nil {
			offset = limit * (page - 1)
		}

		txns, err := u.listTransactions(limit, offset)
		if err != nil {
			log.Warn().Err(err).Str("user", u.Username).
				Msg("failed to list transactions")
			break
		}

		title := fmt.Sprintf("Latest %d transactions", limit)
		if offset > 0 {
			title = fmt.Sprintf("Transactions from %d to %d", offset+1, offset+limit)
		}

		u.notify(mustache.Render(`<b>{{title}}</b>
{{#txns}}
<code>{{StatusSmall}}</code> <code>{{PaddedSatoshis}}</code> {{#TelegramPeer.Valid}}{{#IsUnclaimed}}💤 {{/IsUnclaimed}}{{PeerActionDescription}}{{/TelegramPeer.Valid}}{{^TelegramPeer.Valid}}{{^IsPending}}⚡{{/IsPending}}{{#IsPending}}🕒{{/IsPending}} <i>{{Description}}</i>{{/TelegramPeer.Valid}} <i>{{TimeFormatSmall}}</i> /tx{{HashReduced}}
{{/txns}}
        `, map[string]interface{}{"title": title, "txns": txns}))
		break
	case opts["balance"].(bool):
		// show balance
		info, err := u.getInfo()
		if err != nil {
			log.Warn().Err(err).Str("user", u.Username).Msg("failed to get info")
			break
		}

		u.notify(fmt.Sprintf(`
<b>Balance</b>: %.3f satoshis
<b>Total received</b>: %.3f satoshis
<b>Total sent</b>: %.3f satoshis
<b>Total fees paid</b>: %.3f satoshis
        `, info.Balance, info.TotalReceived, info.TotalSent, info.TotalFees))
		break
	case opts["pay"].(bool), opts["withdraw"].(bool):
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
			id, text, err := decodeNotifyBolt11(u.ChatId, 0, bolt11, optmsats)
			if err != nil {
				break
			}

			rds.Set("payinvoice:"+invlabel, bolt11, s.PayConfirmTimeout)
			rds.Set("payinvoice:"+invlabel+":msats", optmsats, s.PayConfirmTimeout)

			editWithKeyboard(u.ChatId, id,
				text+"\n\nPay the invoice described above?",
				tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("Cancel", fmt.Sprintf("cancel=%d", u.Id)),
						tgbotapi.NewInlineKeyboardButtonData("Yes", "pay="+invlabel),
					),
				),
			)
		} else {
			payInvoice(u, message.MessageID, bolt11, invlabel, optmsats)
		}
		break
	case opts["help"].(bool):
		handleHelp(u)
		break
	case opts["toggle"].(bool):
		if message.Chat.Type == "private" {
			break
		}

		switch {
		case opts["spammy"].(bool):
			if message.Chat.Type == "supergroup" {
				userchatconfig := tgbotapi.ChatConfigWithUser{
					ChatID:             message.Chat.ID,
					SuperGroupUsername: message.Chat.ChatConfig().SuperGroupUsername,
					UserID:             message.From.ID,
				}
				chatmember, err := bot.GetChatMember(userchatconfig)
				if err != nil ||
					(chatmember.Status != "administrator" && chatmember.Status != "creator") {
					log.Warn().Err(err).
						Int64("group", message.Chat.ID).
						Int("user", message.From.ID).
						Msg("toggle impossible. can't get user or not an admin.")
					break
				}
			} else if message.Chat.Type == "group" {
				// ok, everybody can toggle
			} else {
				break
			}

			log.Debug().Int64("group", message.Chat.ID).Msg("toggling spammy")
			spammy, err := toggleSpammy(message.Chat.ID)
			if err != nil {
				log.Warn().Err(err).Msg("failed to toggle spammy")
				break
			}

			if spammy {
				notify(message.Chat.ID, "This group is now spammy.")
			} else {
				notify(message.Chat.ID, "Not spamming anymore.")
			}
		}
	}
}

func handleCallback(cb *tgbotapi.CallbackQuery) {
	u, t, err := ensureUser(cb.From.ID, cb.From.UserName)
	if err != nil {
		log.Warn().Err(err).Int("case", t).
			Str("username", cb.From.UserName).
			Int("id", cb.From.ID).
			Msg("failed to ensure user on callback query")
		return
	}

	messageId := 0
	if cb.Message != nil {
		messageId = cb.Message.MessageID
	}

	switch {
	case cb.Data == "noop":
		goto answerEmpty
	case strings.HasPrefix(cb.Data, "cancel="):
		if strconv.Itoa(u.Id) != cb.Data[7:] {
			log.Warn().Err(err).
				Int("this", u.Id).
				Str("needed", cb.Data[7:]).
				Msg("user can't cancel")
			goto answerEmpty
		}
		removeKeyboardButtons(cb)
		appendTextToMessage(cb, "Canceled.")
		goto answerEmpty
	case strings.HasPrefix(cb.Data, "pay="):
		u, t, err := ensureUser(cb.From.ID, cb.From.UserName)
		if err != nil {
			log.Warn().Err(err).Int("case", t).
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
		sent := payInvoice(u, messageId, bolt11, invlabel, int(optmsats))
		if sent {
			appendTextToMessage(cb, "Payment sent.")
		}
		removeKeyboardButtons(cb)
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

		claimer, t, err := ensureUser(cb.From.ID, cb.From.UserName)
		if err != nil {
			log.Warn().Err(err).Int("case", t).
				Str("username", cb.From.UserName).Int("tgid", cb.From.ID).
				Msg("failed to ensure claimer user on giveaway.")
			goto answerEmpty
		}

		errMsg, err := u.sendInternally(messageId, claimer, sats*1000, "giveaway", nil)
		if err != nil {
			log.Warn().Err(err).Msg("failed to give away")
			claimer.notify("Failed to claim giveaway: " + errMsg)
			goto answerEmpty
		}
		bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, "Payment sent."))
		removeKeyboardButtons(cb)
		claimer.notify(fmt.Sprintf("%s has sent you %d satoshis.", u.AtName(), sats))

		var howtoclaimmessage = ""
		if claimer.ChatId == 0 {
			howtoclaimmessage = " To manage your funds, start a conversation with @lntxbot."
		}

		appendTextToMessage(cb,
			fmt.Sprintf(
				"%d satoshis given from %s to %s.", sats, u.AtName(), claimer.AtName(),
			)+howtoclaimmessage,
		)
		return
	case strings.HasPrefix(cb.Data, "remunc="):
		// remove unclaimed transaction
		// when you tip an invalid account or an account that has never talked with the bot
		hash := cb.Data[7:]
		_, err := pg.Exec(`
DELETE FROM lightning.transaction AS tx
WHERE substring(payment_hash from 0 for $2) = $1
  AND is_unclaimed(tx)
        `, hash, len(hash)+1)
		if err != nil {
			log.Error().Err(err).Str("hash", hash).Msg("failed to remove pending payment")
			appendTextToMessage(cb, "Error.")
			return
		}
		appendTextToMessage(cb, "Transaction canceled.")
	case strings.HasPrefix(cb.Data, "check="):
		// recheck transaction when for some reason it wasn't checked and
		// either confirmed or deleted automatically
		hashfirstchars := cb.Data[6:]
		txn, err := u.getTransaction(hashfirstchars)
		if err != nil {
			log.Warn().Err(err).Str("hash", hashfirstchars).
				Msg("failed to fetch transaction for checking")
			appendTextToMessage(cb, "Error.")
			return
		}
		go func(u User, messageId int, bolt11 string) {
			success, payment, err := ln.WaitPaymentResolution(bolt11)
			if err != nil {
				log.Warn().Err(err).Str("bolt11", bolt11).Str("user", u.Username).
					Msg("unexpected error waiting payment resolution")
				appendTextToMessage(cb, "Unexpected error: please report.")
				return
			}

			u.reactToPaymentStatus(success, messageId, payment)
		}(u, txn.TriggerMessage, txn.PendingBolt11.String)

		appendTextToMessage(cb, "Checking.")
	}

answerEmpty:
	bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, ""))
}

func handleHelp(u User) {
	helpString := strings.Replace(s.Usage, "  c ", "  /", -1)
	u.notifyMarkdown("```\n" + helpString + "\n```")
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

		bolt11, qrpath, err := makeInvoice(u, label, sats, "inline-"+q.ID, "")
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

func decodeNotifyBolt11(chatId int64, replyTo int, bolt11 string, optmsats int) (id int, text string, err error) {
	inv, err := decodeInvoice(bolt11)
	if err != nil {
		errMsg := messageFromError(err, "Failed to decode invoice")
		notify(chatId, errMsg)
		return
	}

	amount := int(inv.Get("msatoshi").Int())
	if amount == 0 {
		amount = optmsats
	}

	text = fmt.Sprintf(`
%d satoshis
<i>%s</i>
<b>Hash</b>: %s
<b>Node</b>: %s
        `,
		amount/1000,
		escapeHTML(inv.Get("description").String()),
		inv.Get("payment_hash").String(),
		inv.Get("payee").String(),
	)

	msg := notifyAsReply(chatId, text, replyTo)
	id = msg.MessageID
	return
}

func payInvoice(u User, messageId int, bolt11, label string, optmsats int) (payment_sent bool) {
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
			return false
		}
		target, err := loadUser(int(targetId), 0)
		if err != nil {
			log.Warn().Err(err).
				Str("intlabel", intlabel).
				Int64("id", targetId).
				Msg("failed to get load internal invoice target from postgres")
			u.notify("Failed to find invoice payee")
			return false
		}

		amount, hash, errMsg, err := u.payInternally(
			messageId,
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

			return false
		}

		// internal payment succeeded
		target.notifyAsReply(
			fmt.Sprintf("Payment received: %d satoshis. /tx%s.", amount/1000, hash[:5]),
			messageIdFromLabel(intlabel),
		)

		return true
	}

	err = u.payInvoice(messageId, bolt11, label, optmsats)
	if err != nil {
		u.notifyAsReply(err.Error(), messageId)
		return false
	}
	return true
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

	// the preimage should be on redis
	preimage := rds.Get("recinvoice:" + label + ":preimage").String()

	err = u.paymentReceived(
		int(msats),
		desc,
		hash,
		preimage,
		label,
	)
	if err != nil {
		u.notify(
			"Payment received, but failed to save on database. Please report this issue: " + label + ".",
		)
	}

	u.notifyAsReply(
		fmt.Sprintf("Payment received: %d. /tx%s.", msats/1000, hash[:5]),
		messageIdFromLabel(label),
	)
}
