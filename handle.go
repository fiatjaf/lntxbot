package main

import (
	"strings"

	"git.alhur.es/fiatjaf/lntxbot/t"
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
	} else if upd.EditedMessage != nil {
		handleEditedMessage(upd.EditedMessage)
	}
}

func invoicePaidListener(invpaid gjson.Result) {
	go handleInvoicePaid(
		invpaid.Get("pay_index").Int(),
		invpaid.Get("msatoshi_received").Int(),
		invpaid.Get("description").String(),
		invpaid.Get("payment_hash").String(),
		invpaid.Get("label").String(),
	)
	go func() {
		if chans, ok := waitingInvoices[invpaid.Get("payment_hash").String()]; ok {
			for _, ch := range chans {
				select {
				case ch <- invpaid:
				default:
				}
			}
		}
	}()
}

func handleInvoicePaid(payindex, msats int64, desc, hash, label string) {
	if payindex > 0 {
		rds.Set("lastinvoiceindex", payindex, 0)
	}

	// extract user id and preimage from label
	messageId, userId, preimage, tag, ok := parseLabel(label)
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
		tag,
	)
	if err != nil {
		receiver.notifyAsReply(t.FAILEDTOSAVERECEIVED, t.T{
			"Label": label,
			"Hash":  hash,
		}, messageId)
		return
	}

	receiver.notifyAsReply(t.PAYMENTRECEIVED, t.T{
		"Sats": msats / 1000,
		"Hash": hash[:5],
	}, messageId)
}
