package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/tidwall/gjson"
)

func handle(upd tgbotapi.Update) {
	if upd.Message != nil {
		// people joining
		if upd.Message.NewChatMembers != nil {
			for _, newmember := range *upd.Message.NewChatMembers {
				handleNewMember(upd.Message, newmember)
			}
		}

		// normal message
		proceed := interceptMessage(upd.Message)
		if proceed {
			handleMessage(upd.Message)
		} else {
			deleteMessage(upd.Message)
		}
	} else if upd.CallbackQuery != nil {
		handleCallback(upd.CallbackQuery)
	} else if upd.InlineQuery != nil {
		handleInlineQuery(upd.InlineQuery)
	}
}

func invoicePaidListener(invpaid gjson.Result) {
	handleInvoicePaid(
		invpaid.Get("pay_index").Int(),
		invpaid.Get("msatoshi_received").Int(),
		invpaid.Get("description").String(),
		invpaid.Get("payment_hash").String(),
		invpaid.Get("label").String(),
	)
}

func handleInvoicePaid(payindex, msats int64, desc, hash, label string) {
	if payindex > 0 {
		rds.Set("lastinvoiceindex", payindex, 0)
	}

	// extract user id and preimage from label
	messageId, userId, preimage, ok := parseLabel(label)
	var receiver User

	if ok {
		// normal invoice
		u, err := loadUser(userId, 0)
		if err != nil {
			log.Warn().Err(err).
				Int("userid", userId).Str("label", label).
				Msg("failed to parse label for received payment or loading user")
			return
		}
		receiver = u
	} else {
		// could be a ticket invoice
		if strings.HasPrefix(label, "newmember:") {
			parts := strings.Split(label, ":")
			chatId, err := strconv.Atoi(parts[2])
			if err != nil {
				log.Error().Err(err).Str("label", label).Msg("failed to parse ticket invoice")
				return
			}

			messageId = 0

			receiver, err = getChatOwner(int64(chatId))
			if err != nil {
				log.Error().Err(err).Str("label", label).Msg("failed to get chat owner in ticket invoice handling")
				return
			}

			preimage = ""
		} else {
			// otherwise we don't know what is this
			log.Debug().Str("label", label).Int64("msat", msats).Msg("unrecognized payment received.")
			return
		}
	}

	// proceed to compute an incoming payment for this user
	err = receiver.paymentReceived(
		int(msats),
		desc,
		hash,
		preimage,
		label,
	)
	if err != nil {
		receiver.notify(
			"Payment received, but failed to save on database. Please report this issue: <code>" + label + "</code>, hash: <code>" + hash + "</code>",
		)
		return
	}

	receiver.notifyAsReply(fmt.Sprintf("Payment received: %d. /tx%s.", msats/1000, hash[:5]), messageId)
}
