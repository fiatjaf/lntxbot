package main

import (
	"errors"

	"github.com/PuerkitoBio/goquery"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func sendTelegramMessageWithAnimationId(chatId int64, fileId string, message string) (id interface{}) {
	video := tgbotapi.NewAnimationShare(chatId, fileId)
	video.Caption = message
	video.ParseMode = "HTML"
	c, err := bot.Send(video)
	if err != nil {
		log.Warn().Str("id", fileId).Str("message", message).Err(err).
			Msg("sending telegram video")
	}
	return c.MessageID
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

func appendToTelegramMessage(cb *tgbotapi.CallbackQuery, text string) {
	if cb.Message != nil {
		text = cb.Message.Text + " " + text
	}

	baseEdit := getBaseEdit(cb)
	bot.Send(tgbotapi.EditMessageTextConfig{
		BaseEdit:              baseEdit,
		Text:                  text,
		DisableWebPagePreview: true,
	})
}

func edit(message *tgbotapi.Message, newText string) {
	bot.Send(tgbotapi.EditMessageTextConfig{
		BaseEdit: tgbotapi.BaseEdit{
			ChatID:    message.Chat.ID,
			MessageID: message.MessageID,
		},
		Text:                  newText,
		DisableWebPagePreview: true,
	})
}

func editAppend(message *tgbotapi.Message, textToAppend string) {
	edit(message, message.Text+textToAppend)
}

func editWithKeyboard(chat int64, msg int, text string, keyboard tgbotapi.InlineKeyboardMarkup) {
	edit := tgbotapi.NewEditMessageText(chat, msg, text)
	edit.ParseMode = "HTML"
	edit.DisableWebPagePreview = true
	edit.BaseEdit.ReplyMarkup = &keyboard
	bot.Send(edit)
}

func isAdmin(chat *tgbotapi.Chat, user *tgbotapi.User) bool {
	if chat.Type == "supergroup" {
		chatmember, err := bot.GetChatMember(tgbotapi.ChatConfigWithUser{
			ChatID:             chat.ID,
			SuperGroupUsername: chat.ChatConfig().SuperGroupUsername,
			UserID:             user.ID,
		})
		if err != nil ||
			(chatmember.Status != "administrator" && chatmember.Status != "creator") {
			log.Warn().Err(err).
				Int64("group", chat.ID).
				Int("user", user.ID).
				Msg("can't get user or not an admin.")
			return false
		}

		return true
	} else if chat.Type == "group" {
		// ok, everybody can toggle
		return true
	} else if chat.Type == "channel" {
		// if you're posting then you're an admin
		return true
	} else {
		return false
	}
}

func deleteMessage(message *tgbotapi.Message) {
	if message == nil || message.Chat == nil {
		return
	}
	bot.Send(tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID))
}

func forwardMessage(message *tgbotapi.Message, targetChat int64) {
	bot.Send(tgbotapi.NewForward(targetChat, message.Chat.ID, message.MessageID))
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
			user, tcase, err := ensureTelegramUser(admin.User.ID, admin.User.UserName, admin.User.LanguageCode)
			if err != nil {
				log.Warn().Err(err).Int("case", tcase).
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

func getUserPictureURL(username string) (string, error) {
	doc, err := goquery.NewDocument("https://t.me/" + username)
	if err != nil {
		return "", err
	}

	image, ok := doc.Find(`meta[property="og:image"]`).First().Attr("content")
	if !ok {
		return "", errors.New("no image available for this user")
	}

	return image, nil
}
