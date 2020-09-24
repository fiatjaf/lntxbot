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
	EDIT        MessageModifier = "EDIT"
	JUSTAPPEND  MessageModifier = "JUSTAPPEND"
	WITHALERT   MessageModifier = "WITHALERT"
	FORCESPAMMY MessageModifier = "FORCESPAMMY"
)

func send(ctx context.Context, things ...interface{}) (id interface{}) {
	var edit bool
	var justAppend bool
	var template t.Key
	var templateData t.T
	var text string
	var file string
	var picture string

	// defaults from ctx
	var target = ctx.Value("initiator").(User)
	var origin = ctx.Value("origin").(string)
	var group GroupChat
	var spammy bool
	if igroup := ctx.Value("group"); igroup != nil {
		group = igroup.(GroupChat)
	}
	if ispammy := ctx.Value("spammy"); ispammy != nil {
		spammy = ispammy.(bool)
	}

	// only telegram
	var keyboard *tgbotapi.InlineKeyboardMarkup
	var forceReply tgbotapi.ForceReply
	var replyToId int                     // will be sent in reply to this -- or if editing will edit this
	var telegramMessage *tgbotapi.Message // unless this is provided, this has precedence in edition priotiry
	var method string = "sendMessage"
	var callbackQuery *tgbotapi.CallbackQuery
	var alert bool

	// only discord
	var linkTo DiscordMessageID
	var discordMessage *discordgo.Message

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
		case tgbotapi.ForceReply:
			forceReply = tgbotapi.ForceReply
		case DiscordMessageID:
			linkTo = thing
		case int:
			replyTo = thing
		case *tgbotapi.Message:
			telegramMessage = thing
		case *discordgo.Message:
			discordMessage = thing
		case MessageModifier:
			switch thing {
			case WITHALERT:
				alert = true
			case FORCESPAMMY:
				spammy = true
			case EDIT:
				edit = true
				if edit {
					method = "editMessageText"
				}
			case JUSTAPPEND:
				justAppend = true
			}
		}
	}

	// build text with params
	if text == "" && template != "" {
		text = translateTemplate(ctx, key, templateData)
	}

	// build the message to send
	switch origin {
	case "telegram":
		if callbackQuery != nil || !edit {
			// it's a reply to a callbackQuery
			bot.AnswerCallbackQuery(tgbotapi.CallbackConfig{
				CallbackQueryID: callbackQuery.ID,
				Text:            text,
				ShowAlert:       alert,
			})
			return nil
		} else {
			// it's a message (or a call to edit a message)
			values := url.Values{
				"parse_mode":               {"HTML"},
				"disable_web_page_preview": {"true"},
			}
			if keyboard != nil {
				jkeyboard, _ := json.Marshal(keyboard)
				values.Set("keyboard", string(jkeyboard))
			} else if forceReply.ForceReply {
				jforceReply, _ := json.Marshal(forceReply)
				values.Set("forceReply", string(jforceReply))
			}

			// determine if we're going to send to the group or in private
			if spammy != nil && group != nil {
				// send to group instead of the the user
				values.Set("chat_id", strconv.Itoa(-group.(GroupChat).TelegramId))
			} else {
				// send to user
				values.Set("chat_id", strconv.Itoa(target.TelegramChatId))
			}

			// editing
			canEdit := (callbackQuery != nil && callbackQuery.InlineMessageID != "") ||
				telegramMessage != nil || replyToId != 0

			if edit && canEdit {
				if callbackQuery != nil && callbackQuery.InlineMessageID != "" {
					values.Set("inline_message_id", callbackQuery.InlineMessageID)
					if callbackQuery.Message && justAppend {
						text = callbackQuery.Message.Text + " " + text
					}
				} else if telegramMessage != nil {
					values.Set("chat_id", strconv.Itoa(telegramMessage.Chat.ID))
					values.Set("message_id", strconv.Itoa(telegramMessage.MessageID))
					if justAppend {
						text = telegramMessage.Text + " " + text
					}
				} else if replyToId != 0 {
					values.Set("message_id", strconv.Itoa(replyToId))
					if justAppend {
						log.Error().Str("text", text).
							Msg("can't append to a message if we only have its id")
						return
					}
				}
			} else {
				// not editing, can add pictures and reply_to targets
				if replyToId != 0 {
					values.Set("reply_to_message_id", strconv.Itoa(replyToId))
				} else if telegramMessage != nil {
					values.Set("reply_to_message_id", strconv.Itoa(
						telegramMessage.MessageID))
				}
				if picture == "" && !edit /* when editing we can't send pictures */ {
					values.Set("text", text)
				} else {
					values.Set("photo", picture)
					values.Set("caption", text)
				}
			}

			// send message
			resp, err := bot.MakeRequest(method, values)
			if err != nil {
				log.Warn().Str("path", pictureURL.String()).Str("text", text).Err(err).
					Msg("error sending message to telegram")
				return
			}
			if !resp.Ok {
				// if it failed because of the reply-to-id just try again without it
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
			// it's a message TODO(edit)
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

			var channelId string
			if spammy != nil && group != nil {
				// send to group instead of the the user
				// TODO(group)
			} else {
				// send to user
				channelId = target.DiscordChannelId
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
