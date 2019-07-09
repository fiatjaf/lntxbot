package main

import (
	"fmt"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strings"
	"regexp"
	"strconv"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/hoisie/mustache"
	"github.com/lucsky/cuid"
)

func handleMessage(message *tgbotapi.Message, bundle *i18n.Bundle) {
	u, t, err := ensureUser(message.From.ID, message.From.UserName)
	if err != nil {
		log.Warn().Err(err).Int("case", t).
			Str("username", message.From.UserName).
			Int("id", message.From.ID).
			Msg("failed to ensure user")
		return
	}
	locale :=  message.From.LanguageCode
	//
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
		text    = strings.ReplaceAll(
			regexp.MustCompile("/([a-z]+)@"+s.ServiceId).ReplaceAllString(message.Text, "/$1"),
			"—", "--",
		)
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
			msgTempl := map[string]interface{}{
				"HashFirstChars":             hashfirstchars,
			}
			msgStr, _ := translateTemplate("TxNotFound", locale, msgTempl)
			u.notifyAsReply(msgStr, message.MessageID)
			return
		}

		//TxInfo = "<code>{{.Status}}</code> {{ .PeerActionDescription}} on {{.TimeFormatted}} {{ .ClaimStatus}}
		//<i>{{.Description}}</i>
		//<b>Payee</b>: {{{.PayeeLink}}} ({{.PayeeAlias}})
		//<b>Hash</b>: {{.Hash}}
		//<b>Preimage</b>: {{.PreimageString}}
		//<b>Amount</b>: {{.Amount}} sat
		//<b>Fee paid</b>: {{.Fees}}"

		claimStatus := ""
		if txn.IsUnclaimed() {
			claimStatus = "(💤 unclaimed)"
		}

		msgTempl := map[string]interface{}{
			"Status": txn.Status,
			"PeerActionDescription": txn.PeerActionDescription(),
			"TimeFormatted": txn.TimeFormat(),
			"ClaimStatus": claimStatus,
			"Description": txn.Description,
			"PayeeLink": txn.PayeeLink(),
			"PayeeAlias": txn.PayeeAlias(),
			"Hash": txn.Hash,
			"PreimageString": txn.Preimage,
			"Amount": txn.Amount,
			"Fees": txn.FeeSatoshis(),

		}

		msgStr, _ := translateTemplate("TxInfo", locale, msgTempl)
		msgStr += "\n" + renderLogInfo(hashfirstchars)

		id := u.notifyAsReply(msgStr, txn.TriggerMessage).MessageID

		if txn.Status == "PENDING" {
			// allow people to cancel pending if they're old enough
			msgStr, _ := translate("RecheckPending", locale)
			editWithKeyboard(u.ChatId, id, text+msgStr,
				tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("Yes", "check="+hashfirstchars),
					),
				),
			)
		}

		if txn.IsUnclaimed() {
			quesStr, _ := translate("RetractQuestion", locale)
			answStr, _ := translate("Yes", locale)
			editWithKeyboard(u.ChatId, id, text+ quesStr,
				tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData(answStr, "remunc="+hashfirstchars),
					),
				),
			)
		}

		return
	}

	// query failed transactions (only available in the first 24h after the failure)
	if strings.HasPrefix(text, "/log") {
		hashfirstchars := text[4:]
		u.notify(renderLogInfo(hashfirstchars))
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
			log.Debug().Err(err).Str("command", text).
				Msg("Failed to parse command")

			method := strings.Split(text, " ")[0][1:]
			handled := handleHelp(u, method, locale)
			if !handled {
				msgStr, _ := translate("WrongCommand", locale)
				u.notify(msgStr)
			}
		}

		// save the fact that we didn't understand this so it can be edited and reevaluated
		rds.Set(fmt.Sprintf("parseerror:%d", message.MessageID), "1", time.Minute*5)

		return
	}

parsed:
	// if we reached this point we should make sure the command won't be editable again
	rds.Del(fmt.Sprintf("parseerror:%d", message.MessageID))

	if opts["paynow"].(bool) {
		opts["pay"] = true
		opts["now"] = true
	}

	switch {
	case opts["start"].(bool):
		if message.Chat.Type == "private" {
			u.setChat(message.Chat.ID)
			msgStr, _ := translate("Welcome", locale)
			u.notify(msgStr)
			handleHelp(u, "", locale)
		}
		break
	case opts["stop"].(bool):
		if message.Chat.Type == "private" {
			u.unsetChat()
			msgStr, _ := translate("StopNotify", locale)
			u.notify(msgStr)
		}
		break
	case opts["app"].(bool), opts["lapp"].(bool):
		handleExternalApp(u, opts, message.MessageID)
		break
	case opts["receive"].(bool), opts["invoice"].(bool), opts["fund"].(bool):
		sats, err := opts.Int("<satoshis>")
		if err != nil {
			// couldn't get an integer, but maybe it's because nothing was specified, so
			// it's an invoice of undefined amount.

			if v, exists := opts["<satoshis>"]; exists && v != nil && v.(string) != "any" {
				// ok, it exists, so it's an invalid amount.
				msgStr, _ := translate("InvalidAmt", locale)
				u.notify(msgStr + v.(string))
				break
			}

			// will be this if "any"
			sats = INVOICE_UNDEFINED_AMOUNT
		}
		var desc string
		if idesc, ok := opts["<description>"]; ok {
			desc = strings.Join(idesc.([]string), " ")
		}

		var preimage string
		if param, ok := opts["--preimage"]; ok {
			preimage, _ = param.(string)
		}

		bolt11, _, qrpath, err := u.makeInvoice(sats, desc, "", nil, message.MessageID, preimage, false)
		if err != nil {
			log.Warn().Err(err).Msg("failed to generate invoice")
			msgStr, _ := translate("FailedInvoice", locale)
			notify(message.Chat.ID, messageFromError(err, msgStr))
			return
		}

		// send invoice with qr code
		notifyWithPicture(message.Chat.ID, qrpath, bolt11)

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
			receiver      *User
			usernameval   interface{}
		)

		// get quantity
		sats, err := opts.Int("<satoshis>")

		if err != nil || sats <= 0 {
			// maybe the order of arguments is inverted
			if val, ok := opts["<satoshis>"].(string); ok && val[0] == '@' {
				// it seems to be
				usernameval = val
				if asats, ok := opts["<receiver>"].([]string); ok && len(asats) == 1 {
					sats, _ = strconv.Atoi(asats[0])
					goto gotusername
				}
			}
			msgStr, _ := translate("InvalidAmt", locale)
			defaultNotify(msgStr + opts["<satoshis>"].(string))
			break
		} else {
			usernameval = opts["<receiver>"]
		}

	gotusername:
		anonymous := false
		if opts["anonymously"].(bool) || opts["--anonymous"].(bool) || opts["sendanonymously"].(bool) {
			anonymous = true
		}

		receiver, todisplayname, err = parseUsername(message, usernameval)
		if err != nil {
			log.Warn().Interface("val", usernameval).Err(err).Msg("failed to parse username")
			break
		}
		if receiver != nil {
			goto ensured
		}

		// no username, this may be a reply-tip
		if message.ReplyToMessage != nil {
			log.Debug().Msg("it's a reply-tip")
			reply := message.ReplyToMessage

			var t int
			rec, t, err := ensureUser(reply.From.ID, reply.From.UserName)
			receiver = &rec
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
		} else {
			// if we ever reach this point then it's because the receiver is missing.
			msgStr0, _ := translate("CantSend", locale)
			msgStr1, _ := translate("NoReceiver", locale)
			defaultNotify( msgStr0 + opts["<satoshis>"].(string) + msgStr1)
			break
		}
	ensured:
		if err != nil {
			log.Warn().Err(err).
				Msg("failed to ensure target user on send/tip.")
			msgStr, _ := translate("SaveReceiverFail", locale)
			defaultNotify(msgStr)
			break
		}

		errMsg, err := u.sendInternally(
			message.MessageID,
			*receiver,
			anonymous,
			sats*1000,
			nil,
			nil,
		)
		if err != nil {
			log.Warn().Err(err).
				Str("from", u.Username).
				Str("to", todisplayname).
				Msg("failed to send/tip")
			msgStr, _ := translate("FailedSend", locale)
			defaultNotify(msgStr + errMsg)
			break
		}

		if receiver.ChatId != 0 {
			if anonymous {
				msgTempl := map[string]interface{}{
					"Sats": sats,
				}
				msgStr, _ := translateTemplate("ReceivedSats", locale, msgTempl)
				receiver.notify(msgStr)
			} else {
				msgTempl := map[string]interface{}{
					"User": u.AtName(),
					"Sats": sats,
				}
				msgStr, _ := translateTemplate("UserSentYouSats", locale, msgTempl)
				receiver.notify(msgStr)
			}
		}

		if message.Chat.Type == "private" {
			warning := ""
			if receiver.ChatId == 0 {
				warnTempl := map[string]interface{}{
					"User": todisplayname,
				}
				warning, _ = translateTemplate("NoUserWarning", locale, warnTempl)
			}
			msgTempl := map[string]interface{}{
				"User": todisplayname,
				"Sats": sats,
				"Warning": warning,
			}
			msgStr, _ := translateTemplate("UserSentToUser", locale, msgTempl)
			u.notifyAsReply(
				msgStr,
				message.MessageID,
			)
			break
		}
		msgTempl := map[string]interface{}{
			"User": todisplayname,
			"Sats": sats,
			"Warning": "",
		}
		msgStr, _ := translateTemplate("UserSentToUser", locale, msgTempl)
		defaultNotify(msgStr)
		break
	case opts["giveaway"].(bool):
		sats, err := opts.Int("<satoshis>")
		if err != nil {
			msgStr, _ := translate("InvalidAmount", locale)
			u.notify(msgStr + opts["<satoshis>"].(string))
			break
		}
		if !u.checkBalanceFor(sats, "giveaway") {
			break
		}
		msgTempl := map[string]interface{}{
			"User": u.AtName(),
			"Sats": sats,
		}
		msgStr, _ := translateTemplate("GiveAwayMsg", locale, msgTempl)
		chattable := tgbotapi.NewMessage(
			message.Chat.ID,
			msgStr,
		)
		chattable.BaseChat.ReplyMarkup = giveawayKeyboard(u.Id, sats)
		bot.Send(chattable)
		break
	case opts["giveflip"].(bool):
		sats, err := opts.Int("<satoshis>")
		if err != nil {
			msgStr, _ := translate("InvalidAmount", locale)
			u.notify(msgStr + opts["<satoshis>"].(string))
			break
		}
		if !u.checkBalanceFor(sats, "giveflip") {
			break
		}

		var nparticipants int
		if n, err := opts.Int("<num_participants>"); err == nil {
			if n < 2 || n > 100 {
				msgStr, _ := translate("InvalidPartNumber", locale)
				u.notify(msgStr + strconv.Itoa(n))
				break
			} else {
				nparticipants = n
			}
		}
		msgTempl := map[string]interface{}{
			"User": u.AtName(),
			"Sats": sats,
			"Participants": nparticipants,
		}
		msgStr, _ := translateTemplate("GiveFlipMsg", locale, msgTempl)
		chattable := tgbotapi.NewMessage(
			message.Chat.ID,
			msgStr,
		)
		giveflipid := cuid.Slug()
		chattable.BaseChat.ReplyMarkup = giveflipKeyboard(giveflipid, u.Id, nparticipants, sats)
		bot.Send(chattable)
		break
	case opts["coinflip"].(bool), opts["lottery"].(bool):
		// open a lottery between a number of users in a group
		sats, err := opts.Int("<satoshis>")
		if err != nil {
			msgStr, _ := translate("InvalidAmount", locale)
			u.notify(msgStr + opts["<satoshis>"].(string))
			break
		}
		if !u.checkBalanceFor(sats, "coinflip") {
			break
		}

		nparticipants := 2
		if n, err := opts.Int("<num_participants>"); err == nil {
			if n < 2 || n > 100 {
				msgStr, _ := translate("InvalidPartNumber", locale)
				u.notify(msgStr + strconv.Itoa(n))
				break
			} else {
				nparticipants = n
			}
		}
		msgTempl := map[string]interface{}{
			"EntrySats": sats,
			"Participants": nparticipants,
			"Prize": sats*nparticipants,
			"Registered": u.AtName(),
		}
		msgStr, _ := translateTemplate("LotteryMsg", locale, msgTempl)
		chattable := tgbotapi.NewMessage( message.Chat.ID, msgStr,)

		coinflipid := cuid.Slug()
		rds.SAdd("coinflip:"+coinflipid, u.Id)
		rds.Expire("coinflip:"+coinflipid, s.GiveAwayTimeout)
		chattable.BaseChat.ReplyMarkup = coinflipKeyboard(coinflipid, nparticipants, sats)
		bot.Send(chattable)
	case opts["fundraise"].(bool), opts["crowdfund"].(bool):
		// many people join, we get all the money and transfer to the target
		sats, err := opts.Int("<satoshis>")
		if err != nil {
			msgStr, _ := translate("InvalidAmount", locale)
			u.notify(msgStr + opts["<satoshis>"].(string))
			break
		}
		if !u.checkBalanceFor(sats, "fundraise") {
			break
		}

		nparticipants, err := opts.Int("<num_participants>")
		if err != nil || nparticipants < 2 || nparticipants > 100 {
			msgStr, _ := translate("InvalidPartNumber", locale)
			u.notify(msgStr + strconv.Itoa(nparticipants))
			break
		}

		receiver, receiverdisplayname, err := parseUsername(message, opts["<receiver>"])
		if err != nil {
			log.Warn().Err(err).Msg("parsing fundraise receiver")
			msgStr, _ := translate("FailedUser", locale)
			u.notify(msgStr)
			break
		}
		msgTempl := map[string]interface{}{
			"ToUser": receiverdisplayname,
			"Participants": nparticipants,
			"Sats": sats,
			"Fund": sats*nparticipants,
			"Registered": u.AtName(),
		}
		msgStr, _ := translateTemplate("FundraiseMsg", locale, msgTempl)
		chattable := tgbotapi.NewMessage(
			message.Chat.ID,
			msgStr,
		)

		fundraiseid := cuid.Slug()
		rds.SAdd("fundraise:"+fundraiseid, u.Id)
		rds.Expire("fundraise:"+fundraiseid, s.GiveAwayTimeout)
		chattable.BaseChat.ReplyMarkup = fundraiseKeyboard(fundraiseid, receiver.Id, nparticipants, sats)
		bot.Send(chattable)
	case opts["hide"].(bool):
		var content string
		if icontent, ok := opts["<message>"]; ok {
			content = strings.Join(icontent.([]string), " ")
		}

		sats, err := opts.Int("<satoshis>")
		if err != nil || sats == 0 {
			u.notify("Invalid amount: " + opts["<satoshis>"].(string))
			break
		}

		hiddenid := cuid.Slug()
		err = rds.Set(fmt.Sprintf("hidden:%d:%s:%d", u.Id, hiddenid, sats), content, s.HiddenMessageTimeout).Err()
		if err != nil {
			u.notify("Failed to store hidden content. Please report: " + err.Error())
			break
		}

		u.notifyAsReply(fmt.Sprintf("Message hidden with id <code>%s</code>.", hiddenid), message.MessageID)
	case opts["reveal"].(bool):
		hiddenid := opts["<hidden_message_id>"].(string)

		found := rds.Keys("hidden:*:" + hiddenid + ":*").Val()
		if len(found) == 0 {
			u.notifyAsReply("No hidden message found with the given id.", message.MessageID)
			break
		}

		redisKey := found[0]
		_, _, _, preview, satoshis, err := getHiddenMessage(redisKey)
		if err != nil {
			u.notify("Error loading hidden message. Please report: " + err.Error())
			break
		}

		chattable := tgbotapi.NewMessage(u.ChatId, preview)
		chattable.BaseChat.ReplyMarkup = revealKeyboard(redisKey, satoshis)
		bot.Send(chattable)
	case opts["transactions"].(bool):
		// show list of transactions
		limit := 25
		offset := 0
		if page, err := opts.Int("--page"); err == nil {
			offset = limit * (page - 1)
		}

		txns, err := u.listTransactions(limit, offset, 16, Both)
		if err != nil {
			log.Warn().Err(err).Str("user", u.Username).
				Msg("failed to list transactions")
			break
		}
		titleTempl := map[string]interface{}{
			"Limit": limit,
		}
		titleStr, _ := translateTemplate("TxHistTitle", locale, titleTempl)

		if offset > 0 {
			titleTempl := map[string]interface{}{
				"From": offset+1,
				"To": offset+limit,
			}
			titleStr, _ = translateTemplate("TxHistTitleOffset", locale, titleTempl)
		}
		u.notify(mustache.Render(`<b>{{title}}</b>
{{#txns}}
<code>{{StatusSmall}}</code> <code>{{PaddedSatoshis}}</code> {{Icon}} {{PeerActionDescription}}{{^TelegramPeer.Valid}}<i>{{Description}}</i>{{/TelegramPeer.Valid}} <i>{{TimeFormatSmall}}</i> /tx{{HashReduced}}
{{/txns}}
        `, map[string]interface{}{"title": titleStr, "txns": txns}))
		break
	case opts["balance"].(bool):
		// show balance
		info, err := u.getInfo()
		if err != nil {
			log.Warn().Err(err).Str("user", u.Username).Msg("failed to get info")
			break
		}

		msgTempl := map[string]interface{}{
			"Sats": info.Balance,
			"USD": getDollarPrice(int64(info.Balance*1000)),
			"Received": info.TotalReceived,
			"Sent": info.TotalSent,
			"Fees": info.TotalFees,
		}
		msgStr, _ := translateTemplate("BalanceMsg", locale, msgTempl)
		u.notify(msgStr)
		break
	case opts["pay"].(bool), opts["withdraw"].(bool), opts["decode"].(bool):
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
					msgStr, _ := translate("NoInvoice", locale)
					u.notify(msgStr)
					break
				}
			}
			msgStr, _ := translate("NoInvoice", locale)
			u.notify(msgStr)
			break
		} else {
			bolt11 = ibolt11.(string)
		}

		optsats, _ := opts.Int("<satoshis>")
		optmsats := optsats * 1000

		if askConfirmation {
			// decode invoice and show a button for confirmation
			inv, nodeAlias, usd, err := decodeInvoice(bolt11)
			if err != nil {
				errMsg := messageFromError(err, "Failed to decode invoice")
				notify(u.ChatId, errMsg)
				break
			}

			amount := int(inv.Get("msatoshi").Int())
			if amount == 0 {
				amount = optmsats
			}

			hash := inv.Get("payment_hash").String()
			msgTempl := map[string]interface{}{
				"Sats": amount/1000,
				"USD": usd,
				"Desc": escapeHTML(inv.Get("description").String()),
				"Hash": hash,
				"Node": nodeLink(inv.Get("payee").String()),
				"Alias": nodeAlias,
			}
			msgStr, _ := translateTemplate("ConfirmInvoice", locale, msgTempl)

			msg := notify(u.ChatId, msgStr)
			id := msg.MessageID

			hashfirstchars := hash[:5]
			rds.Set("payinvoice:"+hashfirstchars, bolt11, s.PayConfirmTimeout)
			rds.Set("payinvoice:"+hashfirstchars+":msats", optmsats, s.PayConfirmTimeout)
			askStr, _ := translate("AskToConfirm", locale)
			yesStr, _ := translate("Yes", locale)
			cancelStr, _ := translate("Cancel", locale)

			cancelTempl := map[string]interface{}{
				"User": u.Id,
			}
			calcelInlStr, _ := translateTemplate("CancelTemp", locale, cancelTempl)

			payTempl := map[string]interface{}{
				"Hash": hashfirstchars,
			}
			payInlStr, _ := translateTemplate("PayTemp", locale, payTempl)

			editWithKeyboard(u.ChatId, id,
				msgStr + askStr,
				tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData(cancelStr, calcelInlStr),
						tgbotapi.NewInlineKeyboardButtonData(yesStr, payInlStr),
					),
				),
			)
		} else {
			err := u.payInvoice(message.MessageID, bolt11, optmsats)
			if err != nil {
				u.notifyAsReply(err.Error(), message.MessageID)
			}
		}
		break
	case opts["bluewallet"].(bool), opts["lndhub"].(bool):
		password := u.Password
		if opts["refresh"].(bool) {
			password, err = u.updatePassword()
			if err != nil {
				log.Warn().Err(err).Str("user", u.Username).Msg("error updating password")
				u.notify("Error updating password. Please report this issue.")
			}
		}

		u.notify(fmt.Sprintf("<code>lndhub://%d:%s@%s</code>", u.Id, password, s.ServiceURL))
	case opts["help"].(bool):
		command, _ := opts.String("<command>")
		handleHelp(u, command, locale)
		break
	case opts["toggle"].(bool):
		if message.Chat.Type == "private" {
			break
		}

		if !isAdmin(message) {
			break
		}

		switch {
		case opts["ticket"].(bool):
			log.Debug().Int64("group", message.Chat.ID).Msg("toggling ticket")
			price, err := opts.Int("<price>")
			if err != nil {
				setTicketPrice(message.Chat.ID, 0)
				msgStr, _ := translate("FreeJoin", locale)
				notify(message.Chat.ID, msgStr)
			}
			setTicketPrice(message.Chat.ID, price)
			filterTempl := map[string]interface{}{
				"Sat": price,
			}
			filterStr, _ := translateTemplate("FilterMsg", locale, filterTempl)
			notify(message.Chat.ID, filterStr)
		case opts["spammy"].(bool):
			log.Debug().Int64("group", message.Chat.ID).Msg("toggling spammy")
			spammy, err := toggleSpammy(message.Chat.ID)
			if err != nil {
				log.Warn().Err(err).Msg("failed to toggle spammy")
				break
			}

			if spammy {
				msgStr, _ := translate("SpammyMsg", locale)
				notify(message.Chat.ID, msgStr)
			} else {
				msgStr, _ := translate("NoSpammyMsg", locale)
				notify(message.Chat.ID, msgStr)
			}
		}
	}
}

func handleEditedMessage(message *tgbotapi.Message, bundle *i18n.Bundle) {
	res, err := rds.Get(fmt.Sprintf("parseerror:%d", message.MessageID)).Result()
	if err != nil {
		return
	}

	if res != "1" {
		return
	}

	handleMessage(message, bundle)
}
