package main

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/tidwall/gjson"
)

func handleTelegramCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	ctx = context.WithValue(ctx, "callbackQuery", cb)

	u, tcase, err := ensureTelegramUser(
		cb.From.ID, cb.From.UserName, cb.From.LanguageCode)
	if err != nil {
		log.Warn().Err(err).Int("case", tcase).
			Str("username", cb.From.UserName).
			Int("id", cb.From.ID).
			Msg("failed to ensure user on callback")
		return
	}

	log.Debug().Str("d", cb.Data).Str("user", u.Username).Msg("got callback")
	ctx = context.WithValue(ctx, "initiator", u)

	if cb.Message != nil {
		// we have access to the full message, means it was done through a /command
		ctx = context.WithValue(ctx, "message", cb.Message)

		if cb.Message.Chat != nil && cb.Message.Chat.Type != "private" {
			// it's a group. try to load the locale for the group.
			if g, err := loadGroup(cb.Message.Chat.ID); err == nil {
				ctx = context.WithValue(ctx, "group", g)
				ctx = context.WithValue(ctx, "locale", g.Locale)
			}
		} else {
			// it's a private chat, probably
			ctx = context.WithValue(ctx, "locale", u.Locale)
		}
	} else {
		// we don't have access to the full message,
		// means it was done through an inline query
		ctx = context.WithValue(ctx, "locale", u.Locale) // default to the user locale
	}

	switch {
	case cb.Data == "noop":
		goto answerEmpty
	case strings.HasPrefix(cb.Data, "txl="):
		parts := strings.Split(cb.Data[4:], "-")
		page, _ := strconv.Atoi(parts[0])
		filter := InOut(parts[1])
		tag := parts[2]
		go displayTransactionList(ctx, page, tag, filter)
		goto answerEmpty
	case strings.HasPrefix(cb.Data, "cancel="):
		if strconv.Itoa(u.Id) != cb.Data[7:] {
			send(ctx, cb, t.CANTCANCEL, WITHALERT)
			return
		}
		removeKeyboardButtons(ctx)
		send(ctx, t.CANCELED, APPEND)
		goto answerEmpty
	case strings.HasPrefix(cb.Data, "pay="):
		handlePayCallback(ctx)
		return
	case strings.HasPrefix(cb.Data, "lnurlpay="):
		defer removeKeyboardButtons(ctx)
		msats, _ := strconv.ParseInt(cb.Data[9:], 10, 64)
		key := fmt.Sprintf("reply:%d:%d", u.Id, cb.Message.MessageID)
		if val, err := rds.Get(key).Result(); err == nil {
			data := gjson.Parse(val)
			handleLNURLPayAmount(ctx, msats, data)
		}
		return
	case strings.HasPrefix(cb.Data, "lnurlall="):
		defer removeKeyboardButtons(ctx)
		msats, _ := strconv.ParseInt(cb.Data[9:], 10, 64)
		key := fmt.Sprintf("reply:%d:%d", u.Id, cb.Message.MessageID)
		if val, err := rds.Get(key).Result(); err == nil {
			data := gjson.Parse(val)
			handleLNURLAllowanceConfirmation(ctx, msats, data)
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
			removeKeyboardButtons(ctx)
			send(ctx, t.CALLBACKEXPIRED, t.T{"BotOp": "Giveaway"}, APPEND)
			goto answerEmpty
		}
		if err = rds.Del("giveaway:" + giveId).Err(); err != nil {
			log.Warn().Err(err).Str("id", giveId).
				Msg("error deleting giveaway check from redis")
			removeKeyboardButtons(ctx)
			send(ctx, t.CALLBACKERROR, t.T{"BotOp": "Giveaway"}, APPEND)
			goto answerEmpty
		}

		fromid, err1 := strconv.Atoi(params[0])
		sats, err2 := strconv.Atoi(params[1])
		if err1 != nil || err2 != nil {
			goto answerEmpty
		}

		giver, err := loadUser(fromid)
		if err != nil {
			log.Warn().Err(err).
				Int("id", fromid).
				Msg("failed to load user")
			goto answerEmpty
		}

		go u.track("giveaway joined", map[string]interface{}{"sats": sats})
		claimer := u

		if !canJoinGiveaway(claimer.Id) {
			send(ctx, u, t.OVERQUOTA, t.T{"App": "giveaway"})
			return
		}

		err = giver.sendInternally(
			ctx,
			claimer,
			false,
			sats*1000,
			"",
			hashString(giveId),
			"giveaway",
		)
		if err != nil {
			log.Warn().Err(err).Msg("failed to giveaway")
			send(ctx, claimer, t.ERROR, t.T{"Err": err.Error()}, WITHALERT)
			return
		}

		removeKeyboardButtons(ctx)
		send(ctx, claimer, t.USERSENTYOUSATS, t.T{
			"User":    giver.AtName(ctx),
			"Sats":    sats,
			"RawSats": "",
			"BotOp":   "/giveaway",
		})

		send(ctx, giver, t.USERSENTTOUSER, t.T{
			"User":              claimer.AtName(ctx),
			"Sats":              sats,
			"RawSats":           "",
			"ReceiverHasNoChat": false,
		})

		if imessage := ctx.Value("message"); imessage != nil {
			message := imessage.(*tgbotapi.Message)
			send(ctx, message.Chat.ID, FORCESPAMMY, t.GIVEAWAYSATSGIVENPUBLIC, t.T{
				"From":             giver.AtName(ctx),
				"To":               claimer.AtName(ctx),
				"Sats":             sats,
				"ClaimerHasNoChat": claimer.TelegramChatId == 0,
				"BotName":          s.ServiceId,
			}, message.MessageID)
		}

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
			removeKeyboardButtons(ctx)
			send(ctx, t.CALLBACKEXPIRED, t.T{"BotOp": "Coinflip"}, APPEND)
			goto answerEmpty
		}

		nparticipants, err1 := strconv.Atoi(params[0])
		sats, err2 := strconv.Atoi(params[1])
		if err1 != nil || err2 != nil {
			log.Warn().Err(err1).Err(err2).Msg("coinflip error")
			removeKeyboardButtons(ctx)
			send(ctx, t.CALLBACKERROR, t.T{"BotOp": "Coinflip"}, APPEND)
			goto answerEmpty
		}

		go u.track("coinflip joined", map[string]interface{}{
			"sats": sats,
			"n":    nparticipants,
		})

		joiner := u

		if !canJoinCoinflip(joiner.Id) {
			send(ctx, t.OVERQUOTA, t.T{"App": "coinflip"}, WITHALERT)
			return
		}
		if !joiner.checkBalanceFor(ctx, sats, "coinflip") {
			goto answerEmpty
		}

		if isMember, err := rds.SIsMember(rkey, joiner.Id).Result(); err != nil || isMember {
			// can't join twice
			send(ctx, t.CANTJOINTWICE, WITHALERT)
			return
		}

		if err := rds.SAdd("coinflip:"+coinflipid, joiner.Id).Err(); err != nil {
			log.Warn().Err(err).Str("coinflip", coinflipid).Msg("error adding participant to coinflip.")
			goto answerEmpty
		}

		if nregistered+1 < nparticipants {
			// append @user to the coinflip message (without removing the keyboard)
			keyboard := coinflipKeyboard(ctx, coinflipid, 0, nparticipants, sats)

			if message := ctx.Value("message"); message != nil {
				send(ctx, message, joiner.AtName(ctx), APPEND, keyboard)
			} else {
				send(ctx, t.COINFLIPAD, t.T{
					"Sats":       sats,
					"Prize":      sats * nparticipants,
					"SpotsLeft":  nparticipants - nregistered,
					"MaxPlayers": nparticipants,
				}, EDIT, keyboard)
			}
		} else {
			// run the lottery
			// even if for some bug we registered more participants than we should
			// we run the lottery with them all
			sparticipants, err := rds.SMembers(rkey).Result()
			go rds.Del(rkey)
			if err != nil {
				log.Warn().Err(err).Msg("failed to get coinflip participants")
				removeKeyboardButtons(ctx)
				send(ctx, t.CALLBACKERROR, t.T{"BotOp": "Coinflip"}, APPEND)
				goto answerEmpty
			}
			swinnerId := sparticipants[rand.Intn(len(sparticipants))]

			// winner id
			winnerId, err := strconv.Atoi(swinnerId)
			if err != nil {
				log.Warn().Err(err).Str("winnerId", swinnerId).Msg("winner id is not an int")
				removeKeyboardButtons(ctx)
				send(ctx, t.CALLBACKERROR, t.T{"BotOp": "Coinflip"}, APPEND)
				goto answerEmpty
			}

			// all participants
			participants := make([]int, nregistered+1)
			for i, spart := range sparticipants {
				part, err := strconv.Atoi(spart)
				if err != nil {
					log.Warn().Err(err).Str("part", spart).
						Msg("participant id is not an int")
					removeKeyboardButtons(ctx)
					send(ctx, t.CALLBACKERROR, t.T{"BotOp": "Coinflip"}, APPEND)
					goto answerEmpty
				}
				participants[i] = part
			}

			winner, err := settleCoinflip(ctx, sats, winnerId, participants)
			if err != nil {
				log.Warn().Err(err).Msg("error processing coinflip transactions")
				removeKeyboardButtons(ctx)
				send(ctx, t.CALLBACKERROR, t.T{"BotOp": "Coinflip"}, APPEND)
				goto answerEmpty
			}

			removeKeyboardButtons(ctx)
			if imessage := ctx.Value("message"); imessage != nil {
				message := imessage.(*tgbotapi.Message)
				send(ctx, message, EDIT, joiner.AtName(ctx)+"\n"+
					translateTemplate(ctx, t.CALLBACKWINNER, t.T{
						"Winner": winner.AtName(ctx),
					}))
				send(ctx, message.Chat.ID, FORCESPAMMY, t.CALLBACKCOINFLIPWINNER,
					t.T{"Winner": winner.AtName(ctx)}, message.MessageID)
			} else {
				send(ctx, t.CALLBACKCOINFLIPWINNER, t.T{"Winner": winner.AtName(ctx)},
					APPEND)
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
			removeKeyboardButtons(ctx)
			send(ctx, t.CALLBACKERROR, t.T{"BotOp": "Giveflip"}, APPEND)
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
			send(ctx, u, t.OVERQUOTA, t.T{"App": "giveflip"})
			return
		}
		if joiner.Id == giverId {
			// giver can't join
			send(ctx, t.GIVERCANTJOIN, WITHALERT)
			return
		}

		if isMember, err := rds.SIsMember(rkey, joiner.Id).Result(); err != nil || isMember {
			// can't join twice
			send(ctx, t.CANTJOINTWICE, WITHALERT)
			return
		}

		if err := rds.SAdd("giveflip:"+giveflipid, joiner.Id).Err(); err != nil {
			log.Warn().Err(err).Str("giveflip", giveflipid).Msg("error adding participant to giveflip.")
			goto answerEmpty
		}
		rds.Expire("giveflip:"+giveflipid, s.GiveAwayTimeout)

		if nregistered+1 < nparticipants {
			// append @user to the giveflip message (without removing the keyboard)

			keyboard := giveflipKeyboard(ctx, giveflipid, giverId, nparticipants, sats)
			if message := ctx.Value("message"); message != nil {
				send(ctx, message, keyboard, joiner.AtName(ctx), APPEND)
			} else {
				send(ctx, t.GIVEFLIPAD, t.T{
					"Sats":       sats,
					"SpotsLeft":  nparticipants - nregistered,
					"MaxPlayers": nparticipants,
				}, EDIT)
			}
		} else {
			// even if for some bug we registered more participants than we should
			// we run the lottery with them all
			sparticipants, err := rds.SMembers(rkey).Result()
			go rds.Del(rkey)
			if err != nil {
				log.Warn().Err(err).Msg("failed to get giveflip participants")
				removeKeyboardButtons(ctx)
				send(ctx, t.CALLBACKERROR, t.T{"BotOp": "Giveflip"}, APPEND)
				goto answerEmpty
			}
			swinnerId := sparticipants[rand.Intn(len(sparticipants))]

			// winner
			winnerId, err := strconv.Atoi(swinnerId)
			if err != nil {
				log.Warn().Err(err).Str("winnerId", swinnerId).Msg("winner id is not an int")
				removeKeyboardButtons(ctx)
				send(ctx, t.CALLBACKERROR, t.T{"BotOp": "Giveflip"}, APPEND)
				goto answerEmpty
			}
			winner, err := loadUser(winnerId)
			if err != nil {
				log.Warn().Err(err).Int("winnerId", winnerId).Msg("failed to load winner on giveflip")
				removeKeyboardButtons(ctx)
				send(ctx, t.CALLBACKERROR, t.T{"BotOp": "Giveflip"}, APPEND)
				goto answerEmpty
			}

			// giver
			giver, err := loadUser(giverId)
			if err != nil {
				log.Warn().Err(err).Int("giverId", giverId).
					Msg("failed to load giver on giveflip")
				removeKeyboardButtons(ctx)
				send(ctx, t.CALLBACKERROR, t.T{"BotOp": "Giveflip"}, APPEND)
				goto answerEmpty
			}

			// get loser names
			var loserNames []string
			for _, spart := range sparticipants {
				partId, _ := strconv.Atoi(spart)
				if partId == winnerId {
					continue
				}

				loser, _ := loadUser(partId)
				loserNames = append(loserNames, loser.AtName(ctx))
			}

			removeKeyboardButtons(ctx)
			err = giver.sendInternally(
				ctx,
				winner,
				false,
				sats*1000,
				"",
				hashString(giveflipid),
				"giveflip",
			)
			if err != nil {
				log.Warn().Err(err).Msg("failed to giveflip")
				send(ctx, winner, t.CLAIMFAILED,
					t.T{"BotOp": "giveflip", "Err": err.Error()})
				goto answerEmpty
			}

			send(ctx, winner, t.USERSENTYOUSATS, t.T{
				"User":  giver.AtName(ctx),
				"Sats":  sats,
				"BotOp": "/giveflip", "RawSats": "",
			})

			send(ctx, t.GIVEFLIPWINNERMSG, t.T{
				"Receiver":          winner.AtName(ctx),
				"Sats":              sats,
				"Sender":            giver.AtName(ctx),
				"Losers":            strings.Join(loserNames, " "),
				"ReceiverHasNoChat": winner.TelegramChatId == 0,
				"BotName":           s.ServiceId,
			}, EDIT)
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
			removeKeyboardButtons(ctx)
			send(ctx, t.CALLBACKEXPIRED, t.T{"BotOp": "Fundraise"}, APPEND)
			goto answerEmpty
		}

		receiverId, err1 := strconv.Atoi(params[0])
		ngivers, err2 := strconv.Atoi(params[1])
		sats, err3 := strconv.Atoi(params[2])
		if err1 != nil || err2 != nil || err3 != nil {
			log.Warn().Err(err1).Err(err2).Err(err3).Msg("error parsing params on fundraise")
			removeKeyboardButtons(ctx)
			send(ctx, t.CALLBACKERROR, t.T{"BotOp": "Fundraise"}, APPEND)
			goto answerEmpty
		}

		go u.track("fundraise joined", map[string]interface{}{
			"sats": sats,
			"n":    ngivers,
		})

		joiner := u

		if !joiner.checkBalanceFor(ctx, sats, "fundraise") {
			goto answerEmpty
		}

		if isMember, err := rds.SIsMember(rkey, joiner.Id).Result(); err != nil || isMember {
			// can't join twice
			send(ctx, t.CANTJOINTWICE, WITHALERT)
			return
		}

		if err := rds.SAdd("fundraise:"+fundraiseid, joiner.Id).Err(); err != nil {
			log.Warn().Err(err).Str("fundraise", fundraiseid).
				Msg("error adding giver to fundraise.")
			send(ctx, t.ERROR, t.T{"Err": err.Error()}, WITHALERT)
			return
		}

		if nregistered+1 < ngivers {
			// append @user to the fundraise message (without removing the keyboard)
			// we don't have to check for cb.Message/messageId here because we don't
			// allow fundraises as inline messages so we always have access to
			// cb.Message.

			send(ctx, ctx.Value("message"), APPEND, joiner.AtName(ctx),
				fundraiseKeyboard(ctx, fundraiseid, 0, receiverId, ngivers, sats))
		} else {
			// commit the fundraise. this is the same as the coinflip,
			// just without randomness.
			sgivers, err := rds.SMembers(rkey).Result()
			go rds.Del(rkey)
			if err != nil {
				log.Warn().Err(err).Msg("failed to get fundraise givers")
				removeKeyboardButtons(ctx)
				send(ctx, t.CALLBACKERROR, t.T{"BotOp": "Fundraise"}, APPEND)
				goto answerEmpty
			}

			// all givers
			givers := make([]int, nregistered+1)
			for i, spart := range sgivers {
				part, err := strconv.Atoi(spart)
				if err != nil {
					log.Warn().Err(err).Str("part", spart).
						Msg("giver id is not an int")
					removeKeyboardButtons(ctx)
					send(ctx, t.CALLBACKERROR, t.T{"BotOp": "Fundraise"}, APPEND)
					goto answerEmpty
				}
				givers[i] = part
			}

			receiver, err := settleFundraise(ctx, sats, receiverId, givers)
			if err != nil {
				log.Warn().Err(err).Msg("error processing fundraise transactions")
				removeKeyboardButtons(ctx)
				send(ctx, t.CALLBACKERROR, t.T{"BotOp": "Fundraise"}, APPEND)
				goto answerEmpty
			}

			removeKeyboardButtons(ctx)
			send(ctx, APPEND, ctx.Value("message"),
				joiner.AtName(ctx)+"\n"+translate(ctx, t.COMPLETED))
			send(ctx, cb.Message.Chat.ID, FORCESPAMMY, ctx.Value("message"),
				t.FUNDRAISECOMPLETE, t.T{"Receiver": receiver.AtName(ctx)})
		}
	case strings.HasPrefix(cb.Data, "rnm"):
		// rename chat
		defer removeKeyboardButtons(ctx)
		renameId := cb.Data[4:]
		data := rds.Get("rename:" + renameId).Val()
		parts := strings.Split(data, "|~|")
		if len(parts) != 3 {
			send(ctx, t.ERROR, APPEND)
			log.Warn().Str("app", "rename").Msg("data isn't split in 3")
			return
		}
		chatId, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			send(ctx, t.ERROR, APPEND)
			log.Warn().Err(err).Str("app", "rename").Msg("failed to parse chatId")
			return
		}
		sats, err := strconv.Atoi(parts[1])
		if err != nil {
			send(ctx, t.ERROR, APPEND)
			log.Warn().Err(err).Str("app", "rename").Msg("failed to parse sats")
			return
		}
		name := parts[2]

		// transfer money
		owner, err := getChatOwner(chatId)
		if err != nil {
			send(ctx, t.ERROR, APPEND)
			log.Warn().Err(err).Str("app", "rename").Msg("failed to get chat owner")
			return
		}

		random, err := randomPreimage()
		if err != nil {
			send(ctx, t.ERROR, APPEND)
			log.Warn().Err(err).Str("app", "rename").Msg("failed to generate random")
			return
		}
		hash := hashString(random)

		err = u.sendInternally(
			ctx, owner, false, sats*1000,
			fmt.Sprintf("Rename group %d to '%s'", chatId, name),
			hash, "rename",
		)
		if err != nil {
			send(ctx, t.ERROR, t.T{"Err": err.Error()}, APPEND)
			return
		}

		// actually change the name
		_, err = bot.SetChatTitle(tgbotapi.SetChatTitleConfig{chatId, name})
		if err != nil {
			send(ctx, t.ERROR, t.T{"Err": "Unauthorized"}, APPEND)
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
			send(ctx, t.ERROR, APPEND)
			return
		}
		send(ctx, t.TXCANCELED, APPEND)

		go u.track("remove unclaimed", nil)
	case strings.HasPrefix(cb.Data, "reveal="):
		// locate hidden message with the key given in the callback data,
		// perform payment between users,
		// reveal message.
		parts := strings.Split(cb.Data[7:], "-")
		hiddenkey := parts[0]

		sourceUserId, hiddenid, hiddenmessage, err := getHiddenMessage(ctx, hiddenkey)
		if err != nil {
			log.Error().Err(err).Str("key", hiddenkey).
				Msg("error locating hidden message")
			removeKeyboardButtons(ctx)
			send(ctx, t.HIDDENMSGNOTFOUND, APPEND)
			send(ctx, u, WITHALERT, t.HIDDENMSGNOTFOUND, nil)
			return
		}

		go u.track("reveal", map[string]interface{}{
			"sats":      hiddenmessage.Satoshis,
			"times":     hiddenmessage.Times,
			"crowdfund": hiddenmessage.Crowdfund,
			"public":    hiddenmessage.Public,
		})

		revealer := u
		didReveal := false

		// cache reveal so we know who has paid to reveal this for now
		var revealerIds []int
		var totalrevealers int

		revealedsetkey := fmt.Sprintf("revealed:%s", hiddenid)

		// also don't let users pay twice
		if alreadypaid, err := rds.SIsMember(revealedsetkey, u.Id).Result(); err != nil {
			send(ctx, u, WITHALERT, t.ERROR, t.T{"Err": err.Error()})
			return
		} else if alreadypaid {
			send(ctx, u, WITHALERT, t.ERROR, t.T{"Err": "can't reveal twice"})
			return
		}

		defer func(u User, revealedsetkey string) {
			if !didReveal {
				return
			}
			rds.SAdd(revealedsetkey, u.Id)
			rds.Expire(revealedsetkey, s.HiddenMessageTimeout).Err()
		}(u, revealedsetkey)

		// get the count of people who paid to reveal up to now
		if revealerIdsStr, err := rds.SMembers(revealedsetkey).Result(); err != nil {
			send(ctx, u, WITHALERT, t.ERROR, t.T{"Err": err.Error()})
			return
		} else {
			totalrevealers = len(revealerIdsStr)
			revealerIds := make([]int, totalrevealers)
			for i, revealerIdsStr := range revealerIdsStr {
				revealerId, err := strconv.Atoi(revealerIdsStr)
				if err != nil {
					send(ctx, u, WITHALERT, t.ERROR, t.T{"Err": err.Error()})
					return
				}
				revealerIds[i] = revealerId
			}
		}

		if hiddenmessage.Crowdfund > 1 && totalrevealers < hiddenmessage.Crowdfund {
			// if this is a crowdfund we must only reveal after the threshold of
			// participants has been reached. before that we will just update the
			// message in-place.
			send(ctx, hiddenmessage.Preview,
				revealKeyboard(ctx, hiddenkey, hiddenmessage, totalrevealers))
			return
		}

		// send the satoshis.
		// if it's a crowdfunding we'll send from everybody at the same time,
		// otherwise just from the current revealer.
		if hiddenmessage.Crowdfund <= 1 {
			revealerIds = []int{u.Id}
		}

		_, err = settleReveal(ctx, hiddenmessage.Satoshis, hiddenid,
			sourceUserId, revealerIds)
		if err != nil {
			log.Warn().Err(err).Str("id", hiddenid).Int("satoshis", hiddenmessage.Satoshis).
				Str("revealer", revealer.Username).Msg("failed to pay to reveal")
			send(ctx, revealer, WITHALERT, t.ERROR, t.T{"Err": err.Error()})
			return
		}

		// actually reveal
		if message := ctx.Value("message"); message != nil {
			// called in the bot's chat
			removeKeyboardButtons(ctx)
			send(ctx, revealer, hiddenmessage.Content, message)
		} else {
			if hiddenmessage.Public {
				// reveal message in-place
				send(ctx, hiddenmessage.revealed(), EDIT)
			} else {
				// reveal message privately
				send(ctx, revealer, hiddenmessage.revealed())
				if hiddenmessage.Times == 0 || hiddenmessage.Times > totalrevealers {
					// more people can still pay for this
					// buttons are kept so others still can pay, but updated
					send(ctx, EDIT, hiddenmessage.Preview,
						revealKeyboard(ctx, hiddenkey, hiddenmessage, totalrevealers))
				} else {
					// end of quota. no more people can reveal.
					send(ctx, EDIT, "A hidden message prompt once lived here.")
					removeKeyboardButtons(ctx)
				}
			}
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
			send(ctx, t.ERROR, APPEND)
			return
		}
		send(ctx, t.CHECKING, APPEND)

		go u.track("check pending", nil)

		go func(u User, messageId int, hash string) {
			pays, err := ln.CallNamed("listpays", "payment_hash", hash)
			if err != nil {
				send(ctx, t.ERROR, t.T{"Err": err.Error()}, APPEND)
				return
			}

			payment := pays.Get("pays.0")
			if !payment.Exists() || payment.Get("status").String() == "failed" {
				// payment failed
				log.Debug().
					Err(err).
					Str("hash", hash).
					Str("pay", payment.String()).
					Msg("canceling failed payment")
				paymentHasFailed(ctx, u, hash)
				return
			} else if payment.Get("status").String() == "pending" {
				// command timed out, should try again later
				send(ctx, t.TXPENDING, APPEND)
			} else {
				// payment succeeded
				paymentHasSucceeded(
					ctx,
					u,
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
		answer := handleExternalAppCallback(ctx)
		bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, answer))
	}

answerEmpty:
	send(ctx, "")
}
