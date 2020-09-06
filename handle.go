package main

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func handle(upd tgbotapi.Update) {
	switch {
	case upd.Message != nil:
		// people joining
		if upd.Message.NewChatMembers != nil {
			for _, newmember := range *upd.Message.NewChatMembers {
				handleNewMember(upd.Message, newmember)
			}
		}

		// normal message
		proceed := interceptMessage(upd.Message)
		if proceed {
			handleMessage(upd.Message)
		} else {
			go deleteMessage(upd.Message)
		}
	case upd.ChannelPost != nil:
		handleMessage(upd.ChannelPost)
	case upd.CallbackQuery != nil:
		// is temporarily s.Banned?
		if _, ok := s.Banned[upd.CallbackQuery.From.ID]; ok {
			log.Debug().Int("tgid", upd.CallbackQuery.From.ID).Msg("got request from banned user")
			return
		}

		handleCallback(upd.CallbackQuery)
	case upd.InlineQuery != nil:
		go handleInlineQuery(upd.InlineQuery)
	case upd.EditedMessage != nil:
		handleEditedMessage(upd.EditedMessage)
	}
}
