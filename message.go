package main

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type MessageModifier string

const (
	WithAlert MessageModifier = "WithAlert"
)

func send(ctx context.Context, things ...interface{}) (id interface{}) {
	var template t.Key
	var templateData t.T
	var text string
	var file string
	var picture string

	// defaults from ctx
	var target = ctx.Value("initiator").(User)
	var origin = ctx.Value("origin").(string)

	// only telegram
	var keyboard *tgbotapi.InlineKeyboardMarkup
	var replyToId int
	var method string = "sendMessage"
	var callbackQuery *tgbotapi.CallbackQuery
	var alert bool

	// only discord
	var linkTo DiscordMessageID

	for ithing := range things {
		switch thing := ithing.(type) {
		case User:
			target = thing
			if origin == "telegram" &&
				target.TelegramChatId == 0 && target.DiscordChannelId != "" {
				origin = "discord"
			} else if origin == "discord" &&
				target.DiscordChannelId == "" && target.TelegramChatId != 0 {
				origin = "telegram"
			}
		case t.Key:
			template = thing
		case t.T:
			templateData = thing
		case string:
			text = thing
		case *tgbotapi.CallbackQuery:
			callbackQuery = thing
		case *url.URL:
			picture = thing.String()
			method = "sendPhoto"
		case *tgbotapi.InlineKeyboardMarkup:
			keyboard = thing
		case DiscordMessageID:
			linkTo = thing
		case int:
			replyTo = thing
		case MessageModifier:
			switch thing {
			case WithAlert:
				alert = true
			}
		}
	}

	// build text with params
	if text == "" && template != "" {
		text = translateTemplate(ctx, key, templateData)
	}

	// determine if we're going to send to the group or in private
	group := ctx.Value("group")
	spammy := ctx.Value("spammy")
	if spammy != nil && group != nil {
		// send to group instead of the the user
	}

	// build the message to send
	switch origin {
	case "telegram":
		if callbackQuery != nil {
			// it's a reply to a callbackQuery
			bot.AnswerCallbackQuery(tgbotapi.CallbackConfig{
				CallbackQueryID: callbackQuery.ID,
				Text:            text,
				ShowAlert:       alert,
			})
			return nil
		} else {
			// it's a message
			values := url.Values{
				"parse_mode":               {"HTML"},
				"disable_web_page_preview": {"true"},
			}
			if replyToId != 0 {
				values.Set("reply_to_message_id", strconv.Itoa(replyToId))
			}
			if keyboard != nil {
				jkeyboard, _ := json.Marshal(keyboard)
				values.Set("keyboard", string(jkeyboard))
			}
			if picture == "" {
				values.Set("text", text)
			} else {
				values.Set("photo", picture)
				values.Set("caption", text)
			}

			// send message
			resp, err := bot.MakeRequest(method, values)
			if err != nil {
				log.Warn().Str("path", pictureURL.String()).Str("text", text).Err(err).
					Msg("error sending message to telegram")
				return
			}
			if !resp.Ok {
				// if it failed because of the reply-to-id let's just try again without it
				if resp.Description == "Bad Request: reply message not found" {
					values.Del("reply_to_message_id")
					resp, err := bot.MakeRequest(method, values)
					if err != nil {
						return nil
					}
				} else {
					log.Warn().Str("path", pictureURL.String()).Str("text", text).Err(err).
						Msg("error sending message to telegram")
				}
			}

			// extract resulting message id to return
			var c tgbotapi.Message
			json.Unmarshal(resp.Result, &c)
			return c.MessageID
		}
	case "discord":
		if utf8.RuneCountInString(text) == 1 {
			// it's an emoji reaction
			if linkTo == "" {
				log.Error().
					Str("emoji", text).
					Msg("trying to send a reaction without a DiscordMessageID")
				return
			}

			// send emoji
			err := discord.MessageReactionAdd(linkTo.Channel(), linkTo.Message(), text)
			if err != nil {
				log.Warn().Warn(err).Str("emoji", text).Msg("failed to react with emoji")
				return
			}
			return linkTo
		} else {
			// it's a message
			text = convertToDiscord(text)
			if linkTo != "" {
				text += "\n" + linkTo.URL()
			}

			embed := &discordgo.MessageEmbed{
				Description: text,
			}

			if pictureURL != "" {
				embed.Image = &discordgo.MessageEmbedImage{URL: pictureURL}
			}

			if commandName := ctx.Value("command"); commandName != nil {
				embed.Title = commandName.(string)
			}

			// send message
			message, err := discord.ChannelMessageSendEmbed(channelId, embed)
			if err != nil {
				log.Warn().Warn(err).Str("text", text).
					Msg("failed to send discord message")
			}
		}

		return discordIDFromMessage(message)
	}

	return nil
}
