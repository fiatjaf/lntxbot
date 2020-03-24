package main

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/tidwall/gjson"
)

func handleCallback(cb *tgbotapi.CallbackQuery) {
	u, tcase, err := ensureUser(cb.From.ID, cb.From.UserName, cb.From.LanguageCode)
	if err != nil {
		log.Warn().Err(err).Int("case", tcase).
			Str("username", cb.From.UserName).
			Int("id", cb.From.ID).
			Msg("failed to ensure user on callback")
		return
	}

	// it's a game!
	if cb.GameShortName != "" {
		switch cb.GameShortName {
		case "poker":
			bot.AnswerCallbackQuery(tgbotapi.CallbackConfig{
				CallbackQueryID: cb.ID,
				URL:             getPokerURL(u),
			})
		}
		return
	}

	log.Debug().Str("d", cb.Data).Str("user", u.Username).Msg("got callback")

	var messageId int
	var locale string
	if cb.Message != nil {
		// we have access to the full message, means it was done through a /command
		messageId = cb.Message.MessageID

		if cb.Message.Chat != nil && cb.Message.Chat.Type != "private" {
			// it's a group. try to load the locale for the group.
			g, _ := loadGroup(cb.Message.Chat.ID)
			locale = g.Locale
		} else {
			// it's a private chat, probably.
			locale = u.Locale
		}
	} else {
		// we don't have access to the full message, means it was done through an inline query
		messageId = 0
		locale = u.Locale // since we don't have info about the group, default to the user locale.
	}

	switch {
	case cb.Data == "noop":
		goto answerEmpty
	case strings.HasPrefix(cb.Data, "txlist="):
		parts := strings.Split(cb.Data[7:], "-")
		page, _ := strconv.Atoi(parts[0])
		filter := InOut(parts[1])
		go handleTransactionList(u, page, filter, cb)
		go u.track("txlist page", map[string]interface{}{"page": page})
		goto answerEmpty
	case strings.HasPrefix(cb.Data, "cancel="):
		if strconv.Itoa(u.Id) != cb.Data[7:] {
			u.alert(cb, t.CANTCANCEL, nil)
			return
		}
		removeKeyboardButtons(cb)
		appendTextToMessage(cb, translate(t.CANCELED, locale))
		goto answerEmpty
	case strings.HasPrefix(cb.Data, "pay="):
		handlePayCallback(u, messageId, locale, cb)
		return
	case strings.HasPrefix(cb.Data, "lnurlpay="):
		defer removeKeyboardButtons(cb)
		msats, _ := strconv.ParseInt(cb.Data[9:], 10, 64)
		key := fmt.Sprintf("reply:%d:%d", u.Id, cb.Message.MessageID)
		if val, err := rds.Get(key).Result(); err == nil {
			data := gjson.Parse(val)
			handleLNURLPayConfirmation(u, msats, data, cb.Message.MessageID)
		}
		return
	case strings.HasPrefix(cb.Data, "give="):
		params := strings.Split(cb.Data[5:], "-")
		if len(params) != 3 {
			goto answerEmpty
		}

		giveId := params[2]
		buttonData := rds.Get("giveaway:" + giveId).Val()
		if buttonData != cb.Data {
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, translateTemplate(t.CALLBACKEXPIRED, locale, t.T{"BotOp": "Giveaway"}))
			goto answerEmpty
		}
		if err = rds.Del("giveaway:" + giveId).Err(); err != nil {
			log.Warn().Err(err).Str("id", giveId).
				Msg("error deleting giveaway check from redis")
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, translateTemplate(t.CALLBACKERROR, locale, t.T{"BotOp": "Giveaway"}))
			goto answerEmpty
		}

		fromid, err1 := strconv.Atoi(params[0])
		sats, err2 := strconv.Atoi(params[1])
		if err1 != nil || err2 != nil {
			goto answerEmpty
		}

		giver, err := loadUser(fromid, 0)
		if err != nil {
			log.Warn().Err(err).
				Int("id", fromid).
				Msg("failed to load user")
			goto answerEmpty
		}

		go u.track("giveaway joined", map[string]interface{}{"sats": sats})

		claimer := u

		if !canJoinGiveaway(claimer.Id) {
			u.notify(t.OVERQUOTA, t.T{"App": "giveaway"})
			return
		}

		errMsg, err := giver.sendInternally(
			messageId,
			claimer,
			false,
			sats*1000,
			"",
			calculateHash(giveId),
			"giveaway",
		)
		if err != nil {
			log.Warn().Err(err).Msg("failed to giveaway")
			claimer.alert(cb, t.ERROR, t.T{"Err": errMsg})
			return
		}

		removeKeyboardButtons(cb)
		claimer.notify(t.USERSENTYOUSATS, t.T{
			"User":  giver.AtName(),
			"Sats":  sats,
			"BotOp": "/giveaway",
		})

		giver.notify(t.USERSENTTOUSER, t.T{
			"Sats":              sats,
			"User":              claimer.AtName(),
			"ReceiverHasNoChat": false,
		})

		appendTextToMessage(cb,
			translateTemplate(t.GIVEAWAYSATSGIVENPUBLIC, locale, t.T{
				"From":             giver.AtName(),
				"To":               claimer.AtName(),
				"Sats":             sats,
				"ClaimerHasNoChat": claimer.ChatId == 0,
				"BotName":          s.ServiceId,
			}),
		)

		goto answerEmpty
	case strings.HasPrefix(cb.Data, "flip="):
		// join a new participant in a coinflip lottery
		// if the total of participants is reached run the coinflip
		params := strings.Split(cb.Data[5:], "-")
		if len(params) != 3 {
			goto answerEmpty
		}

		coinflipid := params[2]
		rkey := "coinflip:" + coinflipid

		nregistered := int(rds.SCard(rkey).Val())
		if nregistered == 0 {
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, translateTemplate(t.CALLBACKEXPIRED, locale, t.T{"BotOp": "Coinflip"}))
			goto answerEmpty
		}

		nparticipants, err1 := strconv.Atoi(params[0])
		sats, err2 := strconv.Atoi(params[1])
		if err1 != nil || err2 != nil {
			log.Warn().Err(err1).Err(err2).Msg("coinflip error")
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, translateTemplate(t.CALLBACKERROR, locale, t.T{"BotOp": "Coinflip"}))
			goto answerEmpty
		}

		go u.track("coinflip joined", map[string]interface{}{
			"sats": sats,
			"n":    nparticipants,
		})

		joiner := u

		if !canJoinCoinflip(joiner.Id) {
			u.alert(cb, t.OVERQUOTA, t.T{"App": "coinflip"})
			return
		}
		if !joiner.checkBalanceFor(sats, "coinflip", cb) {
			goto answerEmpty
		}

		if isMember, err := rds.SIsMember(rkey, joiner.Id).Result(); err != nil || isMember {
			// can't join twice
			u.alert(cb, t.CANTJOINTWICE, nil)
			return
		}

		if err := rds.SAdd("coinflip:"+coinflipid, joiner.Id).Err(); err != nil {
			log.Warn().Err(err).Str("coinflip", coinflipid).Msg("error adding participant to coinflip.")
			goto answerEmpty
		}

		if nregistered+1 < nparticipants {
			// append @user to the coinflip message (without removing the keyboard)
			baseEdit := getBaseEdit(cb)
			baseEdit.ReplyMarkup = coinflipKeyboard(coinflipid, 0, nparticipants, sats, locale)
			edit := tgbotapi.EditMessageTextConfig{BaseEdit: baseEdit}
			if messageId != 0 {
				edit.Text = cb.Message.Text + " " + joiner.AtName()
			} else {
				edit.Text = translateTemplate(t.COINFLIPAD, locale, t.T{
					"Sats":       sats,
					"Prize":      sats * nparticipants,
					"SpotsLeft":  nparticipants - nregistered,
					"MaxPlayers": nparticipants,
				})
			}
			bot.Send(edit)
		} else {
			// run the lottery
			// even if for some bug we registered more participants than we should
			// we run the lottery with them all
			sparticipants, err := rds.SMembers(rkey).Result()
			go rds.Del(rkey)
			if err != nil {
				log.Warn().Err(err).Msg("failed to get coinflip participants")
				removeKeyboardButtons(cb)
				appendTextToMessage(cb, translateTemplate(t.CALLBACKERROR, locale, t.T{"BotOp": "Coinflip"}))
				goto answerEmpty
			}
			swinnerId := sparticipants[rand.Intn(len(sparticipants))]

			// winner id
			winnerId, err := strconv.Atoi(swinnerId)
			if err != nil {
				log.Warn().Err(err).Str("winnerId", swinnerId).Msg("winner id is not an int")
				removeKeyboardButtons(cb)
				appendTextToMessage(cb, translateTemplate(t.CALLBACKERROR, locale, t.T{"BotOp": "Coinflip"}))
				goto answerEmpty
			}

			// all participants
			participants := make([]int, nregistered+1)
			for i, spart := range sparticipants {
				part, err := strconv.Atoi(spart)
				if err != nil {
					log.Warn().Err(err).Str("part", spart).Msg("participant id is not an int")
					removeKeyboardButtons(cb)
					appendTextToMessage(cb, translateTemplate(t.CALLBACKERROR, locale, t.T{"BotOp": "Coinflip"}))
					goto answerEmpty
				}
				participants[i] = part
			}

			winner, err := settleCoinflip(sats, winnerId, participants)
			if err != nil {
				log.Warn().Err(err).Msg("error processing coinflip transactions")
				removeKeyboardButtons(cb)
				appendTextToMessage(cb, translateTemplate(t.CALLBACKERROR, locale, t.T{"BotOp": "Coinflip"}))
				goto answerEmpty
			}

			removeKeyboardButtons(cb)
			if messageId != 0 {
				appendTextToMessage(cb, joiner.AtName()+"\n"+
					translateTemplate(t.CALLBACKWINNER, locale, t.T{"Winner": winner.AtName()}))
				sendMessageAsReply(
					cb.Message.Chat.ID,
					translateTemplate(
						t.CALLBACKCOINFLIPWINNER,
						locale,
						t.T{"Winner": winner.AtName()},
					),
					messageId,
				)
			} else {
				appendTextToMessage(cb, translateTemplate(t.CALLBACKCOINFLIPWINNER, locale, t.T{"Winner": winner.AtName()}))
			}
		}
	case strings.HasPrefix(cb.Data, "gifl="):
		// join a new participant in a giveflip lottery
		// if the total of participants is reached run the giveflip
		params := strings.Split(cb.Data[5:], "-")
		if len(params) != 4 {
			goto answerEmpty
		}

		giverId, err0 := strconv.Atoi(params[0])
		nparticipants, err1 := strconv.Atoi(params[1])
		sats, err2 := strconv.Atoi(params[2])
		if err0 != nil || err1 != nil || err2 != nil {
			log.Warn().Err(err0).Err(err1).Err(err2).Msg("giveflip error")
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, translateTemplate(t.CALLBACKERROR, locale, t.T{"BotOp": "Giveflip"}))
			goto answerEmpty
		}

		giveflipid := params[3]
		rkey := "giveflip:" + giveflipid

		nregistered := int(rds.SCard(rkey).Val())

		go u.track("coinflip joined", map[string]interface{}{
			"sats": sats,
			"n":    nparticipants,
		})

		joiner := u

		if !canJoinGiveflip(joiner.Id) {
			u.notify(t.OVERQUOTA, t.T{"App": "giveflip"})
			return
		}
		if joiner.Id == giverId {
			// giver can't join
			u.alert(cb, t.GIVERCANTJOIN, nil)
			return
		}

		if isMember, err := rds.SIsMember(rkey, joiner.Id).Result(); err != nil || isMember {
			// can't join twice
			u.alert(cb, t.CANTJOINTWICE, nil)
			return
		}

		if err := rds.SAdd("giveflip:"+giveflipid, joiner.Id).Err(); err != nil {
			log.Warn().Err(err).Str("giveflip", giveflipid).Msg("error adding participant to giveflip.")
			goto answerEmpty
		}
		rds.Expire("giveflip:"+giveflipid, s.GiveAwayTimeout)

		if nregistered+1 < nparticipants {
			// append @user to the giveflip message (without removing the keyboard)
			baseEdit := getBaseEdit(cb)
			baseEdit.ReplyMarkup = giveflipKeyboard(giveflipid, giverId, nparticipants, sats, locale)
			edit := tgbotapi.EditMessageTextConfig{BaseEdit: baseEdit}
			if messageId != 0 {
				edit.Text = cb.Message.Text + " " + joiner.AtName()
			} else {
				edit.Text = translateTemplate(t.GIVEFLIPAD, locale, t.T{
					"Sats":       sats,
					"SpotsLeft":  nparticipants - nregistered,
					"MaxPlayers": nparticipants,
				})
			}
			bot.Send(edit)
		} else {
			// even if for some bug we registered more participants than we should
			// we run the lottery with them all
			sparticipants, err := rds.SMembers(rkey).Result()
			go rds.Del(rkey)
			if err != nil {
				log.Warn().Err(err).Msg("failed to get giveflip participants")
				removeKeyboardButtons(cb)
				appendTextToMessage(cb, translateTemplate(t.CALLBACKERROR, locale, t.T{"BotOp": "Giveflip"}))
				goto answerEmpty
			}
			swinnerId := sparticipants[rand.Intn(len(sparticipants))]

			// winner
			winnerId, err := strconv.Atoi(swinnerId)
			if err != nil {
				log.Warn().Err(err).Str("winnerId", swinnerId).Msg("winner id is not an int")
				removeKeyboardButtons(cb)
				appendTextToMessage(cb, translateTemplate(t.CALLBACKERROR, locale, t.T{"BotOp": "Giveflip"}))
				goto answerEmpty
			}
			winner, err := loadUser(winnerId, 0)
			if err != nil {
				log.Warn().Err(err).Int("winnerId", winnerId).Msg("failed to load winner on giveflip")
				removeKeyboardButtons(cb)
				appendTextToMessage(cb, translateTemplate(t.CALLBACKERROR, locale, t.T{"BotOp": "Giveflip"}))
				goto answerEmpty
			}

			// giver
			giver, err := loadUser(giverId, 0)
			if err != nil {
				log.Warn().Err(err).Int("giverId", giverId).Msg("failed to load giver on giveflip")
				removeKeyboardButtons(cb)
				appendTextToMessage(cb, translateTemplate(t.CALLBACKERROR, locale, t.T{"BotOp": "Giveflip"}))
				goto answerEmpty
			}

			// get loser names
			var loserNames []string
			for _, spart := range sparticipants {
				partId, _ := strconv.Atoi(spart)
				if partId == winnerId {
					continue
				}

				loser, _ := loadUser(partId, 0)
				loserNames = append(loserNames, loser.AtName())
			}

			removeKeyboardButtons(cb)
			errMsg, err := giver.sendInternally(
				messageId,
				winner,
				false,
				sats*1000,
				"",
				calculateHash(giveflipid),
				"giveflip",
			)
			if err != nil {
				log.Warn().Err(err).Msg("failed to giveflip")
				winner.notify(t.CLAIMFAILED, t.T{"BotOp": "giveflip", "Err": errMsg})
				goto answerEmpty
			}

			winner.notify(t.USERSENTYOUSATS, t.T{"User": giver.AtName(), "Sats": sats, "BotOp": "/giveflip"})

			bot.Send(tgbotapi.EditMessageTextConfig{
				BaseEdit: getBaseEdit(cb),
				Text: translateTemplate(t.GIVEFLIPWINNERMSG, locale, t.T{
					"Receiver":          winner.AtName(),
					"Sats":              sats,
					"Sender":            giver.AtName(),
					"Losers":            strings.Join(loserNames, " "),
					"ReceiverHasNoChat": winner.ChatId == 0,
					"BotName":           s.ServiceId,
				}),
			})
		}

		goto answerEmpty
	case strings.HasPrefix(cb.Data, "raise="):
		// join a new giver in a fundraising event
		// if the total of givers is reached commit the fundraise
		params := strings.Split(cb.Data[6:], "-")
		if len(params) != 4 {
			goto answerEmpty
		}

		fundraiseid := params[3]
		rkey := "fundraise:" + fundraiseid

		nregistered := int(rds.SCard(rkey).Val())
		if nregistered == 0 {
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, translateTemplate(t.CALLBACKEXPIRED, locale, t.T{"BotOp": "Fundraise"}))
			goto answerEmpty
		}

		receiverId, err1 := strconv.Atoi(params[0])
		ngivers, err2 := strconv.Atoi(params[1])
		sats, err3 := strconv.Atoi(params[2])
		if err1 != nil || err2 != nil || err3 != nil {
			log.Warn().Err(err1).Err(err2).Err(err3).Msg("error parsing params on fundraise")
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, translateTemplate(t.CALLBACKERROR, locale, t.T{"BotOp": "Fundraise"}))
			goto answerEmpty
		}

		go u.track("fundraise joined", map[string]interface{}{
			"sats": sats,
			"n":    ngivers,
		})

		joiner := u

		if !joiner.checkBalanceFor(sats, "fundraise", cb) {
			goto answerEmpty
		}

		if isMember, err := rds.SIsMember(rkey, joiner.Id).Result(); err != nil || isMember {
			// can't join twice
			u.alert(cb, t.CANTJOINTWICE, nil)
			return
		}

		if err := rds.SAdd("fundraise:"+fundraiseid, joiner.Id).Err(); err != nil {
			log.Warn().Err(err).Str("fundraise", fundraiseid).Msg("error adding giver to fundraise.")
			u.alert(cb, t.ERROR, t.T{"Err": err.Error()})
			return
		}

		if nregistered+1 < ngivers {
			// append @user to the fundraise message (without removing the keyboard)
			baseEdit := getBaseEdit(cb)

			// we don't have to check for cb.Message/messageId here because we don't
			// allow fundraises as inline messages so we always have access to cb.Message
			baseEdit.ReplyMarkup = fundraiseKeyboard(fundraiseid, 0, receiverId, ngivers, sats, locale)
			edit := tgbotapi.EditMessageTextConfig{BaseEdit: baseEdit}
			edit.Text = cb.Message.Text + " " + joiner.AtName()
			bot.Send(edit)
		} else {
			// commit the fundraise. this is the same as the coinflip, just without randomness.
			sgivers, err := rds.SMembers(rkey).Result()
			go rds.Del(rkey)
			if err != nil {
				log.Warn().Err(err).Msg("failed to get fundraise givers")
				removeKeyboardButtons(cb)
				appendTextToMessage(cb, translateTemplate(t.CALLBACKERROR, locale, t.T{"BotOp": "Fundraise"}))
				goto answerEmpty
			}

			// all givers
			givers := make([]int, nregistered+1)
			for i, spart := range sgivers {
				part, err := strconv.Atoi(spart)
				if err != nil {
					log.Warn().Err(err).Str("part", spart).Msg("giver id is not an int")
					removeKeyboardButtons(cb)
					appendTextToMessage(cb, translateTemplate(t.CALLBACKERROR, locale, t.T{"BotOp": "Fundraise"}))
					goto answerEmpty
				}
				givers[i] = part
			}

			receiver, err := settleFundraise(sats, receiverId, givers)
			if err != nil {
				log.Warn().Err(err).Msg("error processing fundraise transactions")
				removeKeyboardButtons(cb)
				appendTextToMessage(cb, translateTemplate(t.CALLBACKERROR, locale, t.T{"BotOp": "Fundraise"}))
				goto answerEmpty
			}

			removeKeyboardButtons(cb)
			if messageId != 0 {
				appendTextToMessage(cb, joiner.AtName()+"\n"+translate(t.COMPLETED, locale))
				sendMessageAsReply(
					cb.Message.Chat.ID,
					translateTemplate(t.FUNDRAISECOMPLETE, locale, t.T{"Receiver": receiver.AtName()}),
					messageId,
				)
			} else {
				appendTextToMessage(cb,
					translateTemplate(t.FUNDRAISECOMPLETE, locale, t.T{"Receiver": receiver.AtName()}),
				)
			}
		}
	case strings.HasPrefix(cb.Data, "rnm"):
		// rename chat
		defer removeKeyboardButtons(cb)
		renameId := cb.Data[4:]
		data := rds.Get("rename:" + renameId).Val()
		parts := strings.Split(data, "|~|")
		if len(parts) != 3 {
			appendTextToMessage(cb, translate(t.ERROR, locale))
			log.Warn().Str("app", "rename").Msg("data isn't split in 3")
			return
		}
		chatId, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			appendTextToMessage(cb, translate(t.ERROR, locale))
			log.Warn().Err(err).Str("app", "rename").Msg("failed to parse chatId")
			return
		}
		sats, err := strconv.Atoi(parts[1])
		if err != nil {
			appendTextToMessage(cb, translate(t.ERROR, locale))
			log.Warn().Err(err).Str("app", "rename").Msg("failed to parse sats")
			return
		}
		name := parts[2]

		// transfer money
		owner, err := getChatOwner(chatId)
		if err != nil {
			appendTextToMessage(cb, translate(t.ERROR, locale))
			log.Warn().Err(err).Str("app", "rename").Msg("failed to get chat owner")
			return
		}

		random, err := randomPreimage()
		if err != nil {
			appendTextToMessage(cb, translate(t.ERROR, locale))
			log.Warn().Err(err).Str("app", "rename").Msg("failed to generate random")
			return
		}
		hash := calculateHash(random)

		errMessage, err := u.sendInternally(
			0, owner, false, sats*1000,
			fmt.Sprintf("Rename group %d to '%s'", chatId, name),
			hash, "rename",
		)
		if err != nil {
			appendTextToMessage(cb, translateTemplate(t.ERROR, locale, t.T{
				"Err": errMessage,
			}))
			return
		}

		// actually change the name
		_, err = bot.SetChatTitle(tgbotapi.SetChatTitleConfig{chatId, name})
		if err != nil {
			appendTextToMessage(cb, translateTemplate(t.ERROR, locale, t.T{
				"Err": "Unauthorized",
			}))
			return
		}

		go u.track("rename finish", map[string]interface{}{
			"group": chatId,
			"sats":  sats,
		})
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
			appendTextToMessage(cb, translate(t.ERROR, locale))
			return
		}
		appendTextToMessage(cb, translate(t.TXCANCELED, locale))

		go u.track("remove unclaimed", nil)
	case strings.HasPrefix(cb.Data, "reveal="):
		// locate hidden message with the key given in the callback data,
		// perform payment between users,
		// reveal message.
		parts := strings.Split(cb.Data[7:], "-")
		hiddenkey := parts[0]

		sourceUserId, hiddenid, hiddenmessage, err := getHiddenMessage(hiddenkey, locale)
		if err != nil {
			log.Error().Err(err).Str("key", hiddenkey).
				Msg("error locating hidden message")
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, translate(t.HIDDENMSGNOTFOUND, locale))
			u.alert(cb, t.HIDDENMSGNOTFOUND, nil)
			return
		}

		go u.track("reveal", map[string]interface{}{
			"sats":      hiddenmessage.Satoshis,
			"times":     hiddenmessage.Times,
			"crowdfund": hiddenmessage.Crowdfund,
			"public":    hiddenmessage.Public,
		})

		revealer := u

		// cache reveal so we know who has paid to reveal this for now
		revealerIds, totalrevealers, err := func() (revealerIds []int, totalrevealers int, err error) {
			revealedsetkey := fmt.Sprintf("revealed:%s", hiddenid)

			// also don't let users pay twice
			if alreadypaid, err := rds.SIsMember(revealedsetkey, u.Id).Result(); err != nil {
				return nil, 0, err
			} else if alreadypaid {
				return nil, 0, errors.New("Can't reveal twice.")
			}
			if err := rds.SAdd(revealedsetkey, u.Id).Err(); err != nil {
				return nil, 0, err
			}

			// expire this set after the hidden message has expired
			if err := rds.Expire(revealedsetkey, s.HiddenMessageTimeout).Err(); err != nil {
				return nil, 0, err
			}

			// get the count of people who paid to reveal up to now
			if revealerIdsStr, err := rds.SMembers(revealedsetkey).Result(); err != nil {
				return nil, 0, err
			} else {
				totalrevealers = len(revealerIdsStr)
				revealerIds := make([]int, totalrevealers)
				for i, revealerIdsStr := range revealerIdsStr {
					revealerId, err := strconv.Atoi(revealerIdsStr)
					if err != nil {
						return nil, 0, err
					}
					revealerIds[i] = revealerId
				}

				return revealerIds, totalrevealers, nil
			}
		}()
		if err != nil {
			u.alert(cb, t.ERROR, t.T{"Err": err.Error()})
			return
		}

		if hiddenmessage.Crowdfund > 1 && totalrevealers < hiddenmessage.Crowdfund {
			// if this is a crowdfund we must only reveal after the threshold of
			// participants has been reached. before that we will just update the message in-place.
			baseEdit := getBaseEdit(cb)
			baseEdit.ReplyMarkup = revealKeyboard(hiddenkey, hiddenmessage, totalrevealers, locale)
			bot.Send(tgbotapi.EditMessageTextConfig{
				BaseEdit:              baseEdit,
				Text:                  hiddenmessage.Preview,
				ParseMode:             "HTML",
				DisableWebPagePreview: true,
			})
			return
		}

		// send the satoshis.
		// if it's a crowdfunding we'll send from everybody at the same time,
		// otherwise just from the current revealer.
		if hiddenmessage.Crowdfund <= 1 {
			revealerIds = []int{u.Id}
		}

		_, err = settleReveal(hiddenmessage.Satoshis, hiddenid, sourceUserId, revealerIds)
		if err != nil {
			log.Warn().Err(err).Str("id", hiddenid).Int("satoshis", hiddenmessage.Satoshis).
				Str("revealer", revealer.Username).Msg("failed to pay to reveal")
			revealer.alert(cb, t.ERROR, t.T{"Err": err.Error()})
			return
		}

		// actually reveal
		if messageId == 0 { // was prompted from an inline query
			if hiddenmessage.Public {
				// reveal message in-place
				baseEdit := getBaseEdit(cb)
				bot.Send(tgbotapi.EditMessageTextConfig{
					BaseEdit:              baseEdit,
					Text:                  hiddenmessage.revealed(),
					ParseMode:             "HTML",
					DisableWebPagePreview: true,
				})
			} else {
				// reveal message privately
				sendMessageAsText(revealer.ChatId, hiddenmessage.revealed())
				if hiddenmessage.Times == 0 || hiddenmessage.Times > totalrevealers {
					// more people can still pay for this
					// buttons are kept so others still can pay, but updated
					baseEdit := getBaseEdit(cb)
					baseEdit.ReplyMarkup = revealKeyboard(hiddenkey, hiddenmessage, totalrevealers, locale)
					bot.Send(tgbotapi.EditMessageTextConfig{
						BaseEdit:              baseEdit,
						Text:                  hiddenmessage.Preview,
						ParseMode:             "HTML",
						DisableWebPagePreview: true,
					})
				} else {
					// end of quota. no more people can reveal.
					baseEdit := getBaseEdit(cb)
					bot.Send(tgbotapi.EditMessageTextConfig{
						BaseEdit:              baseEdit,
						Text:                  "A hidden message prompt once lived here.",
						ParseMode:             "HTML",
						DisableWebPagePreview: true,
					})
					removeKeyboardButtons(cb)
				}
			}
		} else {
			// called in the bot's chat
			removeKeyboardButtons(cb)
			sendMessageAsReply(revealer.ChatId, hiddenmessage.Content, messageId)
		}

		go u.track("reveal", map[string]interface{}{
			"sats":      hiddenmessage.Satoshis,
			"times":     hiddenmessage.Times,
			"crowdfund": hiddenmessage.Crowdfund,
			"public":    hiddenmessage.Public,
		})

		break
	case strings.HasPrefix(cb.Data, "check="):
		// recheck transaction when for some reason it wasn't checked and
		// either confirmed or deleted automatically
		hashfirstchars := cb.Data[6:]
		txn, err := u.getTransaction(hashfirstchars)
		if err != nil {
			log.Warn().Err(err).Str("hash", hashfirstchars).
				Msg("failed to fetch transaction for checking")
			appendTextToMessage(cb, translate(t.ERROR, locale))
			return
		}
		appendTextToMessage(cb, translate(t.CHECKING, locale))

		go u.track("check pending", nil)

		go func(u User, messageId int, hash string) {
			sendpays, _ := ln.CallNamed("listsendpays", "payment_hash", hash)
			if sendpays.Get("payments.#").Int() == 0 {
				// payment was never tried
				log.Debug().
					Err(err).
					Str("hash", hash).
					Msg("canceling payment because it is not on listsendpays")
				paymentHasFailed(u, messageId, hash)
				return
			}

			bolt11 := sendpays.Get("payments.0.bolt11").String()
			if bolt11 == "" {
				appendTextToMessage(cb, translate(t.UNEXPECTED, locale))
				return
			}
			pays, _ := ln.Call("listpays", bolt11)
			payment := pays.Get("listpays.0")
			if !payment.Exists() || payment.Get("status").String() == "failed" {
				// payment failed
				log.Debug().
					Err(err).
					Str("hash", hash).
					Str("pay", payment.String()).
					Msg("canceling failed payment")
				paymentHasFailed(u, messageId, hash)
				return
			} else if payment.Get("status").String() == "pending" {
				// command timed out, should try again later
				appendTextToMessage(cb, translate(t.TXPENDING, locale))
			} else {
				// payment succeeded
				paymentHasSucceeded(
					u,
					messageId,
					payment.Get("msatoshi").Float(),
					payment.Get("msatoshi_sent").Float(),
					payment.Get("payment_preimage").String(),
					"",
					payment.Get("payment_hash").String(),
				)
			}
		}(u, txn.TriggerMessage, txn.Hash)
	case strings.HasPrefix(cb.Data, "x="):
		// callback from external app
		answer := handleExternalAppCallback(u, messageId, cb)
		bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, answer))
	}

answerEmpty:
	bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, ""))
}
