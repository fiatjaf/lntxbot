package main

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/docopt/docopt-go"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/lucsky/cuid"
)

func handleMessage(message *tgbotapi.Message) {
	u, tcase, err := ensureUser(message.From.ID, message.From.UserName, message.From.LanguageCode)
	if err != nil {
		log.Warn().Err(err).Int("case", tcase).
			Str("username", message.From.UserName).
			Int("id", message.From.ID).
			Msg("failed to ensure user")
		return
	}

	// by default we use the user locale for the group object, because
	// we may end up sending the message to the user instead of to the group
	// (if, for example, the user calls /coinflip on his own chat) then
	// we at least want the correct language used there.
	g := GroupChat{TelegramId: message.Chat.ID, Locale: u.Locale}

	if message.Chat.Type == "private" {
		// after ensuring the user we should always enable him to
		// receive payment notifications and so on, as not all people will
		// remember to call /start
		u.setChat(message.Chat.ID)
	} else {
		// when we're in a group, load the group
		loadedGroup, err := loadGroup(message.Chat.ID)
		if err != nil {
			if err != sql.ErrNoRows {
				log.Warn().Err(err).Int64("id", message.Chat.ID).Msg("failed to load group")
			}
			// proceed with an empty group (manually defined before)
		} else {
			// we manage to load a group, use it then
			g = loadedGroup
		}

		if message.Entities == nil || len(*message.Entities) == 0 ||
			// unless in the private chat, only messages starting with
			// bot commands will work
			(*message.Entities)[0].Type != "bot_command" ||
			(*message.Entities)[0].Offset != 0 {
			return
		}
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
			u.notifyAsReply(t.TXNOTFOUND, t.T{"HashFirstChars": hashfirstchars}, message.MessageID)
			return
		}

		txstatus := translateTemplate(t.TXINFO, u.Locale, t.T{
			"Txn":     txn,
			"LogInfo": renderLogInfo(hashfirstchars),
		})
		msgId := sendMessageAsReply(u.ChatId, txstatus, txn.TriggerMessage).MessageID

		if txn.Status == "PENDING" {
			// allow people to cancel pending if they're old enough
			editWithKeyboard(u.ChatId, msgId, txstatus+"\n\n"+translate(t.RECHECKPENDING, u.Locale),
				tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData(translate(t.YES, u.Locale), "check="+hashfirstchars),
					),
				),
			)
		}

		if txn.IsUnclaimed() {
			editWithKeyboard(u.ChatId, msgId, txstatus+"\n\n"+translate(t.RETRACTQUESTION, u.Locale),
				tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData(translate(t.YES, u.Locale), "remunc="+hashfirstchars),
					),
				),
			)
		}

		return
	}

	// query failed transactions (only available in the first 24h after the failure)
	if strings.HasPrefix(text, "/log") {
		hashfirstchars := text[4:]
		sendMessage(u.ChatId, renderLogInfo(hashfirstchars))
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
				Msg("failed to parse command")

			method := strings.Split(text, " ")[0][1:]
			handled := handleHelp(u, method)
			if !handled {
				u.notify(t.WRONGCOMMAND, nil)
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
			u.notify(t.WELCOME, nil)
			handleHelp(u, "")
		}
		break
	case opts["stop"].(bool):
		if message.Chat.Type == "private" {
			u.unsetChat()
			u.notify(t.STOPNOTIFY, nil)
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
				u.notify(t.INVALIDAMT, t.T{"Amount": v})
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
			u.notify(t.FAILEDINVOICE, t.T{"Err": messageFromError(err)})
			return
		}

		// send invoice with qr code
		sendMessageWithPicture(message.Chat.ID, qrpath, bolt11)

		break
	case opts["send"].(bool), opts["tip"].(bool):
		// default notify function to use depending on many things
		var defaultNotify func(t.Key, t.T)
		if message.Chat.Type == "private" {
			defaultNotify = func(key t.Key, data t.T) { u.notifyAsReply(key, data, message.MessageID) }
		} else if isSpammy(message.Chat.ID) {
			defaultNotify = func(key t.Key, data t.T) { g.notifyAsReply(key, data, message.MessageID) }
		} else {
			defaultNotify = func(key t.Key, data t.T) { u.notify(key, data) }
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
			defaultNotify(t.INVALIDAMOUNT, t.T{"Amount": opts["<satoshis>"]})
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
			rec, t, err := ensureUser(reply.From.ID, reply.From.UserName, reply.From.LanguageCode)
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
			defaultNotify(t.CANTSENDNORECEIVER, t.T{"Sats": opts["<satoshis>"]})
			break
		}
	ensured:
		if err != nil {
			log.Warn().Err(err).Msg("failed to ensure target user on send/tip.")
			defaultNotify(t.SAVERECEIVERFAIL, nil)
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
			defaultNotify(t.FAILEDSEND, t.T{"Err": errMsg})
			break
		}

		if receiver.ChatId != 0 {
			if anonymous {
				receiver.notify(t.RECEIVEDSATSANON, t.T{"Sats": sats})
			} else {
				receiver.notify(t.USERSENTYOUSATS, t.T{
					"User": u.AtName(),
					"Sats": sats,
				})
			}
		}

		if message.Chat.Type == "private" {
			u.notifyAsReply(t.USERSENTTOUSER, t.T{
				"User":              todisplayname,
				"Sats":              sats,
				"ReceiverHasNoChat": receiver.ChatId == 0,
			}, message.MessageID)
			break
		}

		defaultNotify(t.USERSENTTOUSER, t.T{
			"User":              todisplayname,
			"Sats":              sats,
			"ReceiverHasNoChat": false,
		})
		break
	case opts["giveaway"].(bool):
		sats, err := opts.Int("<satoshis>")
		if err != nil {
			u.notify(t.INVALIDAMOUNT, t.T{"Amount": opts["<satoshis>"]})
			break
		}
		if !u.checkBalanceFor(sats, "giveaway") {
			break
		}
		chattable := tgbotapi.NewMessage(
			message.Chat.ID,
			translateTemplate(t.GIVEAWAYMSG, g.Locale, t.T{
				"User": u.AtName(),
				"Sats": sats,
			}),
		)
		chattable.BaseChat.ReplyMarkup = giveawayKeyboard(u.Id, sats, g.Locale)
		bot.Send(chattable)
		break
	case opts["giveflip"].(bool):
		sats, err := opts.Int("<satoshis>")
		if err != nil {
			u.notify(t.INVALIDAMOUNT, t.T{"Amount": opts["<satoshis>"]})
			break
		}
		if !u.checkBalanceFor(sats, "giveflip") {
			break
		}

		var nparticipants int
		if n, err := opts.Int("<num_participants>"); err == nil {
			if n < 2 || n > 100 {
				u.notify(t.INVALIDPARTNUMBER, t.T{"Number": strconv.Itoa(n)})
				break
			} else {
				nparticipants = n
			}
		}
		chattable := tgbotapi.NewMessage(
			message.Chat.ID,
			translateTemplate(t.GIVEFLIPMSG, g.Locale, t.T{
				"User":         u.AtName(),
				"Sats":         sats,
				"Participants": nparticipants,
			}),
		)
		giveflipid := cuid.Slug()
		chattable.BaseChat.ReplyMarkup = giveflipKeyboard(giveflipid, u.Id, nparticipants, sats, g.Locale)
		bot.Send(chattable)
		break
	case opts["coinflip"].(bool), opts["lottery"].(bool):
		// open a lottery between a number of users in a group
		sats, err := opts.Int("<satoshis>")
		if err != nil {
			u.notify(t.INVALIDAMT, t.T{"Amount": opts["<satoshis>"]})
			break
		}
		if !u.checkBalanceFor(sats, "coinflip") {
			break
		}

		nparticipants := 2
		if n, err := opts.Int("<num_participants>"); err == nil {
			if n < 2 || n > 100 {
				u.notify(t.INVALIDPARTNUMBER, t.T{"Number": strconv.Itoa(n)})
				break
			} else {
				nparticipants = n
			}
		}
		chattable := tgbotapi.NewMessage(
			message.Chat.ID,
			translateTemplate(t.LOTTERYMSG, g.Locale, t.T{
				"EntrySats":    sats,
				"Participants": nparticipants,
				"Prize":        sats * nparticipants,
				"Registered":   u.AtName(),
			}),
		)

		coinflipid := cuid.Slug()
		rds.SAdd("coinflip:"+coinflipid, u.Id)
		rds.Expire("coinflip:"+coinflipid, s.GiveAwayTimeout)
		chattable.BaseChat.ReplyMarkup = coinflipKeyboard(coinflipid, nparticipants, sats, g.Locale)
		bot.Send(chattable)
	case opts["fundraise"].(bool), opts["crowdfund"].(bool):
		// many people join, we get all the money and transfer to the target
		sats, err := opts.Int("<satoshis>")
		if err != nil {
			u.notify(t.INVALIDAMOUNT, t.T{"Amount": opts["<satoshis>"]})
			break
		}
		if !u.checkBalanceFor(sats, "fundraise") {
			break
		}

		nparticipants, err := opts.Int("<num_participants>")
		if err != nil || nparticipants < 2 || nparticipants > 100 {
			u.notify(t.INVALIDPARTNUMBER, t.T{"Number": nparticipants})
			break
		}

		receiver, receiverdisplayname, err := parseUsername(message, opts["<receiver>"])
		if err != nil {
			log.Warn().Err(err).Msg("parsing fundraise receiver")
			u.notify(t.FAILEDUSER, nil)
			break
		}
		chattable := tgbotapi.NewMessage(
			message.Chat.ID,
			translateTemplate(t.FUNDRAISEAD, g.Locale, t.T{
				"ToUser":       receiverdisplayname,
				"Participants": nparticipants,
				"Sats":         sats,
				"Fund":         sats * nparticipants,
				"Registered":   u.AtName(),
			}),
		)

		fundraiseid := cuid.Slug()
		rds.SAdd("fundraise:"+fundraiseid, u.Id)
		rds.Expire("fundraise:"+fundraiseid, s.GiveAwayTimeout)
		keyboard := fundraiseKeyboard(fundraiseid, receiver.Id, nparticipants, sats, g.Locale)
		chattable.BaseChat.ReplyMarkup = &keyboard
		bot.Send(chattable)
	case opts["hide"].(bool):
		var content string
		if icontent, ok := opts["<message>"]; ok {
			content = strings.Join(icontent.([]string), " ")
		}

		sats, err := opts.Int("<satoshis>")
		if err != nil || sats == 0 {
			u.notify(t.INVALIDAMOUNT, t.T{"Amount": opts["<satoshis>"]})
			break
		}

		hiddenid := cuid.Slug()
		err = rds.Set(fmt.Sprintf("hidden:%d:%s:%d", u.Id, hiddenid, sats), content, s.HiddenMessageTimeout).Err()
		if err != nil {
			u.notify(t.HIDDENSTOREFAIL, t.T{"Err": err.Error()})
			break
		}

		u.notifyAsReply(t.HIDDENWITHID, t.T{"HiddenId": hiddenid}, message.MessageID)
	case opts["reveal"].(bool):
		hiddenid := opts["<hidden_message_id>"].(string)

		found := rds.Keys("hidden:*:" + hiddenid + ":*").Val()
		if len(found) == 0 {
			u.notifyAsReply(t.HIDDENMSGNOTFOUND, nil, message.MessageID)
			break
		}

		redisKey := found[0]
		_, _, _, preview, satoshis, err := getHiddenMessage(redisKey, g.Locale)
		if err != nil {
			u.notify(t.HIDDENMSGFAIL, t.T{"Err": err.Error()})
			break
		}

		chattable := tgbotapi.NewMessage(u.ChatId, preview)
		chattable.BaseChat.ReplyMarkup = revealKeyboard(redisKey, satoshis, g.Locale)
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

		u.notify(t.TXLIST, t.T{
			"Offset":       offset,
			"Limit":        limit,
			"From":         offset + 1,
			"To":           offset + limit,
			"Transactions": txns,
		})
		break
	case opts["balance"].(bool):
		// show balance
		info, err := u.getInfo()
		if err != nil {
			log.Warn().Err(err).Str("user", u.Username).Msg("failed to get info")
			break
		}

		u.notify(t.BALANCEMSG, t.T{
			"Sats":     info.Balance,
			"USD":      getDollarPrice(int64(info.Balance * 1000)),
			"Received": info.TotalReceived,
			"Sent":     info.TotalSent,
			"Fees":     info.TotalFees,
		})
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
					u.notify(t.NOINVOICE, nil)
					break
				}
			}
			u.notify(t.NOINVOICE, nil)
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
				u.notify(t.FAILEDDECODE, t.T{"Err": messageFromError(err)})
				break
			}

			amount := int(inv.Get("msatoshi").Int())
			if amount == 0 {
				amount = optmsats
			}

			hash := inv.Get("payment_hash").String()
			text := translateTemplate(t.CONFIRMINVOICE, u.Locale, t.T{
				"Sats":  amount / 1000,
				"USD":   usd,
				"Desc":  escapeHTML(inv.Get("description").String()),
				"Hash":  hash,
				"Node":  nodeLink(inv.Get("payee").String()),
				"Alias": nodeAlias,
			})

			msgId := sendMessage(u.ChatId, text).MessageID

			hashfirstchars := hash[:5]
			rds.Set("payinvoice:"+hashfirstchars, bolt11, s.PayConfirmTimeout)
			rds.Set("payinvoice:"+hashfirstchars+":msats", optmsats, s.PayConfirmTimeout)

			editWithKeyboard(u.ChatId, msgId,
				text+"\n\n"+translate(t.ASKTOCONFIRM, u.Locale),
				tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData(
							translate(t.CANCEL, u.Locale),
							fmt.Sprintf("cancel=%d", u.Id)),
						tgbotapi.NewInlineKeyboardButtonData(
							translate(t.YES, u.Locale),
							fmt.Sprintf("pay=%s", hashfirstchars)),
					),
				),
			)
		} else {
			err := u.payInvoice(message.MessageID, bolt11, optmsats)
			if err != nil {
				u.notifyAsReply(t.ERROR, t.T{"Err": err.Error()}, message.MessageID)
			}
		}
		break
	case opts["bluewallet"].(bool), opts["lndhub"].(bool):
		password := u.Password
		if opts["refresh"].(bool) {
			password, err = u.updatePassword()
			if err != nil {
				log.Warn().Err(err).Str("user", u.Username).Msg("error updating password")
				u.notify(t.BLUEWALLETPASSWORDUPDATEERROR, t.T{"Err": err.Error()})
			}
		}

		u.notify(t.BLUEWALLETCREDENTIALS, t.T{
			"Credentials": fmt.Sprintf("lndhub://%d:%s@%s", u.Id, password, s.ServiceURL),
		})
	case opts["help"].(bool):
		command, _ := opts.String("<command>")
		handleHelp(u, command)
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
				sendMessage(message.Chat.ID, translate("FreeJoin", g.Locale))
			}
			setTicketPrice(message.Chat.ID, price)
			g.notify(t.TICKETMSG, t.T{
				"Sat": price,
			})
		case opts["spammy"].(bool):
			log.Debug().Int64("group", message.Chat.ID).Msg("toggling spammy")
			spammy, err := toggleSpammy(message.Chat.ID)
			if err != nil {
				log.Warn().Err(err).Msg("failed to toggle spammy")
				break
			}

			g.notify(t.SPAMMYMSG, t.T{"Spammy": spammy})
		}
	}
}

func handleEditedMessage(message *tgbotapi.Message) {
	res, err := rds.Get(fmt.Sprintf("parseerror:%d", message.MessageID)).Result()
	if err != nil {
		return
	}

	if res != "1" {
		return
	}

	handleMessage(message)
}
