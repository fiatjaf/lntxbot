package main

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func handle(upd tgbotapi.Update) {
	ctx := context.WithValue(context.Background(), "origin", "telegram")

	switch {
	case upd.Message != nil:
		// people joining
		if upd.Message.NewChatMembers != nil {
			for _, newmember := range *upd.Message.NewChatMembers {
				handleTelegramNewMember(ctx, upd.Message, newmember)
			}
			return
		}

		// people leaving
		if upd.Message.LeftChatMember != nil {
			return
		}

		// normal message
		proceed := interceptMessage(upd.Message)
		if proceed {
			handleTelegramMessage(ctx, upd.Message)
		} else {
			go deleteMessage(upd.Message)
		}
	case upd.ChannelPost != nil:
		handleTelegramMessage(ctx, upd.ChannelPost)
	case upd.CallbackQuery != nil:
		// is temporarily s.Banned?
		if _, ok := s.Banned[upd.CallbackQuery.From.ID]; ok {
			log.Debug().Int("tgid", upd.CallbackQuery.From.ID).
				Msg("got request from banned user")
			return
		}

		handleTelegramCallback(ctx, upd.CallbackQuery)
	case upd.InlineQuery != nil:
		go handleInlineQuery(ctx, upd.InlineQuery)
	}
}
