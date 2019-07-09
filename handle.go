package main

import (
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/tidwall/gjson"
)

func handle(upd tgbotapi.Update, bundle *i18n.Bundle) {
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
			handleMessage(upd.Message, bundle)
		} else {
			deleteMessage(upd.Message)
		}
	} else if upd.CallbackQuery != nil {
		handleCallback(upd.CallbackQuery)
	} else if upd.InlineQuery != nil {
		handleInlineQuery(upd.InlineQuery)
	} else if upd.EditedMessage != nil {
		handleEditedMessage(upd.EditedMessage, bundle)
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
			receiver, err = chatOwnerFromTicketLabel(label)
			if err != nil {
				return
			}
			messageId = 0
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
	//TODO: connect locale to user property
	locale := "en"
	if err != nil {
		msgTempl := map[string]interface{}{
			"Label": label,
			"Hash": hash,
		}
		msgStr, _ := translateTemplate("FailedToSavePayReq", locale, msgTempl)
		receiver.notify(
			msgStr,
		)
		return
	}
	msgTempl := map[string]interface{}{
		"Sats": msats/1000,
		"Hash": hash[:5],
	}
	msgStr, _ := translateTemplate("PaymentRecieved", locale, msgTempl)
	receiver.notify(
		msgStr,
	)
	receiver.notifyAsReply(msgStr, messageId)
}
