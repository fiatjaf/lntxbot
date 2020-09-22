package main

import (
	"net/url"

	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func (u User) sendMessage(text string) (id interface{}) {
	if u.isTelegram() {
		return sendTelegramMessage(u.TelegramChatId, text).MessageID
	} else if u.isDiscord() {
		return sendDiscordMessage(u.DiscordChannelId, text)
	} else {
		log.Warn().Interface("user", u).
			Msg("can't message user without chat or channel")
		return nil
	}
}

func (u User) sendMessageAsReply(text string, replyToId int) (id interface{}) {
	if u.isTelegram() {
		return sendTelegramMessageAsReply(u.TelegramChatId, text, replyToId).MessageID
	} else if u.isDiscord() {
		return sendDiscordMessage(u.DiscordChannelId, text)
	} else {
		log.Warn().Interface("user", u).
			Msg("can't message user without chat or channel")
		return nil
	}
}

func (u User) sendMessageWithPicture(pictureURL *url.URL, text string) (id interface{}) {
	if u.isTelegram() {
		return sendTelegramMessageWithPicture(u.TelegramChatId, pictureURL, text).
			MessageID
	} else if u.isDiscord() {
		return sendDiscordMessageWithPicture(u.DiscordChannelId, pictureURL, text)
	} else {
		log.Warn().Interface("user", u).
			Msg("can't message user without chat or channel")
		return
	}
}

func (u User) notify(key t.Key, templateData t.T) tgbotapi.Message {
	return u.notifyAsReply(key, templateData, 0)
}

func (u User) notifyAsReply(key t.Key, templateData t.T, replyToId int) tgbotapi.Message {
	if u.isTelegram() {
		return u.notifyWithKeyboard(key, templateData, nil, replyToId)
	} else if u.isDiscord() {
		html := translateTemplate(key, u.Locale, templateData)
		sendDiscordMessage(u.DiscordChannelId, html)
	}

	return tgbotapi.Message{} // TODO return an id here maybe, so discord can work
}

func (u User) notifyWithKeyboard(key t.Key, templateData t.T, keyboard *tgbotapi.InlineKeyboardMarkup, replyToId int) tgbotapi.Message {
	if u.TelegramChatId == 0 {
		log.Info().Str("user", u.Username).Str("key", string(key)).
			Msg("can't notify user as it hasn't started a chat with the bot.")
		return tgbotapi.Message{}
	}
	log.Debug().Str("user", u.Username).
		Str("key", string(key)).Interface("data", templateData).
		Msg("notifying user")

	msg := translateTemplate(key, u.Locale, templateData)
	return sendTelegramMessageWithKeyboard(u.TelegramChatId, msg, keyboard, replyToId)
}

func (u User) alert(cb *tgbotapi.CallbackQuery, key t.Key, templateData t.T) (tgbotapi.APIResponse, error) {
	return bot.AnswerCallbackQuery(tgbotapi.NewCallbackWithAlert(cb.ID, translateTemplate(key, u.Locale, templateData)))
}
