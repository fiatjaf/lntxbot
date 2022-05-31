package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	cmap "github.com/orcaman/concurrent-map"
)

var pendingApproval = cmap.New()

func handleTelegramNewMember(
	ctx context.Context,
	joinMessage *tgbotapi.Message,
	newmember tgbotapi.User,
) {
	g, err := loadTelegramGroup(joinMessage.Chat.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			// fine, this group has no settings
			return
		}

		log.Error().Err(err).Str("chat", joinMessage.Chat.Title).
			Msg("error fetching group chat on new member")
		return
	}

	if g.Ticket == 0 {
		// no ticket policy
		return
	}

	joinKey := fmt.Sprintf("%d:%d", newmember.ID, joinMessage.Chat.ID)
	if _, isPending := pendingApproval.Get(joinKey); isPending {
		// user joined, left and joined again.
		// do nothing as the old timer is still counting.
		return
	}

	var username string
	if newmember.UserName != "" {
		username = "@" + newmember.UserName
	} else {
		username = newmember.FirstName
	}

	chatOwner, err := getChatOwner(joinMessage.Chat.ID)
	if err != nil {
		log.Warn().Err(err).Msg("chat has no owner, can't create ticket. allowing user.")
		return
	}

	expiration := time.Minute * 15

	// for registered users we will send a keyboard
	// for unregistered users an invoice
	var (
		keyboard *tgbotapi.InlineKeyboardMarkup
		bolt11   string
		hash     string
	)

	target, _ := loadTelegramUser(newmember.ID)

	if info, err := target.getInfo(); err == nil &&
		info.BalanceMsat < int64(g.Ticket*1000) {

		bolt11, hash, err = chatOwner.makeInvoice(ctx, &MakeInvoiceArgs{
			IgnoreRateLimit: true,
			Msatoshi:        int64(g.Ticket) * 1000,
			Description: fmt.Sprintf(
				"ticket for %s to join %s (%d).",
				username, joinMessage.Chat.Title, joinMessage.Chat.ID,
			),
			Tag:    "ticket",
			Extra:  InvoiceExtra{Message: joinMessage},
			Expiry: &expiration,
		})
		if err != nil {
			log.Warn().Err(err).
				Str("chat", joinMessage.Chat.Title).
				Str("username", username).
				Msg("failed to create a ticket invoice. allowing user.")
			return
		}

		go chatOwner.track("ticket shown", map[string]interface{}{
			"sats":    g.Ticket,
			"group":   joinMessage.Chat.ID,
			"invoice": true,
		})
	} else {
		keyboard = &tgbotapi.InlineKeyboardMarkup{
			[][]tgbotapi.InlineKeyboardButton{
				{
					tgbotapi.NewInlineKeyboardButtonData(
						fmt.Sprintf(translateTemplate(ctx,
							t.PAYAMOUNT, t.T{"Sats": float64(g.Ticket)})),
						fmt.Sprintf("ticket=%s", joinKey),
					),
				},
			},
		}

		go chatOwner.track("ticket shown", map[string]interface{}{
			"sats":    g.Ticket,
			"group":   joinMessage.Chat.ID,
			"balance": true,
		})
	}

	notifyMessageId := send(ctx, g, t.TICKETMESSAGE, t.T{
		"User": username,
		"Sats": g.Ticket,
	}, keyboard)

	var invoiceMessage *tgbotapi.Message
	if bolt11 != "" {
		invoiceMessageId := send(ctx, g, qrURL(bolt11), "<code>"+bolt11+"</code>")
		if invoiceMessageId == nil {
			log.Error().Stringer("group", &g).
				Msg("failed to send invoice message on new member")
			send(ctx, g, t.ERROR, t.T{"Err": "Telegram has refused a message from us."})
			return
		}

		invoiceMessage = &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: joinMessage.Chat.ID},
			MessageID: invoiceMessageId.(int),
		}
	}

	kickdata := KickData{
		"ticket",
		invoiceMessage,
		&tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: joinMessage.Chat.ID},
			MessageID: notifyMessageId.(int),
		},
		joinMessage,
		tgbotapi.ChatMemberConfig{
			UserID: newmember.ID,
			ChatID: joinMessage.Chat.ID,
		},
		chatOwner,
		0,
		username,
		hash,
		g.Ticket,
	}

	kickdatajson, _ := json.Marshal(kickdata)
	err = rds.HSet("ticket-pending", joinKey, string(kickdatajson)).Err()
	if err != nil {
		log.Warn().Err(err).Str("kickdata", string(kickdatajson)).
			Msg("error saving kickdata")
	}
	pendingApproval.Set(joinKey, kickdata)
	go waitToKick(ctx, joinKey, kickdata)
}

func handleTicketClickPay(ctx context.Context, joinKey string) {
	payer := ctx.Value("initiator").(User)

	log := log.With().Str("ticket-key", joinKey).Logger()

	kickdatastr, err := rds.HGet("ticket-pending", joinKey).Result()
	if err != nil {
		log.Warn().Err(err).Msg("error getting ticket pending on callback")
		return
	}

	var kickdata KickData
	if err := json.Unmarshal([]byte(kickdatastr), &kickdata); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal kickdata from redis")
		return
	}

	// anyone can pay, but if the payer is the group owner we don't do a transaction
	if payer.Id == kickdata.ChatOwner.Id {
		dispatchGeneric(joinKey, nil)
		ticketPaid(ctx, joinKey, kickdata)
		return
	}

	err = payer.sendInternally(
		ctx,
		kickdata.ChatOwner,
		false,
		int64(kickdata.Sats*1000),
		0,
		fmt.Sprintf("Ticket for group entrance at %s.",
			telegramMessageLink(kickdata.NotifyMessage)),
		"",
		"ticket",
	)
	if err != nil {
		send(ctx, payer, t.ERROR, t.T{"Err": err.Error()})
		return
	}

	dispatchGeneric(joinKey, nil)
	ticketPaid(ctx, joinKey, kickdata)
}

func ticketPaid(ctx context.Context, joinKey string, kickdata KickData) {
	// invoice was paid, accept user in group.

	g, err := loadTelegramGroup(kickdata.JoinMessage.Chat.ID)
	if err != nil {
		log.Error().Err(err).Str("chat", kickdata.JoinMessage.Chat.Title).
			Msg("error fetching group chat after ticked paid")
		return
	}

	log.Debug().Str("join-key", joinKey).Msg("ticket paid")
	pendingApproval.Remove(joinKey)
	rds.HDel("ticket-pending", joinKey)

	// delete the invoice message
	if kickdata.InvoiceMessage != nil {
		deleteMessage(kickdata.InvoiceMessage)
	}

	// replace caption
	send(ctx, EDIT, g, kickdata.NotifyMessage, t.TICKETUSERALLOWED,
		t.T{"User": kickdata.TargetUsername}, &tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{},
		})

	go kickdata.ChatOwner.track("user allowed", map[string]interface{}{
		"sats":  kickdata.Sats,
		"group": kickdata.JoinMessage.Chat.ID,
	})
}

func ticketNotPaid(ctx context.Context, joinKey string, kickdata KickData) {
	// didn't pay. kick
	log.Info().Str("join-key", joinKey).Msg("ticket expired, kicking user")

	bot.KickChatMember(tgbotapi.KickChatMemberConfig{
		ChatMemberConfig: kickdata.ChatMemberConfig,
		UntilDate:        time.Now().AddDate(0, 0, 1).Unix(),
	})

	pendingApproval.Remove(joinKey)
	rds.HDel("ticket-pending", joinKey)

	// delete messages
	if kickdata.JoinMessage != nil {
		deleteMessage(kickdata.JoinMessage)
	}
	if kickdata.InvoiceMessage != nil {
		deleteMessage(kickdata.InvoiceMessage)
	}
	if kickdata.NotifyMessage != nil {
		deleteMessage(kickdata.NotifyMessage)
	}
}
