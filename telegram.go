package main

import (
	"errors"
	"strconv"
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

func getTelegramUserPictureURL(username string) (string, error) {
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

func examineTelegramUsername(
	message *tgbotapi.Message,
	value interface{},
) (u *User, err error) {
	var username string
	var user User
	var uid int

	switch val := value.(type) {
	case []string:
		if len(val) > 0 {
			username = strings.Join(val, " ")
		}
	case string:
		username = val
	case int:
		uid = val
	}

	if intval, err := strconv.Atoi(username); err == nil {
		uid = intval
	}

	if username != "" {
		username = strings.ToLower(username)
	}

	if username == "" && uid == 0 {
		return nil, errors.New("no user")
	}

	// check entities for user type
	if message.Entities != nil {
		for _, entity := range *message.Entities {
			if entity.Type == "text_mention" && entity.User != nil {
				// user without username
				uid = entity.User.ID
				user, err = ensureTelegramId(uid)
				if err != nil {
					return nil, err
				}

				return &user, nil
			}
			if entity.Type == "mention" {
				// user with username
				uname := username[1:]
				user, err = ensureTelegramUsername(uname)
				if err != nil {
					return nil, err
				}

				return &user, nil
			}
		}
	}

	// if the user identifier passed was neither @someone (mention) nor a text_mention
	// (for users without usernames but still painted blue and autocompleted by telegram)
	// and we have a uid that means it's the case where just a numeric id was given
	// and nothing more.
	if uid != 0 {
		user, err = ensureTelegramId(uid)
		if err != nil {
			return nil, err
		}

		return &user, nil
	}

	return nil, errors.New("no user")
}

func messageHasCaption(message *tgbotapi.Message) bool {
	return message.Caption != "" ||
		message.Photo != nil ||
		message.Document != nil ||
		message.Audio != nil ||
		message.Animation != nil
}
