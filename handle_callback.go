package main

import (
	"math/rand"
	"strconv"
	"strings"
	"time"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/fiatjaf/lightningd-gjson-rpc"
	"github.com/go-telegram-bot-api/telegram-bot-api"
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
		page, _ := strconv.Atoi(cb.Data[7:])
		handleTransactionList(u, page, cb)
		goto answerEmpty
	case strings.HasPrefix(cb.Data, "cancel="):
		if strconv.Itoa(u.Id) != cb.Data[7:] {
			bot.AnswerCallbackQuery(tgbotapi.NewCallbackWithAlert(cb.ID, translate(t.CANTCANCEL, locale)))
			return
		}
		removeKeyboardButtons(cb)
		appendTextToMessage(cb, translate(t.CANCELED, locale))
		goto answerEmpty
	case strings.HasPrefix(cb.Data, "pay="):
		handlePayCallback(u, messageId, locale, cb)
		return
	case strings.HasPrefix(cb.Data, "give="):
		params := strings.Split(cb.Data[5:], "-")
		if len(params) != 3 {
			goto answerEmpty
		}

		buttonData := rds.Get("giveaway:" + params[2]).Val()
		if buttonData != cb.Data {
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, translate(t.CALLBACKBUTTONEXPIRED, locale))
			goto answerEmpty
		}
		if err = rds.Del("giveaway:" + params[2]).Err(); err != nil {
			log.Warn().Err(err).Str("id", params[2]).
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

		u, err := loadUser(fromid, 0)
		if err != nil {
			log.Warn().Err(err).
				Int("id", fromid).
				Msg("failed to load user")
			goto answerEmpty
		}

		claimer := u

		errMsg, err := u.sendInternally(messageId, claimer, false, sats*1000, "giveaway", nil)
		if err != nil {
			log.Warn().Err(err).Msg("failed to giveaway")
			claimer.notify(t.CALLBACKERROR, t.T{
				"BotOp": "giveaway",
				"Err":   errMsg,
			})
			goto answerEmpty
		}
		bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, "Payment sent."))
		removeKeyboardButtons(cb)
		claimer.notify(t.USERSENTYOUSATS, t.T{
			"User":  u.AtName(),
			"Sats":  sats,
			"BotOp": "/giveaway",
		})

		appendTextToMessage(cb,
			translateTemplate(t.GIVEAWAYSATSGIVENPUBLIC, locale, t.T{
				"From":             u.AtName(),
				"To":               claimer.AtName(),
				"Sats":             sats,
				"ClaimerHasNoChat": claimer.ChatId == 0,
				"BotName":          s.ServiceId,
			}),
		)
		return
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

		joiner := u

		if !canJoinCoinflip(joiner.Id) {
			bot.AnswerCallbackQuery(
				tgbotapi.NewCallbackWithAlert(cb.ID, translate(t.COINFLIPOVERQUOTA, joiner.Locale)))
			return
		}

		if !joiner.checkBalanceFor(sats, "coinflip") {
			goto answerEmpty
		}

		if isMember, err := rds.SIsMember(rkey, joiner.Id).Result(); err != nil || isMember {
			// can't join twice
			bot.AnswerCallbackQuery(tgbotapi.NewCallbackWithAlert(cb.ID, translate(t.CANTJOINTWICE, joiner.Locale)))
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

		joiner := u

		if joiner.Id == giverId {
			// giver can't join
			bot.AnswerCallbackQuery(tgbotapi.NewCallbackWithAlert(cb.ID, translate(t.GIVERCANTJOIN, joiner.Locale)))
			return
		}

		if isMember, err := rds.SIsMember(rkey, joiner.Id).Result(); err != nil || isMember {
			// can't join twice
			bot.AnswerCallbackQuery(tgbotapi.NewCallbackWithAlert(cb.ID, translate(t.CANTJOINTWICE, joiner.Locale)))
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

			errMsg, err := giver.sendInternally(messageId, winner, false, sats*1000, "giveflip", nil)
			if err != nil {
				log.Warn().Err(err).Msg("failed to giveflip")
				winner.notify(t.CLAIMFAILED, t.T{"BotOp": "giveflip", "Err": errMsg})
				goto answerEmpty
			}
			bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, "Payment sent."))
			removeKeyboardButtons(cb)
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

		return
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

		joiner := u

		if !joiner.checkBalanceFor(sats, "fundraise") {
			goto answerEmpty
		}

		if isMember, err := rds.SIsMember(rkey, joiner.Id).Result(); err != nil || isMember {
			// can't join twice
			bot.AnswerCallbackQuery(tgbotapi.NewCallbackWithAlert(cb.ID, translate(t.CANTJOINTWICE, joiner.Locale)))
			return
		}

		if err := rds.SAdd("fundraise:"+fundraiseid, joiner.Id).Err(); err != nil {
			log.Warn().Err(err).Str("fundraise", fundraiseid).Msg("error adding giver to fundraise.")
			goto answerEmpty
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
	case strings.HasPrefix(cb.Data, "reveal="):
		// locate hidden message with the key given in the callback data,
		// perform payment between users,
		// reveal message.
		hiddenkey := cb.Data[7:]
		sourceUserId, hiddenid, content, _, satoshis, err := getHiddenMessage(hiddenkey, locale)
		if err != nil {
			log.Error().Err(err).Str("key", hiddenkey).Msg("error locating hidden message")
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, translate(t.HIDDENMSGNOTFOUND, locale))
			bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, translate(t.HIDDENMSGFAIL, locale)))
			return
		}

		sourceuser, err := loadUser(sourceUserId, 0)
		if err != nil {
			log.Warn().Err(err).
				Int("id", sourceUserId).
				Msg("failed to load source user on reveal")
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, translate(t.ERROR, locale))
			bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, translate(t.HIDDENMSGFAIL, locale)))
			return
		}

		revealer := u

		errMsg, err := u.sendInternally(messageId, sourceuser, false, satoshis*1000, "reveal", nil)
		if err != nil {
			log.Warn().Err(err).Str("key", hiddenkey).Int("satoshis", satoshis).
				Str("username", cb.From.UserName).Int("tgid", cb.From.ID).
				Msg("failed to pay to reveal")
			bot.AnswerCallbackQuery(
				tgbotapi.NewCallback(cb.ID, translateTemplate(t.HIDDENMSGFAIL, locale, t.T{"Err": errMsg})),
			)
			return
		}

		// actually reveal
		if messageId != 0 {
			removeKeyboardButtons(cb)
			sendMessageAsReply(revealer.ChatId, content, messageId)
		} else {
			baseEdit := getBaseEdit(cb)
			bot.Send(tgbotapi.EditMessageTextConfig{
				BaseEdit: baseEdit,
				Text:     content,
			})
		}

		// notify both parties
		revealer.notify(t.HIDDENREVEAL, t.T{"Sats": satoshis, "Id": hiddenid})
		sourceuser.notify(t.HIDDENSOURCE, t.T{"Sats": satoshis, "Id": hiddenid, "Revealer": revealer.AtName()})

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
		go func(u User, messageId int, hash string) {
			payment, err := ln.Call("waitsendpay", hash)
			if err != nil {
				// an error we know it's a final error
				if cmderr, ok := err.(lightning.ErrorCommand); ok {
					if cmderr.Code == 203 || cmderr.Code == 208 || cmderr.Code == 209 {
						log.Debug().
							Err(err).
							Str("hash", hash).
							Msg("canceling failed payment because it has failed failed")
						paymentHasFailed(u, messageId, hash)
						return
					}

					// if it's not a final error but it's been a long time call it final
					if res, err := ln.CallNamed("listpayments", "payment_hash", hash); err == nil &&
						res.Get("payments.#").Int() == 1 &&
						time.Unix(res.Get("payments.0.created_at").Int(), 0).Add(time.Hour).
							Before(time.Now()) &&
						res.Get("payments.0.status").String() == "failed" {

						log.Debug().
							Err(err).
							Str("hash", hash).
							Str("pay", res.Get("payments.0").String()).
							Msg("canceling failed payment because it's been a long time")
						paymentHasFailed(u, messageId, hash)
					}
				}

				// unknown error, report
				log.Warn().Err(err).Str("hash", hash).Str("user", u.Username).
					Msg("unexpected error waiting payment resolution")
				appendTextToMessage(cb, translate(t.UNEXPECTED, locale))
				return
			}

			// payment succeeded
			paymentHasSucceeded(
				u,
				messageId,
				payment.Get("msatoshi").Float(),
				payment.Get("msatoshi_sent").Float(),
				payment.Get("payment_preimage").String(),
				payment.Get("payment_hash").String(),
			)
		}(u, txn.TriggerMessage, txn.Hash)
		appendTextToMessage(cb, translate(t.CHECKING, locale))
	case strings.HasPrefix(cb.Data, "app="):
		answer := handleExternalAppCallback(u, messageId, cb)
		bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, answer))
	}

answerEmpty:
	bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, ""))
}
