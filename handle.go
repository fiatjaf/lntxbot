package main

import (
	"fmt"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/tidwall/gjson"
)

func handle(upd tgbotapi.Update) {
	if upd.Message != nil {
		if upd.Message.NewChatMembers != nil {
			for _, newmember := range *upd.Message.NewChatMembers {
				handleNewMember(upd.Message.Chat, newmember)
			}
		} else {
			proceed := interceptMessage(upd.Message)
			if proceed {
				handleMessage(upd.Message)
			} else {
				deleteMessage(upd.Message)
			}
		}
	} else if upd.CallbackQuery != nil {
		handleCallback(upd.CallbackQuery)
	} else if upd.InlineQuery != nil {
		handleInlineQuery(upd.InlineQuery)
	} else if upd.EditedMessage != nil {
	}
}

func handleInvoicePaid(res gjson.Result) {
	index := res.Get("pay_index").Int()
	rds.Set("lastinvoiceindex", index, 0)

	msats := res.Get("msatoshi_received").Int()
	desc := res.Get("description").String()
	hash := res.Get("payment_hash").String()
	label := res.Get("label").String()

	// extract user id and preimage from label
	messageId, userId, preimage, ok := parseLabel(label)
	if ok {
		// normal invoice
		u, err := loadUser(userId, 0)
		if err != nil {
			log.Warn().Err(err).
				Int("userid", userId).Str("label", label).Int64("index", index).
				Msg("failed to parse label for received payment or loading user")
			return
		}

		err = u.paymentReceived(
			int(msats),
			desc,
			hash,
			preimage,
			label,
		)
		if err != nil {
			u.notify(
				"Payment received, but failed to save on database. Please report this issue: <code>" + label + "</code>, hash: <code>" + hash + "</code>",
			)
		}

		u.notifyAsReply(fmt.Sprintf("Payment received: %d. /tx%s.", msats/1000, hash[:5]), messageId)
	} else {
		// could be a ticket invoice
		if _, ok := pendingApproval[label]; ok {
			// but we won't handle it here
		} else {
			log.Debug().Str("label", label).Int64("msat", msats).Msg("unrecognized payment received.")
		}
	}
}
