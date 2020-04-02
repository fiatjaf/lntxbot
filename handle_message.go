package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/go-lnurl"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/lucsky/cuid"
	"github.com/skip2/go-qrcode"
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

	// this is just to send to amplitude
	var group *int64 = nil

	if message.Chat.Type == "private" {
		// after ensuring the user we should always enable him to
		// receive payment notifications and so on, as not all people will
		// remember to call /start
		u.setChat(message.Chat.ID)
		g.TelegramId = -g.TelegramId // because we invert when sending a message
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

		group = &message.Chat.ID

		if message.Entities == nil || len(*message.Entities) == 0 ||
			// unless in the private chat, only messages starting with
			// bot commands will work
			(*message.Entities)[0].Type != "bot_command" ||
			(*message.Entities)[0].Offset != 0 {
			return
		}
	}

	var (
		opts        = make(docopt.Opts)
		isCommand   = false
		messageText = strings.ReplaceAll(
			regexp.MustCompile("/([\\w_]+)@"+s.ServiceId).ReplaceAllString(message.Text, "/$1"),
			"—", "--",
		)
	)

	log.Debug().Str("t", messageText).Int("user", u.Id).Msg("got message")

	// when receiving a forwarded invoice (from messages from other people?)
	// or just the full text of a an invoice (shared from a phone wallet?)
	if !strings.HasPrefix(messageText, "/") {
		if bolt11, lnurltext, ok := searchForInvoice(u, *message); ok {
			if bolt11 != "" {
				opts, _, _ = parse("/pay " + bolt11)
				goto parsed
			}
			if lnurltext != "" {
				opts, _, _ = parse("/lnurl " + lnurltext)
				goto parsed
			}
		}
	}

	// otherwise parse the slash command
	opts, isCommand, err = parse(messageText)
	if !isCommand {
		if message.ReplyToMessage != nil && message.ReplyToMessage.From.ID == bot.Self.ID {
			// may be a written reply to a specific bot prompt
			handleReply(u, message, message.ReplyToMessage.MessageID)
		}

		return
	}
	if err != nil {
		if message.Chat.Type == "private" {
			// only tell we don't understand commands when in a private chat
			// because these commands we're not understanding
			// may be targeting other bots in a group, so we're spamming people.
			log.Debug().Err(err).Str("command", messageText).
				Msg("failed to parse command")

			method := strings.Split(messageText, " ")[0][1:]
			handled := handleHelp(u, method)
			if !handled {
				u.notify(t.WRONGCOMMAND, nil)
			}
		}

		// save the fact that we didn't understand this so it can be edited and reevaluated
		rds.Set(fmt.Sprintf("parseerror:%d", message.MessageID), "1", time.Minute*5)

		return
	}

	go u.track("command", map[string]interface{}{
		"command": strings.Split(strings.Split(message.Text, " ")[0], "_")[0],
		"group":   group,
	})

parsed:
	// if we reached this point we should make sure the command won't be editable again
	rds.Del(fmt.Sprintf("parseerror:%d", message.MessageID))

	if opts["paynow"].(bool) {
		opts["pay"] = true
		opts["now"] = true
	}

	switch {
	case opts["start"].(bool), opts["tutorial"].(bool):
		if message.Chat.Type == "private" {
			u.setChat(message.Chat.ID)

			if tutorial, err := opts.String("<tutorial>"); err != nil || tutorial == "" {
				handleTutorial(u, tutorial)
			} else {
				u.notify(t.WELCOME, nil)
				handleTutorial(u, "")
			}
			go u.track("start", nil)
		}
		break
	case opts["stop"].(bool):
		if message.Chat.Type == "private" {
			u.unsetChat()
			u.notify(t.STOPNOTIFY, nil)
			go u.track("stop", nil)
		}
		break
	case opts["microbet"].(bool), opts["fundbtc"].(bool),
		opts["satellite"].(bool), opts["gifts"].(bool),
		opts["sats4ads"].(bool),
		opts["qiwi"].(bool), opts["yandex"].(bool),
		opts["bitrefill"].(bool), opts["bitclouds"].(bool),
		opts["etleneum"].(bool):
		handleExternalApp(u, opts, message)
		break
	case opts["bluewallet"].(bool), opts["lndhub"].(bool):
		go u.track("bluewallet", map[string]interface{}{
			"refresh": opts["refresh"].(bool),
		})

		password := u.Password
		if opts["refresh"].(bool) {
			password, err = u.updatePassword()
			if err != nil {
				log.Warn().Err(err).Str("user", u.Username).Msg("error updating password")
				u.notify(t.APIPASSWORDUPDATEERROR, t.T{"Err": err.Error()})
				return
			}
			u.notify(t.COMPLETED, nil)
		} else {
			blueURL := fmt.Sprintf("lndhub://%d:%s@%s", u.Id, password, s.ServiceURL)
			qrpath := qrImagePath(fmt.Sprintf("bluewallet-%d", u.Id))
			qrcode.WriteFile(blueURL, qrcode.Medium, 256, qrpath)
			sendMessageWithPicture(u.ChatId, qrpath, "<code>"+blueURL+"</code>")
		}
	case opts["api"].(bool):
		go u.track("api", nil)

		passwordFull := u.Password
		passwordInvoice := calculateHash(passwordFull)
		passwordReadOnly := calculateHash(passwordInvoice)

		tokenFull := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%s", u.Id, passwordFull)))
		tokenInvoice := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%s", u.Id, passwordInvoice)))
		tokenReadOnly := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%s", u.Id, passwordReadOnly)))

		switch {
		case opts["full"].(bool):
			qrpath := qrImagePath(fmt.Sprintf("api-%d-%s", u.Id, "full"))
			qrcode.WriteFile(tokenFull, qrcode.Medium, 256, qrpath)
			sendMessageWithPicture(u.ChatId, qrpath, tokenFull)
		case opts["invoice"].(bool):
			qrpath := qrImagePath(fmt.Sprintf("api-%d-%s", u.Id, "invoice"))
			qrcode.WriteFile(tokenInvoice, qrcode.Medium, 256, qrpath)
			sendMessageWithPicture(u.ChatId, qrpath, tokenInvoice)
		case opts["readonly"].(bool):
			qrpath := qrImagePath(fmt.Sprintf("api-%d-%s", u.Id, "readonly"))
			qrcode.WriteFile(tokenReadOnly, qrcode.Medium, 256, qrpath)
			sendMessageWithPicture(u.ChatId, qrpath, tokenReadOnly)
		case opts["url"].(bool):
			qrpath := qrImagePath(fmt.Sprintf("api-%d-%s", u.Id, "url"))
			qrcode.WriteFile(s.ServiceURL+"/", qrcode.Medium, 256, qrpath)
			sendMessageWithPicture(u.ChatId, qrpath, s.ServiceURL+"/")
		case opts["refresh"].(bool):
			opts["bluewallet"] = true
			goto parsed
		default:
			u.notify(t.APICREDENTIALS, t.T{
				"Full":       tokenFull,
				"Invoice":    tokenInvoice,
				"ReadOnly":   tokenReadOnly,
				"ServiceURL": s.ServiceURL,
			})
		}
	case opts["tx"].(bool):
		// individual transaction query
		hash := opts["<hash>"].(string)
		if len(hash) < 5 {
			u.notify(t.ERROR, t.T{"Err": "hash too small."})
			return
		}
		go u.track("view tx", nil)
		go handleSingleTransaction(u, hash, message.MessageID)
	case opts["log"].(bool):
		// query failed transactions (only available in the first 24h after the failure)
		hash := opts["<hash>"].(string)
		if len(hash) < 5 {
			u.notify(t.ERROR, t.T{"Err": "hash too small."})
			return
		}
		go u.track("view log", nil)
		go sendMessage(u.ChatId, renderLogInfo(hash))
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
			extra         string
		)

		// get quantity
		sats, err := opts.Int("<satoshis>")

		if err != nil || sats <= 0 {
			defaultNotify(t.INVALIDAMOUNT, t.T{"Amount": opts["<satoshis>"]})
			break
		} else {
			usernameval = opts["<receiver>"]
		}

		anonymous := false
		if opts["anonymously"].(bool) || opts["--anonymous"].(bool) || opts["sendanonymously"].(bool) {
			anonymous = true
		}

		receiver, todisplayname, _ = parseUsername(message, usernameval)
		if receiver != nil {
			goto ensured
		}

		// no username, this may be a reply-tip
		if message.ReplyToMessage != nil {
			if iextra, ok := opts["<receiver>"]; ok {
				// in this case this may be a tipping message
				extra = strings.Join(iextra.([]string), " ")
			}

			log.Debug().Str("extra", extra).Msg("it's a reply-tip")
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
			extra,
			"",
			"",
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

		go u.track("send", map[string]interface{}{
			"group":     group,
			"reply-tip": message.ReplyToMessage != nil,
			"sats":      sats,
		})

		break
	case opts["giveaway"].(bool):
		sats, err := opts.Int("<satoshis>")
		if err != nil {
			u.notify(t.INVALIDAMOUNT, t.T{"Amount": opts["<satoshis>"]})
			break
		}
		if !canJoinGiveaway(u.Id) {
			u.notify(t.OVERQUOTA, t.T{"App": "giveaway"})
			return
		}
		if !u.checkBalanceFor(sats, "giveaway", nil) {
			break
		}

		sendMessageWithKeyboard(
			message.Chat.ID,
			translateTemplate(t.GIVEAWAYMSG, g.Locale, t.T{
				"User": u.AtName(),
				"Sats": sats,
			}),
			giveawayKeyboard(u.Id, sats, g.Locale),
			0,
		)

		go u.track("giveaway created", map[string]interface{}{
			"group": message.Chat.ID,
			"sats":  sats,
		})
		break
	case opts["giveflip"].(bool):
		sats, err := opts.Int("<satoshis>")
		if err != nil {
			u.notify(t.INVALIDAMOUNT, t.T{"Amount": opts["<satoshis>"]})
			break
		}
		if !canCreateGiveflip(u.Id) {
			u.notify(t.RATELIMIT, nil)
			return
		}
		if !canJoinGiveflip(u.Id) {
			u.notify(t.OVERQUOTA, t.T{"App": "giveflip"})
			return
		}
		if !u.checkBalanceFor(sats, "giveflip", nil) {
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
		} else {
			nparticipants = 2
		}

		giveflipid := cuid.Slug()
		sendMessageWithKeyboard(
			message.Chat.ID,
			translateTemplate(t.GIVEFLIPMSG, g.Locale, t.T{
				"User":         u.AtName(),
				"Sats":         sats,
				"Participants": nparticipants,
			}),
			giveflipKeyboard(giveflipid, u.Id, nparticipants, sats, g.Locale),
			0,
		)

		go u.track("giveflip created", map[string]interface{}{
			"group": message.Chat.ID,
			"sats":  sats,
			"n":     nparticipants,
		})
		break
	case opts["coinflip"].(bool), opts["lottery"].(bool):
		enabled := areCoinflipsEnabled(message.Chat.ID)
		if !enabled {
			forwardMessage(message, u.ChatId)
			deleteMessage(message)
			u.notify(t.COINFLIPSENABLEDMSG, t.T{"Enabled": false})
			break
		}

		// open a lottery between a number of users in a group
		sats, err := opts.Int("<satoshis>")
		if err != nil {
			u.notify(t.INVALIDAMT, t.T{"Amount": opts["<satoshis>"]})
			break
		}

		if !canCreateCoinflip(u.Id) {
			u.notify(t.RATELIMIT, nil)
			return
		}
		if !canJoinCoinflip(u.Id) {
			u.notify(t.OVERQUOTA, t.T{"App": "coinflip"})
			return
		}
		if !u.checkBalanceFor(sats, "coinflip", nil) {
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

		sendMessageWithKeyboard(
			message.Chat.ID,
			translateTemplate(t.LOTTERYMSG, g.Locale, t.T{
				"EntrySats":    sats,
				"Participants": nparticipants,
				"Prize":        sats * nparticipants,
				"Registered":   u.AtName(),
			}),
			coinflipKeyboard("", u.Id, nparticipants, sats, g.Locale),
			0,
		)

		// save this to limit coinflip creation per user
		go u.track("coinflip created", map[string]interface{}{
			"group": message.Chat.ID,
			"sats":  sats,
			"n":     nparticipants,
		})
		rds.Set(fmt.Sprintf("recentcoinflip:%d", u.Id), "t", time.Minute*30)
	case opts["fundraise"].(bool), opts["crowdfund"].(bool):
		// many people join, we get all the money and transfer to the target
		sats, err := opts.Int("<satoshis>")
		if err != nil {
			u.notify(t.INVALIDAMOUNT, t.T{"Amount": opts["<satoshis>"]})
			break
		}
		if !u.checkBalanceFor(sats, "fundraise", nil) {
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

		sendMessageWithKeyboard(
			message.Chat.ID,
			translateTemplate(t.FUNDRAISEAD, g.Locale, t.T{
				"ToUser":       receiverdisplayname,
				"Participants": nparticipants,
				"Sats":         sats,
				"Fund":         sats * nparticipants,
				"Registered":   u.AtName(),
			}),
			fundraiseKeyboard("", u.Id, receiver.Id, nparticipants, sats, g.Locale),
			0,
		)

		go u.track("fundraise created", map[string]interface{}{
			"group": message.Chat.ID,
			"sats":  sats,
			"n":     nparticipants,
		})
	case opts["hide"].(bool):
		hiddenid := getHiddenId(message) // deterministic

		var content string
		var preview string

		// if there's a replyto, use that as the content
		if message.ReplyToMessage != nil {
			content = message.ReplyToMessage.Text
		}

		// or use the inline message
		// -- or if there's a replyo and inline, the inline part is the preview
		if icontent, ok := opts["<message>"]; ok {
			message := strings.Join(icontent.([]string), " ")
			if content != "" {
				// we are using the text from the replyto as the content, this is the preview
				preview = message
			} else {
				// otherwise parse the ~ thing
				contentparts := strings.SplitN(message, "~", 2)
				if len(contentparts) == 2 {
					preview = contentparts[0]
					content = contentparts[1]
				}
			}
		} else if message.ReplyToMessage == nil {
			// no content found
			u.notify(t.ERROR, t.T{"Err": err.Error()})
			return
		}

		sats, err := opts.Int("<satoshis>")
		if err != nil || sats == 0 {
			u.notify(t.INVALIDAMOUNT, t.T{"Amount": opts["<satoshis>"]})
			return
		}

		public := opts["--public"].(bool)
		if private := opts["--private"].(bool); private {
			public = false
		}

		crowdfund, _ := opts.Int("--crowdfund")
		if crowdfund > 1 {
			public = true
		} else {
			crowdfund = 1
		}

		payabletimes, _ := opts.Int("--revealers")
		if payabletimes > 1 {
			public = false
			crowdfund = 1
		} else {
			payabletimes = 0
		}

		hiddenmessage := HiddenMessage{
			Preview:   preview,
			Content:   content,
			Times:     payabletimes,
			Crowdfund: crowdfund,
			Public:    public,
			Satoshis:  sats,
		}
		hiddenmessagejson, err := json.Marshal(hiddenmessage)
		if err != nil {
			u.notify(t.ERROR, t.T{"Err": err.Error()})
			return
		}

		err = rds.Set(fmt.Sprintf("hidden:%d:%s", u.Id, hiddenid), string(hiddenmessagejson), s.HiddenMessageTimeout).Err()
		if err != nil {
			u.notify(t.ERROR, t.T{"Err": err.Error()})
			return
		}

		siq := "reveal " + hiddenid
		sendMessageWithKeyboard(u.ChatId,
			translateTemplate(t.HIDDENWITHID, u.Locale, t.T{
				"HiddenId": hiddenid,
				"Message":  hiddenmessage,
			}),
			&tgbotapi.InlineKeyboardMarkup{
				InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
					{
						tgbotapi.InlineKeyboardButton{
							Text:              translate(t.HIDDENSHAREBTN, u.Locale),
							SwitchInlineQuery: &siq,
						},
					},
				},
			}, message.MessageID,
		)

		go u.track("hide", map[string]interface{}{
			"sats":      hiddenmessage.Satoshis,
			"times":     hiddenmessage.Times,
			"crowdfund": hiddenmessage.Crowdfund,
			"public":    hiddenmessage.Public,
		})

		break
	case opts["reveal"].(bool):
		go func() {
			hiddenid := opts["<hidden_message_id>"].(string)

			redisKey, ok := findHiddenKey(hiddenid)
			if !ok {
				u.notifyAsReply(t.HIDDENMSGNOTFOUND, nil, message.MessageID)
				return
			}

			_, _, hidden, err := getHiddenMessage(redisKey, g.Locale)
			if err != nil {
				u.notify(t.ERROR, t.T{"Err": err.Error()})
				return
			}

			sendMessageWithKeyboard(u.ChatId, hidden.Preview, revealKeyboard(redisKey, hidden, 0, g.Locale), 0)
		}()
	case opts["transactions"].(bool):
		go func() {
			page, _ := opts.Int("--page")
			filter := Both
			if opts["--in"].(bool) {
				filter = In
			} else if opts["--out"].(bool) {
				filter = Out
			}
			handleTransactionList(u, page, filter, nil)

			go u.track("txlist", nil)
		}()
	case opts["balance"].(bool):
		go func() {
			go u.track("balance", map[string]interface{}{"apps": opts["apps"].(bool)})

			if opts["apps"].(bool) {
				// balance of apps
				taggedbalances, err := u.getTaggedBalances()
				if err != nil {
					log.Warn().Err(err).Str("user", u.Username).Msg("failed to get info")
					u.notify(t.ERROR, t.T{"Err": err.Error()})
					return
				}

				u.notify(t.TAGGEDBALANCEMSG, t.T{"Balances": taggedbalances})
			} else {
				// normal balance
				info, err := u.getInfo()
				if err != nil {
					log.Warn().Err(err).Str("user", u.Username).Msg("failed to get info")
					u.notify(t.ERROR, t.T{"Err": err.Error()})
					return
				}

				u.notify(t.BALANCEMSG, t.T{
					"Sats":     info.Balance,
					"Usable":   info.UsableBalance,
					"Received": info.TotalReceived,
					"Sent":     info.TotalSent,
					"Fees":     info.TotalFees,
				})
			}
		}()
	case opts["pay"].(bool), opts["withdraw"].(bool), opts["decode"].(bool):
		if opts["lnurl"].(bool) {
			// generate an lnurl-withdraw so a remote wallet can send an invoice
			sats, err := opts.Int("<satoshis>")
			if err != nil {
				u.notify(t.INVALIDAMOUNT, t.T{"Amount": opts["<satoshis>"]})
				break
			}
			handleLNCreateLNURLWithdraw(u, sats, message.MessageID)

			go u.track("lnurl generate", map[string]interface{}{"sats": sats})
		} else {
			// normal payment flow
			handlePay(u, opts, message.MessageID, message.ReplyToMessage)
		}
		break
	case opts["receive"].(bool), opts["invoice"].(bool), opts["fund"].(bool):
		go func() {
			if opts["lnurl"].(bool) {
				// print static lnurl-pay for this user
				lnurl, _ := lnurl.LNURLEncode(fmt.Sprintf("%s/lnurl/pay?userid=%d", s.ServiceURL, u.Id))
				qrpath := qrImagePath(fmt.Sprintf("lnurlpay-%d", u.Id))
				qrcode.WriteFile(lnurl, qrcode.Medium, 256, qrpath)
				sendMessageWithPicture(message.Chat.ID, qrpath, lnurl)

				go u.track("print lnurl", nil)
			} else {
				sats, err := opts.Int("<satoshis>")
				if err != nil {
					if opts["any"].(bool) {
						sats = 0
					} else {
						handleHelp(u, "receive")
						return
					}
				}

				go u.track("make invoice", map[string]interface{}{"sats": sats})

				desc := getVariadicFieldOrReplyToContent(opts, message, "<description>")

				bolt11, _, qrpath, err := u.makeInvoice(makeInvoiceArgs{
					Msatoshi:  int64(sats) * 1000,
					Desc:      desc,
					MessageId: message.MessageID,
				})
				if err != nil {
					log.Warn().Err(err).Msg("failed to generate invoice")
					u.notify(t.FAILEDINVOICE, t.T{"Err": messageFromError(err)})
					return
				}

				// send invoice with qr code
				sendMessageWithPicture(message.Chat.ID, qrpath, bolt11)
			}
		}()
	case opts["lnurl"].(bool):
		go handleLNURL(u, opts["<lnurl>"].(string), handleLNURLOpts{
			messageId: message.MessageID,
		})
	case opts["rename"].(bool):
		go func() {
			if message.Chat.Type == "private" {
				return
			}

			name := opts["<name>"].(string)

			var price int
			pg.Get(&price,
				"SELECT renamable FROM telegram.chat WHERE telegram_id = $1",
				-message.Chat.ID)
			if price == 0 {
				g.notify(t.GROUPNOTRENAMABLE, nil)
				return
			}
			if !isAdmin(message.Chat, &bot.Self) {
				g.notify(t.GROUPNOTRENAMABLE, nil)
				return
			}

			sendMessageWithKeyboard(
				message.Chat.ID,
				translateTemplate(t.RENAMEPROMPT, g.Locale, t.T{
					"Sats": price,
					"Name": name,
				}),
				renameKeyboard(message.From.ID,
					message.Chat.ID, price, name, g.Locale),
				message.MessageID,
			)

			go u.track("rename started", map[string]interface{}{
				"group": message.Chat.ID,
				"sats":  price,
			})
		}()
	case opts["apps"].(bool):
		go u.track("apps", nil)
		handleTutorial(u, "apps")
		break
	case opts["help"].(bool):
		command, _ := opts.String("<command>")
		go u.track("help", map[string]interface{}{"command": command})
		handleHelp(u, command)
		break
	case opts["toggle"].(bool):
		go func() {
			if message.Chat.Type == "private" {
				// on private chats we can use /toggle language <lang>, nothing else
				switch {
				case opts["language"].(bool):
					if lang, err := opts.String("<lang>"); err == nil {
						go u.track("toggle language", map[string]interface{}{
							"lang":     lang,
							"personal": true,
						})
						log.Info().Str("user", u.Username).Str("language", lang).Msg("toggling language")
						err := setLanguage(u.ChatId, lang)
						if err != nil {
							log.Warn().Err(err).Msg("failed to toggle language")
							u.notify(t.ERROR, t.T{"Err": err.Error()})
							break
						}
						u.notify(t.LANGUAGEMSG, t.T{"Language": lang})
					} else {
						u.notify(t.LANGUAGEMSG, t.T{"Language": u.Locale})
					}
				}

				return
			}
			if !isAdmin(message.Chat, message.From) {
				return
			}

			g, err := ensureGroup(message.Chat.ID, message.From.LanguageCode)
			if err != nil {
				log.Warn().Err(err).Str("user", u.Username).Int64("group", message.Chat.ID).Msg("failed to ensure group")
				return
			}

			switch {
			case opts["ticket"].(bool):
				log.Info().Int64("group", message.Chat.ID).Msg("toggling ticket")
				price, err := opts.Int("<price>")
				if err != nil {
					setTicketPrice(message.Chat.ID, 0)
					g.notify(t.FREEJOIN, nil)
				}

				go u.track("toggle ticket", map[string]interface{}{
					"group": message.Chat.ID,
					"sats":  price,
				})

				setTicketPrice(message.Chat.ID, price)
				if price > 0 {
					g.notify(t.TICKETMSG, t.T{
						"Sat":     price,
						"BotName": s.ServiceId,
					})
				}
			case opts["renamable"].(bool):
				log.Info().Int64("group", message.Chat.ID).Msg("toggling renamable")
				price, err := opts.Int("<price>")
				if err != nil {
					setTicketPrice(message.Chat.ID, 0)
					g.notify(t.FREEJOIN, nil)
				}

				go u.track("toggle renamable", map[string]interface{}{
					"group": message.Chat.ID,
					"sats":  price,
				})

				setRenamablePrice(message.Chat.ID, price)
				if price > 0 {
					g.notify(t.RENAMABLEMSG, t.T{
						"Sat":     price,
						"BotName": s.ServiceId,
					})
				}
			case opts["spammy"].(bool):
				log.Debug().Int64("group", message.Chat.ID).Msg("toggling spammy")
				spammy, err := toggleSpammy(message.Chat.ID)
				if err != nil {
					log.Warn().Err(err).Msg("failed to toggle spammy")
					g.notify(t.ERROR, t.T{"Err": err.Error()})
					break
				}

				go u.track("toggle spammy", map[string]interface{}{
					"group":  message.Chat.ID,
					"spammy": spammy,
				})

				g.notify(t.SPAMMYMSG, t.T{"Spammy": spammy})
			case opts["coinflips"].(bool):
				log.Debug().Int64("group", message.Chat.ID).Msg("toggling coinflips")
				enabled, err := toggleCoinflips(message.Chat.ID)
				if err != nil {
					log.Warn().Err(err).Msg("failed to toggle coinflips")
					g.notify(t.ERROR, t.T{"Err": err.Error()})
					break
				}

				go u.track("toggle coinflips", map[string]interface{}{
					"group":   message.Chat.ID,
					"enabled": enabled,
				})

				g.notify(t.COINFLIPSENABLEDMSG, t.T{"Enabled": enabled})
			case opts["language"].(bool):
				if lang, err := opts.String("<lang>"); err == nil {
					log.Info().Int64("group", message.Chat.ID).Str("language", lang).Msg("toggling language")
					err := setLanguage(message.Chat.ID, lang)
					if err != nil {
						log.Warn().Err(err).Msg("failed to toggle language")
						u.notify(t.ERROR, t.T{"Err": err.Error()})
						break
					}

					go u.track("toggle language", map[string]interface{}{
						"group": message.Chat.ID,
						"lang":  lang,
					})

					g.notify(t.LANGUAGEMSG, t.T{"Language": lang})
				} else {
					g.notify(t.LANGUAGEMSG, t.T{"Language": g.Locale})
				}

			}
		}()
	}
}

func handleEditedMessage(message *tgbotapi.Message) {
	// is this a hidden message?
	_, ok := findHiddenKey(getHiddenId(message))
	if ok {
		// yes, so we'll process it again even though it wasn't wrong at the first try
		handleMessage(message)
		return
	}

	// proceed
	res, err := rds.Get(fmt.Sprintf("parseerror:%d", message.MessageID)).Result()
	if err != nil {
		return
	}

	if res != "1" {
		return
	}

	handleMessage(message)
}
