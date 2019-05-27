package main

import (
	"fmt"
	"time"

	"github.com/fiatjaf/lightningd-gjson-rpc"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

var pendingApproval = make(map[string]bool)

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

	notify(chat.ID, fmt.Sprintf(
		"Hello, %s. Please pay the following invoice for %d sat if you want to stay in this group:",
		username, sats))

	label := fmt.Sprintf("newmember:%d:%d", newmember.ID, chat.ID)

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

	notifyWithPicture(chat.ID, qrpath, bolt11)

	chatmemberconfig := tgbotapi.ChatMemberConfig{
		ChatID: chat.ID,
		UserID: newmember.ID,
	}

	pendingApproval[label] = true

	go func(label string, chatmemberconfig tgbotapi.ChatMemberConfig) {
		inv, err := ln.CallWithCustomTimeout(time.Minute*31, "waitinvoice", label)

		banuntil := time.Now()
		banuntil.AddDate(0, 0, 1)

		if err != nil {
			if _, ok := err.(lightning.ErrorTimeout); ok {
				goto kick
			} else {
				log.Error().Err(err).Msg("error on waitinvoice for kicking user")
			}
		}

		if inv.Get("status").String() != "paid" {
			goto kick
		} else {
			goto allow
		}

	kick:
		// didn't pay. kick.
		bot.KickChatMember(tgbotapi.KickChatMemberConfig{
			chatmemberconfig,
			banuntil.Unix(),
		})
		return

	allow:
		// the user did pay. allow.
		delete(pendingApproval, label)
		return

	}(label, chatmemberconfig)
}

func interceptMessage(message *tgbotapi.Message) (proceed bool) {
	label := fmt.Sprintf("newmember:%d:%d", message.From.ID, message.Chat.ID)
	if _, isPending := pendingApproval[label]; isPending {
		return false
	}
	return true
}
