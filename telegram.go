package main

import (
	"errors"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func sendMessage(chatId int64, msg string) tgbotapi.Message { return sendMessageAsReply(chatId, msg, 0) }
func sendMessageAsReply(chatId int64, msg string, replyToId int) tgbotapi.Message {
	return sendMessageWithKeyboard(chatId, msg, nil, replyToId)
}

func sendMessageWithKeyboard(chatId int64, msg string, keyboard *tgbotapi.InlineKeyboardMarkup, replyToId int) tgbotapi.Message {
	chattable := tgbotapi.NewMessage(chatId, msg)
	chattable.BaseChat.ReplyToMessageID = replyToId
	chattable.ParseMode = "HTML"
	chattable.DisableWebPagePreview = true
	if keyboard != nil {
		chattable.BaseChat.ReplyMarkup = *keyboard
	}
	message, err := bot.Send(chattable)
	if err != nil {
		if strings.Index(err.Error(), "reply message not found") != -1 {
			chattable.BaseChat.ReplyToMessageID = 0
			message, err = bot.Send(chattable)
		}

		log.Warn().Err(err).Int64("chat", chatId).Str("msg", msg).Msg("error sending message")
	}
	return message
}

func sendMessageWithPicture(chatId int64, picturepath string, message string) tgbotapi.Message {
	if picturepath == "" {
		return sendMessage(chatId, message)
	} else {
		defer os.Remove(picturepath)
		photo := tgbotapi.NewPhotoUpload(chatId, picturepath)
		photo.Caption = message
		photo.ParseMode = "HTML"
		c, err := bot.Send(photo)
		if err != nil {
			log.Warn().Str("path", picturepath).Str("message", message).Err(err).
				Msg("error sending photo")
			return sendMessage(chatId, message)
		} else {
			return c
		}
	}
}

func sendMessageWithAnimationId(chatId int64, fileId string, message string) tgbotapi.Message {
	video := tgbotapi.NewAnimationShare(chatId, fileId)
	video.Caption = message
	video.ParseMode = "HTML"
	c, err := bot.Send(video)
	if err != nil {
		log.Warn().Str("id", fileId).Str("message", message).Err(err).Msg("error sending video")
	}
	return c
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

func appendTextToMessage(cb *tgbotapi.CallbackQuery, text string) {
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
				Msg("can't get user or not an admin.")
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
			user, tcase, err := ensureUser(admin.User.ID, admin.User.UserName, admin.User.LanguageCode)
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
