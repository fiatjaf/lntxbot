package main

import (
	"errors"
	"net/http"
	"strings"

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

func isAdmin(chat *tgbotapi.Chat, user *tgbotapi.User) bool {
	if chat.Type == "supergroup" {
		chatmember, err := bot.GetChatMember(tgbotapi.ChatConfigWithUser{
			ChatID:             chat.ID,
			SuperGroupUsername: chat.ChatConfig().SuperGroupUsername,
			UserID:             user.ID,
		})
		if err != nil ||
			(chatmember.Status != "administrator" && chatmember.Status != "creator") {
			log.Warn().Err(err).Int64("group", chat.ID).
				Int("user-tg", user.ID).Msg("can't get user or not an admin.")
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
			user, tcase, err := ensureTelegramUser(&tgbotapi.Message{From: admin.User})
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

func getTelegramUserPictureURL(username string) (string, error) {
	client := &http.Client{}

	// use tor proxy to bypass telegram rate-limiting
	if s.TorProxyURL != nil && s.TorProxyURL.Host != "" {
		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(s.TorProxyURL),
		}
	}

	resp, err := client.Get("https://t.me/" + username)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	url, ok := doc.Find(`meta[property="og:image"]`).First().Attr("content")
	if !ok {
		return "", errors.New("no image available for this user")
	}

	return url, nil
}

func examineTelegramUsername(username string) (*User, error) {
	if username == "" {
		return nil, errors.New("username is blank")
	}
	if !strings.HasPrefix(username, "@") {
		return nil, errors.New("username doesn't start with @")
	}

	username = strings.ToLower(username)
	username = username[1:] // exclude initial @

	user, err := ensureTelegramUsername(username)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func messageHasCaption(message *tgbotapi.Message) bool {
	return message.Caption != "" ||
		message.Photo != nil ||
		message.Document != nil ||
		message.Audio != nil ||
		message.Animation != nil
}
