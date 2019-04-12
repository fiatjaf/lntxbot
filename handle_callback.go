package main

import (
	"fmt"
	"strconv"
	"strings"

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
			swinnerId, err := rds.SRandMember(rkey).Result()
			if err != nil {
				log.Warn().Err(err).Str("coinflip", params[2]).
					Msg("error getting random participant from redis.")
				removeKeyboardButtons(cb)
				appendTextToMessage(cb, "\n\nCoinflip error.")
				goto answerEmpty
			}

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
			sparticipants, err := rds.SMembers(rkey).Result()
			if err != nil {
				log.Warn().Err(err).Msg("failed to get coinflip participants")
				removeKeyboardButtons(cb)
				appendTextToMessage(cb, "\n\nCoinflip error.")
				goto answerEmpty
			}
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

			go rds.Del(rkey)

			winner, err := processCoinflip(sats, winnerId, participants, 0)
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
