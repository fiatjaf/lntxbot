package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	cmap "github.com/orcaman/concurrent-map"
)

var pendingApproval = cmap.New()

type KickData struct {
	InvoiceMessage   tgbotapi.Message          `json:"invoice_message"`
	NotifyMessage    tgbotapi.Message          `json:"notify_message"`
	JoinMessage      tgbotapi.Message          `json:"join_message"`
	ChatMemberConfig tgbotapi.ChatMemberConfig `json:"chat_member_config"`
	NewMember        tgbotapi.User             `json:"new_member"`
	Hash             string                    `json:"hash"`
	Sats             int                       `json:"sats"`
}

func handleNewMember(joinMessage *tgbotapi.Message, newmember tgbotapi.User) {
	g, err := loadGroup(joinMessage.Chat.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			// fine, this group has no settings
			return
		}

		log.Error().Err(err).Str("chat", joinMessage.Chat.Title).Msg("error fetching group chat on new member")
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

	notifyMessage := g.notify(t.SPAMFILTERMESSAGE, t.T{
		"User": username,
		"Sats": g.Ticket,
	})

	chatOwner, err := getChatOwner(joinMessage.Chat.ID)
	if err != nil {
		log.Warn().Err(err).Msg("chat has no owner, can't create ticket. allowing user.")
		return
	}

	chatOwner.track("ticket shown", map[string]interface{}{
		"group": joinMessage.Chat.ID,
		"sats":  g.Ticket,
	})

	expiration := time.Minute * 15

	bolt11, hash, qrpath, err := chatOwner.makeInvoice(makeInvoiceArgs{
		IgnoreInvoiceSizeLimit: true,
		Msatoshi:               int64(g.Ticket) * 1000,
		Desc: fmt.Sprintf(
			"ticket for %s to join %s (%d).",
			username, joinMessage.Chat.Title, joinMessage.Chat.ID,
		),
		Tag: "ticket",
		Extra: map[string]interface{}{
			"member": newmember.ID,
			"chat":   joinMessage.Chat.ID,
		},
		Expiry: &expiration,
	})
	if err != nil {
		log.Warn().Err(err).
			Str("chat", joinMessage.Chat.Title).
			Str("username", username).
			Msg("failed to create a ticket invoice. allowing user.")
		return
	}

	invoiceMessage := sendTelegramMessageWithPicture(joinMessage.Chat.ID, qrpath, bolt11)

	kickdata := KickData{
		invoiceMessage,
		notifyMessage,
		*joinMessage,
		tgbotapi.ChatMemberConfig{
			UserID: newmember.ID,
			ChatID: joinMessage.Chat.ID,
		},
		newmember,
		hash,
		g.Ticket,
	}

	kickdatajson, _ := json.Marshal(kickdata)
	err = rds.HSet("ticket-pending", joinKey, string(kickdatajson)).Err()
	if err != nil {
		log.Warn().Err(err).Str("kickdata", string(kickdatajson)).Msg("error saving kickdata")
	}
	pendingApproval.Set(joinKey, kickdata)
	go waitToKick(joinKey, kickdata)
}

func waitToKick(joinKey string, kickdata KickData) {
	log.Debug().Str("join-key", joinKey).Msg("waiting to kick")
	select {
	case <-waitInvoice(kickdata.Hash):
		// invoice was paid, accept user in group.
		ticketPaid(joinKey, kickdata)
	case <-time.After(15 * time.Minute):
		// didn't pay. kick
		log.Info().Str("join-key", joinKey).Msg("invoice expired, kicking user")

		bot.KickChatMember(tgbotapi.KickChatMemberConfig{
			ChatMemberConfig: kickdata.ChatMemberConfig,
			UntilDate:        time.Now().AddDate(0, 0, 1).Unix(),
		})

		pendingApproval.Remove(joinKey)
		rds.HDel("ticket-pending", joinKey)

		// delete messages
		deleteMessage(&kickdata.JoinMessage)
		deleteMessage(&kickdata.NotifyMessage)
		deleteMessage(&kickdata.InvoiceMessage)
	}
}

func ticketPaid(joinKey string, kickdata KickData) {
	g, err := loadGroup(kickdata.JoinMessage.Chat.ID)
	if err != nil {
		log.Error().Err(err).Str("chat", kickdata.JoinMessage.Chat.Title).
			Msg("error fetching group chat after ticked paid")
		return
	}

	log.Debug().Str("join-key", joinKey).Msg("ticket paid")
	pendingApproval.Remove(joinKey)
	rds.HDel("ticket-pending", joinKey)

	// delete the invoice message
	deleteMessage(&kickdata.InvoiceMessage)

	user, _, _ := ensureTelegramUser(
		kickdata.NewMember.ID,
		kickdata.NewMember.UserName,
		kickdata.NewMember.LanguageCode,
	)

	// replace caption
	_, err = bot.Send(tgbotapi.NewEditMessageText(
		kickdata.NotifyMessage.Chat.ID,
		kickdata.NotifyMessage.MessageID,
		translateTemplate(t.USERALLOWED, g.Locale, t.T{"User": user.AtName()}),
	))
	if err != nil {
		log.Warn().Err(err).Msg("failed to replace invoice with 'paid' message.")
	}

	go user.track("user allowed", map[string]interface{}{
		"sats":  kickdata.Sats,
		"group": kickdata.JoinMessage.Chat.ID,
	})
}

func startKicking() {
	data, err := rds.HGetAll("ticket-pending").Result()
	if err != nil {
		log.Warn().Err(err).Msg("error getting tickets pending")
		return
	}

	for joinKey, kickdatastr := range data {
		var kickdata KickData
		err := json.Unmarshal([]byte(kickdatastr), &kickdata)
		if err != nil {
			log.Warn().Err(err).Msg("failed to unmarshal kickdata from redis")
			continue
		}

		log.Debug().Msg("restarted kick invoice wait")
		pendingApproval.Set(joinKey, kickdata)
		go waitToKick(joinKey, kickdata)
	}
}

func interceptMessage(message *tgbotapi.Message) (proceed bool) {
	joinKey := fmt.Sprintf("%d:%d", message.From.ID, message.Chat.ID)
	if _, isPending := pendingApproval.Get(joinKey); isPending {
		log.Debug().Str("user", message.From.String()).Msg("user pending, can't speak")
		return false
	}
	return true
}
