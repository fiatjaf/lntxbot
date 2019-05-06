package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	lightning "github.com/fiatjaf/lightningd-gjson-rpc"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

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

		hashfirstchars := cb.Data[4:]
		bolt11, err := rds.Get("payinvoice:" + hashfirstchars).Result()
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

		optmsats, _ := rds.Get("payinvoice:" + hashfirstchars + ":msats").Int64()
		err = u.payInvoice(messageId, bolt11, int(optmsats))
		if err == nil {
			appendTextToMessage(cb, "Attempting payment.")
		} else {
			appendTextToMessage(cb, err.Error())
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

		errMsg, err := u.sendInternally(messageId, claimer, false, sats*1000, "giveaway", nil)
		if err != nil {
			log.Warn().Err(err).Msg("failed to give away")
			claimer.notify("Failed to claim giveaway: " + errMsg)
			goto answerEmpty
		}
		bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, "Payment sent."))
		removeKeyboardButtons(cb)
		claimer.notify(fmt.Sprintf("%s has sent you %d sat.", u.AtName(), sats))

		var howtoclaimmessage = ""
		if claimer.ChatId == 0 {
			howtoclaimmessage = " To manage your funds, start a conversation with @lntxbot."
		}

		appendTextToMessage(cb,
			fmt.Sprintf(
				"%d sat given from %s to %s.", sats, u.AtName(), claimer.AtName(),
			)+howtoclaimmessage,
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
			appendTextToMessage(cb, "\n\nCoinflip expired.")
			goto answerEmpty
		}

		nparticipants, err1 := strconv.Atoi(params[0])
		sats, err2 := strconv.Atoi(params[1])
		if err1 != nil || err2 != nil {
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, "\n\nCoinflip error.")
			goto answerEmpty
		}

		joiner, t, err := ensureUser(cb.From.ID, cb.From.UserName)
		if err != nil {
			log.Warn().Err(err).Int("case", t).
				Str("username", cb.From.UserName).Int("tgid", cb.From.ID).
				Msg("failed to ensure joiner user on coinflip.")
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, "\n\nCoinflip error.")
			goto answerEmpty
		}

		if !joiner.checkBalanceFor(sats, "coinflip") {
			goto answerEmpty
		}

		if isMember, err := rds.SIsMember(rkey, joiner.Id).Result(); err != nil || isMember {
			joiner.notify("You can't join a coinflip twice.")
			goto answerEmpty
		}

		if err := rds.SAdd("coinflip:"+coinflipid, joiner.Id).Err(); err != nil {
			log.Warn().Err(err).Str("coinflip", coinflipid).Msg("error adding participant to coinflip.")
			goto answerEmpty
		}

		if nregistered+1 < nparticipants {
			// append @user to the coinflip message (without removing the keyboard)
			baseEdit := getBaseEdit(cb)
			keyboard := coinflipKeyboard(coinflipid, nparticipants, sats)
			baseEdit.ReplyMarkup = &keyboard
			edit := tgbotapi.EditMessageTextConfig{BaseEdit: baseEdit}
			if cb.Message != nil {
				edit.Text = cb.Message.Text + " " + joiner.AtName()
			} else {
				edit.Text = fmt.Sprintf("Pay %d and get a change to win %d! %d out of %d spots left!",
					sats, sats*nparticipants, nparticipants-nregistered, nparticipants)
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
				appendTextToMessage(cb, "\n\nCoinflip error.")
				goto answerEmpty
			}
			swinnerId := sparticipants[rand.Intn(len(sparticipants))]

			// winner id
			winnerId, err := strconv.Atoi(swinnerId)
			if err != nil {
				log.Warn().Err(err).Str("winnnerId", swinnerId).Msg("winner id is not an int")
				removeKeyboardButtons(cb)
				appendTextToMessage(cb, "\n\nCoinflip error.")
				goto answerEmpty
			}

			// all participants
			participants := make([]int, nregistered+1)
			for i, spart := range sparticipants {
				part, err := strconv.Atoi(spart)
				if err != nil {
					log.Warn().Err(err).Str("part", spart).Msg("participant id is not an int")
					removeKeyboardButtons(cb)
					appendTextToMessage(cb, "\n\nCoinflip error.")
					goto answerEmpty
				}
				participants[i] = part
			}

			winner, err := fromManyToOne(sats, winnerId, participants, "coinflip",
				"You're the winner of a coinflip for a prize of %[1]d sat. The losers were: %[2]s",
				"You've lost %[1]d in a coinflip. The winner was %[2]s.")
			if err != nil {
				log.Warn().Err(err).Msg("error processing coinflip transactions")
				removeKeyboardButtons(cb)
				appendTextToMessage(cb, "\n\nCoinflip error.")
				goto answerEmpty
			}

			removeKeyboardButtons(cb)
			if cb.Message != nil {
				appendTextToMessage(cb, joiner.AtName()+"\nWinner: "+winner.AtName())
				notifyAsReply(
					cb.Message.Chat.ID,
					"Coinflip winner: "+winner.AtName(),
					cb.Message.MessageID,
				)
			} else {
				appendTextToMessage(cb, "Winner: "+winner.AtName())
			}
		}
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
			appendTextToMessage(cb, "\n\nFundraising expired.")
			goto answerEmpty
		}

		receiverId, err1 := strconv.Atoi(params[0])
		ngivers, err2 := strconv.Atoi(params[1])
		sats, err3 := strconv.Atoi(params[2])
		if err1 != nil || err2 != nil || err3 != nil {
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, "\n\nFundraising error.")
			goto answerEmpty
		}

		joiner, t, err := ensureUser(cb.From.ID, cb.From.UserName)
		if err != nil {
			log.Warn().Err(err).Int("case", t).
				Str("username", cb.From.UserName).Int("tgid", cb.From.ID).
				Msg("failed to ensure joiner user on fundraise.")
			removeKeyboardButtons(cb)
			appendTextToMessage(cb, "\n\nFundraising error.")
			goto answerEmpty
		}

		if !joiner.checkBalanceFor(sats, "fundraise") {
			goto answerEmpty
		}

		if isMember, err := rds.SIsMember(rkey, joiner.Id).Result(); err != nil || isMember {
			joiner.notify("You can't join a fundraise twice.")
			goto answerEmpty
		}

		if err := rds.SAdd("fundraise:"+fundraiseid, joiner.Id).Err(); err != nil {
			log.Warn().Err(err).Str("fundraise", fundraiseid).Msg("error adding giver to fundraise.")
			goto answerEmpty
		}

		if nregistered+1 < ngivers {
			// append @user to the fundraise message (without removing the keyboard)
			baseEdit := getBaseEdit(cb)
			keyboard := fundraiseKeyboard(fundraiseid, receiverId, ngivers, sats)
			baseEdit.ReplyMarkup = &keyboard
			edit := tgbotapi.EditMessageTextConfig{BaseEdit: baseEdit}
			if cb.Message != nil {
				edit.Text = cb.Message.Text + " " + joiner.AtName()
			} else {
				edit.Text = fmt.Sprintf("Pay %d and get a change to win %d! %d out of %d spots left!",
					sats, sats*ngivers, ngivers-nregistered, ngivers)
			}
			bot.Send(edit)
		} else {
			// commit the fundraise. this is the same as the coinflip, just without randomness.
			sgivers, err := rds.SMembers(rkey).Result()
			go rds.Del(rkey)
			if err != nil {
				log.Warn().Err(err).Msg("failed to get fundraise givers")
				removeKeyboardButtons(cb)
				appendTextToMessage(cb, "\n\nFundraising error.")
				goto answerEmpty
			}

			// all givers
			givers := make([]int, nregistered+1)
			for i, spart := range sgivers {
				part, err := strconv.Atoi(spart)
				if err != nil {
					log.Warn().Err(err).Str("part", spart).Msg("giver id is not an int")
					removeKeyboardButtons(cb)
					appendTextToMessage(cb, "\n\nFundraising error.")
					goto answerEmpty
				}
				givers[i] = part
			}

			receiver, err := fromManyToOne(sats, receiverId, givers, "fundraise",
				"You've received %[1]d sat of a fundraise from %[2]s",
				"You've given %[1]d in a fundraise to %[2]s.")
			if err != nil {
				log.Warn().Err(err).Msg("error processing fundraise transactions")
				removeKeyboardButtons(cb)
				appendTextToMessage(cb, "\n\nFundraise error.")
				goto answerEmpty
			}

			removeKeyboardButtons(cb)
			if cb.Message != nil {
				appendTextToMessage(cb, joiner.AtName()+"\nCompleted!")
				notifyAsReply(cb.Message.Chat.ID,
					"Fundraising for "+receiver.AtName()+" completed!", cb.Message.MessageID)
			} else {
				appendTextToMessage(cb, "Fundraising for "+receiver.AtName()+" completed!")
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
		go func(u User, messageId int, hash string) {
			res, err := ln.Call("waitsendpay", hash)
			if err != nil {
				// an error we know it's a final error
				if cmderr, ok := err.(lightning.ErrorCommand); ok {
					if cmderr.Code == 203 || cmderr.Code == 208 || cmderr.Code == 209 {
						log.Debug().
							Err(err).
							Str("hash", hash).
							Msg("canceling failed payment because it has failed failed")
						u.paymentHasFailed(messageId, hash)
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
						u.paymentHasFailed(messageId, hash)
					}
				}

				// unknown error, report
				log.Warn().Err(err).Str("hash", hash).Str("user", u.Username).
					Msg("unexpected error waiting payment resolution")
				appendTextToMessage(cb, "Unexpected error: please report.")
				return
			}

			// payment succeeded
			u.paymentHasSucceeded(messageId, res)
		}(u, txn.TriggerMessage, txn.Hash)

		appendTextToMessage(cb, "Checking.")
	}

answerEmpty:
	bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, ""))
}
