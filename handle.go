package main

import (
	"strings"
	"time"

	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func handle(upd tgbotapi.Update) {
	switch {
	case upd.Message != nil:
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
			go deleteMessage(upd.Message)
		}
	case upd.ChannelPost != nil:
		handleMessage(upd.ChannelPost)
	case upd.CallbackQuery != nil:
		// is temporarily s.Banned?
		if _, ok := s.Banned[upd.CallbackQuery.From.ID]; ok {
			log.Debug().Int("tgid", upd.CallbackQuery.From.ID).Msg("got request from banned user")
			return
		}

		handleCallback(upd.CallbackQuery)
	case upd.InlineQuery != nil:
		go handleInlineQuery(upd.InlineQuery)
	case upd.EditedMessage != nil:
		handleEditedMessage(upd.EditedMessage)
	}
}

func handleInvoicePaid(payindex, msats int64, desc, hash, preimage, label string) {
	var receiver User
	var messageId int
	var tag string

	// could be a ticket invoice
	if strings.HasPrefix(label, "newmember:") {
		receiver, err = chatOwnerFromTicketLabel(label)
		if err != nil {
			return
		}
		messageId = 0
	} else {
		// extract user id from label
		var ok bool
		var userId int
		messageId, userId, tag, ok = parseLabel(label)

		if ok {
			// normal invoice
			u, err := loadUser(userId, 0)
			if err != nil {
				log.Warn().Err(err).
					Int("userid", userId).Str("label", label).
					Msg("failed to parse label for received payment or loading user")
				return
			}

			u.track("got payment", map[string]interface{}{
				"sats": float64(msats) / 1000,
			})

			receiver = u
		} else {
			// otherwise we don't know what is this
			log.Debug().Str("label", label).Int64("msat", msats).Msg("unrecognized payment received.")
			return
		}
	}

	// is there a comment associated with this?
	go func() {
		time.Sleep(3 * time.Second)
		comment := rds.Get("lnurlpay-comment:" + hash).Val()
		if comment != "" {
			receiver.notify(t.LNURLPAYCOMMENT, t.T{
				"Text":           comment,
				"HashFirstChars": hash[:5],
			})
		}
	}()

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
