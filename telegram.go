package main

import (
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func tgsend(chattable tgbotapi.Chattable) (sent tgbotapi.Message, err error) {
	sent, err = bot.Send(chattable)
	if err != nil && strings.Index(err.Error(), "reply message not found") != -1 {
		switch c := chattable.(type) {
		case tgbotapi.MessageConfig:
			c.BaseChat.ReplyToMessageID = 0
			return tgsend(c)
		}
	} else if err != nil {
		return
	}
	return sent, nil
}

func sendTelegramMessage(chatId int64, msg string) tgbotapi.Message {
	return sendTelegramMessageAsReply(chatId, msg, 0)
}

func sendTelegramMessageAsReply(chatId int64, msg string, replyToId int) tgbotapi.Message {
	return sendTelegramMessageWithKeyboard(chatId, msg, nil, replyToId)
}

func sendTelegramMessageWithKeyboard(chatId int64, msg string, keyboard *tgbotapi.InlineKeyboardMarkup, replyToId int) tgbotapi.Message {
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
		} else {
			log.Warn().Err(err).Int64("chat", chatId).Str("msg", msg).
				Msg("sending telegram keyboard message")
		}
	}
	return message
}

func sendTelegramMessageAsText(chatId int64, msg string) tgbotapi.Message {
	chattable := tgbotapi.NewMessage(chatId, msg)
	chattable.DisableWebPagePreview = true
	c, err := bot.Send(chattable)
	if err != nil {
		log.Warn().Str("message", msg).Err(err).Msg("sending telegram text message")
	}
	return c
}

func sendTelegramMessageWithPicture(
	chatId int64,
	pictureURL *url.URL,
	text string,
) tgbotapi.Message {
	resp, err := bot.MakeRequest("sendPhoto", url.Values{
		"chat_id":    {strconv.FormatInt(chatId, 10)},
		"photo":      {pictureURL.String()},
		"caption":    {text},
		"parse_mode": {"HTML"},
	})
	if err != nil || !resp.Ok {
		log.Warn().Str("path", pictureURL.String()).Str("text", text).Err(err).
			Msg("sending telegram photo")
		return sendTelegramMessage(chatId, text)
	} else {
		var c tgbotapi.Message
		json.Unmarshal(resp.Result, &c)
		return c
	}
}

func sendTelegramMessageWithAnimationId(chatId int64, fileId string, message string) tgbotapi.Message {
	video := tgbotapi.NewAnimationShare(chatId, fileId)
	video.Caption = message
	video.ParseMode = "HTML"
	c, err := bot.Send(video)
	if err != nil {
		log.Warn().Str("id", fileId).Str("message", message).Err(err).
			Msg("sending telegram video")
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
