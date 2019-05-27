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
		// no fee policy
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

	ln.Call("delinvoice", label, "unpaid") // we don't care if it doesn't exist

	res, err := ln.Call("invoice", map[string]interface{}{
		"msatoshi":    sats * 1000,
		"label":       label,
		"description": fmt.Sprintf("For %s to join %s (%d).", username, chat.Title, chat.ID),
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
	}

	pendingApproval[label] = kickdata
	waitToKick(label, kickdata)
}

func waitToKick(label string, kickdata KickData) {
	invpaid, err := ln.CallWithCustomTimeout(time.Minute*60, "waitinvoice", label)
	if err == nil && invpaid.Get("status").String() == "paid" {
		// the user did pay. allow.
		delete(pendingApproval, label)
		rds.HDel("ticket-pending", label)
		replaceMessage(&kickdata.InvoiceMessage, "Invoice paid.")
		return
	} else if err != nil {
		if cmderr, ok := err.(lightning.ErrorCommand); ok {
			if cmderr.Code == -1 {
				log.Info().Str("label", label).Msg("invoice deleted, ignore as user may be trying to join again")
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

func interceptMessage(message *tgbotapi.Message) (proceed bool) {
	label := fmt.Sprintf("newmember:%d:%d", message.From.ID, message.Chat.ID)
	if _, isPending := pendingApproval[label]; isPending {
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
