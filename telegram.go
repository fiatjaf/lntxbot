package main

import (
	"fmt"
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

func giveAwayKeyboard(u User, sats int) tgbotapi.InlineKeyboardMarkup {
	giveawayid := cuid.Slug()
	buttonData := fmt.Sprintf("give=%d-%d-%s", u.Id, sats, giveawayid)

	rds.Set("giveaway:"+giveawayid, buttonData, s.GiveAwayTimeout)

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Cancel", fmt.Sprintf("cancel=%d", u.Id)),
			tgbotapi.NewInlineKeyboardButtonData(
				"Claim!",
				buttonData,
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
					"<", "&lt;", -1),
				">", "&gt;", -1),
			"&", "&amp;", -1),
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
