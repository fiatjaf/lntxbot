package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/fiatjaf/lightningd-gjson-rpc"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

var pendingApproval = make(map[string]KickData)

func handleNewMember(chat *tgbotapi.Chat, newmember tgbotapi.User) {
	sats, err := getTicketPrice(chat.ID)
	if err != nil {
		log.Error().Err(err).Str("chat", chat.Title).Msg("error fetching ticket price for chat")
		return
	}

	if sats == 0 {
		// no ticket policy
		return
	}

	var username string
	if newmember.UserName != "" {
		username = "@" + newmember.UserName
	} else {
		username = newmember.FirstName
	}

	notifyMessage := notify(chat.ID, fmt.Sprintf(
		"Hello, %s. You have 30min to pay the following invoice for %d sat if you want to stay in this group:",
		username, sats))

	label := fmt.Sprintf("newmember:%d:%d", newmember.ID, chat.ID)

	ln.Call("delinvoice", label, "unpaid")  // we don't care if it doesn't exist
	ln.Call("delinvoice", label, "paid")    // we don't care if it doesn't exist
	ln.Call("delinvoice", label, "expired") // we don't care if it doesn't exist

	res, err := ln.Call("invoice", map[string]interface{}{
		"msatoshi":    sats * 1000,
		"label":       label,
		"description": fmt.Sprintf("Ticket for %s to join %s (%d).", username, chat.Title, chat.ID),
		"expiry":      int(time.Minute * 30 / time.Second),
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to make invoice on new member")
		return
	}

	bolt11 := res.Get("bolt11").String()
	qrpath := generateQR(label, bolt11)

	invoiceMessage := notifyWithPicture(chat.ID, qrpath, bolt11)

	kickdata := KickData{
		invoiceMessage,
		notifyMessage,
		tgbotapi.ChatMemberConfig{
			UserID: newmember.ID,
			ChatID: chat.ID,
		},
		res.Get("payment_hash").String(),
	}

	kickdatajson, _ := json.Marshal(kickdata)
	err = rds.HSet("ticket-pending", label, string(kickdatajson)).Err()
	if err != nil {
		log.Warn().Err(err).Str("kickdata", string(kickdatajson)).Msg("error saving kickdata")
	}
	pendingApproval[label] = kickdata
	go waitToKick(label, kickdata)
}

func waitToKick(label string, kickdata KickData) {
	log.Debug().Str("label", label).Msg("waiting to kick")
	invpaid, err := ln.CallWithCustomTimeout(time.Minute*60, "waitinvoice", label)
	if err == nil && invpaid.Get("status").String() == "paid" {
		// the user did pay. allow.
		ticketPaid(label, kickdata)
		return
	} else if err != nil {
		if cmderr, ok := err.(lightning.ErrorCommand); ok {
			if cmderr.Code == -1 {
				log.Info().Str("label", label).
					Msg("invoice deleted, assume it was paid internally")
				delete(pendingApproval, label)
				rds.HDel("ticket-pending", label)
				return
			} else if cmderr.Code == -2 {
				// didn't pay. kick.
				log.Info().Str("label", label).Msg("invoice expired, kicking user")

				banuntil := time.Now()
				banuntil.AddDate(0, 0, 1)

				bot.KickChatMember(tgbotapi.KickChatMemberConfig{
					kickdata.ChatMemberConfig,
					banuntil.Unix(),
				})

				delete(pendingApproval, label)
				rds.HDel("ticket-pending", label)

				// delete messages
				deleteMessage(&kickdata.NotifyMessage)
				deleteMessage(&kickdata.InvoiceMessage)
				return
			}
		}
		log.Warn().Err(err).Msg("unexpected error while waiting to kick")
	} else {
		// should never happen
		log.Error().Str("invoice", invpaid.String()).
			Msg("got a waitinvoice response for an invoice that wasn't paid")
	}
}

func ticketPaid(label string, kickdata KickData) {
	log.Debug().Str("label", label).Msg("ticket paid")
	delete(pendingApproval, label)
	rds.HDel("ticket-pending", label)

	// replace caption
	_, err := bot.Send(tgbotapi.NewEditMessageText(
		kickdata.NotifyMessage.Chat.ID,
		kickdata.NotifyMessage.MessageID,
		"Invoice paid. User allowed.",
	))
	if err != nil {
		log.Warn().Err(err).Msg("failed to replace invoice with 'paid' message.")
	}

	// delete the other message
	deleteMessage(&kickdata.InvoiceMessage)
}

func interceptMessage(message *tgbotapi.Message) (proceed bool) {
	label := fmt.Sprintf("newmember:%d:%d", message.From.ID, message.Chat.ID)
	if _, isPending := pendingApproval[label]; isPending {
		log.Debug().Str("user", message.From.String()).Msg("user pending, can't speak")
		return false
	}
	return true
}

func startKicking() {
	data, err := rds.HGetAll("ticket-pending").Result()
	if err != nil {
		log.Warn().Err(err).Msg("error getting tickets pending")
		return
	}

	for label, kickdatastr := range data {
		var kickdata KickData
		err := json.Unmarshal([]byte(kickdatastr), &kickdata)
		if err != nil {
			log.Warn().Err(err).Msg("failed to unmarshal kickdata from redis")
			continue
		}

		pendingApproval[label] = kickdata
		go waitToKick(label, kickdata)
	}
}
