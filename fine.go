package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/lucsky/cuid"
)

func handleFine(ctx context.Context, opts docopt.Opts) {
	chatOwner := ctx.Value("initiator").(User)

	switch message := ctx.Value("message").(type) {
	case *tgbotapi.Message:
		if message.Chat.Type == "private" {
			send(ctx, chatOwner, t.MUSTBEGROUP)
			return
		}
		msats, err := parseSatoshis(opts)
		if err != nil || msats == 0 {
			send(ctx, chatOwner, t.ERROR, t.T{"Err": err.Error()})
			return
		}
		reason := getVariadicFieldOrReplyToContent(ctx, opts, "<reason>")
		if message.ReplyToMessage == nil {
			send(ctx, chatOwner, t.MISSINGRECEIVER)
			return
		}
		if message.ReplyToMessage.From.ID == message.From.ID {
			// can't fine yourself
			return
		}
		if !isAdmin(message.Chat, message.From) {
			send(ctx, chatOwner, t.MUSTBEADMIN)
			return
		}

		target, cas, err := ensureTelegramUser(message.ReplyToMessage)
		if err != nil {
			send(ctx, chatOwner, t.ERROR, t.T{"Err": err.Error()})
			log.Warn().Err(err).Int("case", cas).
				Str("username", message.ReplyToMessage.From.UserName).
				Int("id", message.ReplyToMessage.From.ID).
				Msg("failed to ensure user on reply-fine")
			return
		}

		fineKey := cuid.Slug()

		// for registered users we will send a keyboard
		// for unregistered users an invoice
		var (
			keyboard *tgbotapi.InlineKeyboardMarkup
			bolt11   string
			hash     string
		)
		if info, err := target.getInfo(); err == nil && info.BalanceMsat < msats {
			expiry := 15 * time.Minute
			bolt11, hash, err = chatOwner.makeInvoice(ctx, &MakeInvoiceArgs{
				IgnoreInvoiceSizeLimit: true,
				Msatoshi:               msats,
				Description: fmt.Sprintf(
					"fine for %s (%s).",
					target.AtName(ctx), reason,
				),
				Tag:    "fine",
				Extra:  InvoiceExtra{Message: message},
				Expiry: &expiry,
			})
			if err != nil {
				log.Warn().Err(err).
					Stringer("chat-owner", &chatOwner).Stringer("target", &target).
					Msg("failed to create a fine invoice.")
				return
			}

			go chatOwner.track("fine issued", map[string]interface{}{
				"sats":    msats / 1000,
				"group":   message.Chat.ID,
				"invoice": true,
			})
		} else {
			keyboard = &tgbotapi.InlineKeyboardMarkup{
				[][]tgbotapi.InlineKeyboardButton{
					{
						tgbotapi.NewInlineKeyboardButtonData(
							fmt.Sprintf(translateTemplate(ctx,
								t.PAYAMOUNT, t.T{"Sats": float64(msats) / 1000})),
							fmt.Sprintf("fine=%s", fineKey),
						),
					},
				},
			}

			go chatOwner.track("fine issued", map[string]interface{}{
				"sats":    msats / 1000,
				"group":   message.Chat.ID,
				"balance": true,
			})
		}

		notifyMessageId := send(ctx, message.Chat.ID, keyboard, t.FINEMESSAGE, t.T{
			"FinedUser": target.AtName(ctx),
			"Sats":      msats / 1000,
			"Reason":    reason,
		}, FORCESPAMMY)

		var invoiceMessage *tgbotapi.Message
		if bolt11 != "" {
			invoiceMessageId := send(ctx, message.Chat.ID,
				qrURL(bolt11), "<code>"+bolt11+"</code>", FORCESPAMMY)
			if invoiceMessageId == nil {
				log.Error().Int64("group", message.Chat.ID).
					Msg("failed to send invoice message on fine")
				send(ctx, message.Chat.ID,
					t.ERROR, t.T{"Err": "Error sending Telegram message, please report."}, FORCESPAMMY)
				return
			}

			invoiceMessage = &tgbotapi.Message{
				Chat:      &tgbotapi.Chat{ID: message.Chat.ID},
				MessageID: invoiceMessageId.(int),
			}
		}

		// reuse the kick stuff we have for tickets
		kickdata := KickData{
			"fine",
			invoiceMessage,
			&tgbotapi.Message{
				Chat:      &tgbotapi.Chat{ID: message.Chat.ID},
				MessageID: notifyMessageId.(int),
			},
			nil,
			tgbotapi.ChatMemberConfig{
				UserID: int(target.TelegramId),
				ChatID: message.Chat.ID,
			},
			chatOwner,
			target.Id,
			target.AtName(ctx),
			hash,
			int(msats / 1000),
		}
		kickdatajson, _ := json.Marshal(kickdata)
		err = rds.HSet("ticket-pending", fineKey, string(kickdatajson)).Err()
		if err != nil {
			log.Warn().Err(err).Str("kickdata", string(kickdatajson)).
				Msg("error saving kickdata")
		}
		go waitToKick(ctx, fineKey, kickdata)
	}
}

func handleFineClickPay(ctx context.Context, fineKey string) {
	payer := ctx.Value("initiator").(User)

	log := log.With().Str("fine-key", fineKey).Logger()

	kickdatastr, err := rds.HGet("ticket-pending", fineKey).Result()
	if err != nil {
		log.Warn().Err(err).Msg("error getting fine pending on callback")
		return
	}

	var kickdata KickData
	if err := json.Unmarshal([]byte(kickdatastr), &kickdata); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal kickdata from redis")
		return
	}

	if payer.Id != kickdata.TargetId {
		return
	}

	err = payer.sendInternally(
		ctx,
		kickdata.ChatOwner,
		false,
		int64(kickdata.Sats*1000),
		0,
		fmt.Sprintf("Fine at %s.", telegramMessageLink(kickdata.NotifyMessage)),
		"",
		"fine",
	)
	if err != nil {
		send(ctx, payer, t.ERROR, t.T{"Err": err.Error()})
		return
	}

	dispatchGeneric(fineKey, nil)
	finePaid(ctx, fineKey, kickdata)
}

func fineNotPaid(ctx context.Context, fineKey string, kickdata KickData) {
	log.Info().Str("fine-key", fineKey).Interface("chat-member", kickdata.ChatMemberConfig).
		Msg("fine expired, kicking user")

	rds.HDel("ticket-pending", fineKey)

	// delete invoice message if it exists
	if kickdata.InvoiceMessage != nil {
		deleteMessage(kickdata.InvoiceMessage)
	}

	// remove keyboard if any
	send(ctx, kickdata.NotifyMessage, EDIT, &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{},
	})

	// send a new message notifying the group of the failure to pay
	send(ctx, kickdata.NotifyMessage.Chat.ID, kickdata.NotifyMessage, t.FINEFAILURE, t.T{
		"User": kickdata.TargetUsername,
	}, FORCESPAMMY)
}

func finePaid(ctx context.Context, fineKey string, kickdata KickData) {
	log.Debug().Str("fine-key", fineKey).Msg("fine paid")
	rds.HDel("ticket-pending", fineKey)

	// delete the invoice message
	if kickdata.InvoiceMessage != nil {
		deleteMessage(kickdata.InvoiceMessage)
	}

	// remove keyboard if any
	send(ctx, kickdata.NotifyMessage, EDIT, &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{},
	})

	// send a new message notifying the group of the success paying
	send(ctx, kickdata.NotifyMessage.Chat.ID, kickdata.NotifyMessage, t.FINESUCCESS, t.T{
		"User": kickdata.TargetUsername,
	}, FORCESPAMMY)

	go kickdata.ChatOwner.track("fine paid", map[string]interface{}{
		"sats":  kickdata.Sats,
		"group": kickdata.JoinMessage.Chat.ID,
	})
}
