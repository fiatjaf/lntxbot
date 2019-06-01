package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/lucsky/cuid"
)

func notify(chatId int64, msg string) tgbotapi.Message {
	return notifyAsReply(chatId, msg, 0)
}

func notifyAsReply(chatId int64, msg string, replyToId int) tgbotapi.Message {
	chattable := tgbotapi.NewMessage(chatId, msg)
	chattable.BaseChat.ReplyToMessageID = replyToId
	chattable.ParseMode = "HTML"
	chattable.DisableWebPagePreview = true
	message, err := bot.Send(chattable)
	if err != nil {
		if strings.Index(err.Error(), "reply message not found") != -1 {
			chattable.BaseChat.ReplyToMessageID = 0
			message, err = bot.Send(chattable)
		}

		log.Warn().Int64("chat", chatId).Str("msg", msg).Err(err).Msg("error sending message")
	}
	return message
}

func notifyMarkdown(chatId int64, msg string) tgbotapi.Message {
	chattable := tgbotapi.NewMessage(chatId, msg)
	chattable.ParseMode = "Markdown"
	chattable.DisableWebPagePreview = true
	message, err := bot.Send(chattable)
	if err != nil {
		log.Warn().Int64("chat", chatId).Str("msg", msg).Err(err).Msg("error sending message")
	}
	return message
}

func notifyWithPicture(chatId int64, picturepath string, message string) tgbotapi.Message {
	if picturepath == "" {
		return notify(chatId, message)
	} else {
		defer os.Remove(picturepath)
		photo := tgbotapi.NewPhotoUpload(chatId, picturepath)
		photo.Caption = message
		c, err := bot.Send(photo)
		if err != nil {
			log.Warn().Str("path", picturepath).Str("message", message).Err(err).Msg("error sending photo")
			return notify(chatId, message)
		} else {
			return c
		}
	}
}

func getBaseEdit(cb *tgbotapi.CallbackQuery) tgbotapi.BaseEdit {
	baseedit := tgbotapi.BaseEdit{
		InlineMessageID: cb.InlineMessageID,
	}

	if cb.Message != nil {
		baseedit.MessageID = cb.Message.MessageID
		baseedit.ChatID = cb.Message.Chat.ID
	}

	return baseedit
}

func giveawayKeyboard(giverId, sats int) tgbotapi.InlineKeyboardMarkup {
	giveawayid := cuid.Slug()
	buttonData := fmt.Sprintf("give=%d-%d-%s", giverId, sats, giveawayid)

	rds.Set("giveaway:"+giveawayid, buttonData, s.GiveAwayTimeout)

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Cancel", fmt.Sprintf("cancel=%d", giverId)),
			tgbotapi.NewInlineKeyboardButtonData(
				"Claim!",
				buttonData,
			),
		),
	)
}

func giveflipKeyboard(giveflipid string, giverId, nparticipants, sats int) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Cancel", fmt.Sprintf("cancel=%d", giverId)),
			tgbotapi.NewInlineKeyboardButtonData(
				"Try to win!",
				fmt.Sprintf("gifl=%d-%d-%d-%s", giverId, nparticipants, sats, giveflipid),
			),
		),
	)
}

func coinflipKeyboard(coinflipid string, nparticipants, sats int) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"Join lottery",
				fmt.Sprintf("flip=%d-%d-%s", nparticipants, sats, coinflipid),
			),
		),
	)
}

func fundraiseKeyboard(
	fundraiseid string,
	receiverId int,
	nparticipants int,
	sats int,
) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"Contribute",
				fmt.Sprintf("raise=%d-%d-%d-%s", receiverId, nparticipants, sats, fundraiseid),
			),
		),
	)
}

func escapeHTML(m string) string {
	return strings.Replace(
		strings.Replace(
			strings.Replace(
				strings.Replace(
					m,
					"&", "&amp;", -1),
				"<", "&lt;", -1),
			">", "&gt;", -1),
		"\"", "&quot;", -1)
}

func removeKeyboardButtons(cb *tgbotapi.CallbackQuery) {
	baseEdit := getBaseEdit(cb)

	baseEdit.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			[]tgbotapi.InlineKeyboardButton{},
		},
	}

	bot.Send(tgbotapi.EditMessageReplyMarkupConfig{
		BaseEdit: baseEdit,
	})
}

func appendTextToMessage(cb *tgbotapi.CallbackQuery, text string) {
	if cb.Message != nil {
		text = cb.Message.Text + " " + text
	}

	baseEdit := getBaseEdit(cb)
	bot.Send(tgbotapi.EditMessageTextConfig{
		BaseEdit: baseEdit,
		Text:     text,
	})
}

func editWithKeyboard(chat int64, msg int, text string, keyboard tgbotapi.InlineKeyboardMarkup) {
	edit := tgbotapi.NewEditMessageText(chat, msg, text)
	edit.ParseMode = "HTML"
	edit.DisableWebPagePreview = true
	edit.BaseEdit.ReplyMarkup = &keyboard
	bot.Send(edit)
}

func isAdmin(message *tgbotapi.Message) bool {
	if message.Chat.Type == "supergroup" {
		chatmember, err := bot.GetChatMember(tgbotapi.ChatConfigWithUser{
			ChatID:             message.Chat.ID,
			SuperGroupUsername: message.Chat.ChatConfig().SuperGroupUsername,
			UserID:             message.From.ID,
		})
		if err != nil ||
			(chatmember.Status != "administrator" && chatmember.Status != "creator") {
			log.Warn().Err(err).
				Int64("group", message.Chat.ID).
				Int("user", message.From.ID).
				Msg("toggle impossible. can't get user or not an admin.")
			return false
		}

		return true
	} else if message.Chat.Type == "group" {
		// ok, everybody can toggle
		return true
	} else {
		return false
	}
}

func deleteMessage(message *tgbotapi.Message) {
	bot.Send(tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID))
}

func getChatOwner(chatId int64) (User, error) {
	admins, err := bot.GetChatAdministrators(tgbotapi.ChatConfig{
		ChatID: chatId,
	})
	if err != nil {
		return User{}, err
	}

	for _, admin := range admins {
		if admin.Status == "creator" {
			user, t, err := ensureUser(admin.User.ID, admin.User.UserName)
			if err != nil {
				log.Warn().Err(err).Int("case", t).
					Str("username", admin.User.UserName).
					Int("id", admin.User.ID).
					Msg("failed to ensure user when fetching chat owner")
				return user, err
			}

			return user, nil
		}
	}

	return User{}, errors.New("chat has no owner")
}
