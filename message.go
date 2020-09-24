package main

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type MessageModifier string

const (
	EDIT        MessageModifier = "EDIT"
	APPEND      MessageModifier = "APPEND"
	WITHALERT   MessageModifier = "WITHALERT"
	FORCESPAMMY MessageModifier = "FORCESPAMMY"
)

func send(ctx context.Context, things ...interface{}) (id interface{}) {
	var edit bool
	var justAppend bool
	var template t.Key
	var templateData t.T
	var text string
	var pictureURL string
	var documentURL string

	// defaults from ctx
	var origin string
	if iorigin := ctx.Value("initiator"); iorigin != nil {
		origin, _ = iorigin.(string)
	}
	var target *User
	if itarget := ctx.Value("initiator"); itarget != nil {
		if ftarget, ok := itarget.(User); ok {
			target = &ftarget
		}
	}
	var group *GroupChat
	if igroup := ctx.Value("group"); igroup != nil {
		if fgroup, ok := igroup.(GroupChat); ok {
			group = &fgroup
		}
	}
	var spammy bool
	if ispammy := ctx.Value("spammy"); ispammy != nil {
		spammy = ispammy.(bool)
	}
	var locale string
	if ilocale := ctx.Value("locale"); ilocale != nil {
		locale = ilocale.(string)
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

	for _, ithing := range things {
		switch thing := ithing.(type) {
		case User:
			target = &thing
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
			origin = "telegram"
		case *url.URL:
			spl := strings.Split(thing.Path, ".")
			ext := spl[len(spl)-1]

			if len(ext) > 5 || ext == "png" || ext == "jpg" || ext == "jpeg" {
				method = "sendPhoto"
				pictureURL = thing.String()
			} else {
				method = "sendDocument"
				documentURL = thing.String()
			}
		case *tgbotapi.InlineKeyboardMarkup:
			keyboard = thing
			// if telegram, this will be ignored
		case tgbotapi.ForceReply:
			forceReply = tgbotapi.ForceReply{ForceReply: true}
			// if not telegram, this will be ignored
		case DiscordMessageID:
			linkTo = thing
			origin = "discord"
		case int:
			replyToId = thing
		case *tgbotapi.Message:
			telegramMessage = thing
			origin = "telegram"
		case *discordgo.Message:
			discordMessage = thing
			origin = "discord"
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
			case APPEND:
				edit = true
				justAppend = true
			}
		default:
			log.Debug().Interface("param", ithing).Msg("unrecognized param on send()")
		}
	}

	// build text with params
	if text == "" && template != "" {
		// fallback locale to user
		if locale == "" {
			if target != nil {
				locale = target.Locale
			}
		}
		// TODO must also use group locale if we're going to send to a group

		ctx = context.WithValue(ctx, "locale", locale)
		text = translateTemplate(ctx, template, templateData)
	}

	// either a user or a group must be a target (or there should be a callback)
	if target == nil && group == nil && callbackQuery == nil {
		log.Error().Str("text", text).Msg("no target user or group for message")
		return nil
	}

	// determine if we're going to send to the group or in private
	var useGroup = (!spammy && group != nil) || (group != nil && target == nil)

	// can be "api", "background", "external"
	if origin != "telegram" && origin != "discord" {
		if useGroup {
			origin = "telegram"
			// TODO discord group
		} else if target.TelegramChatId != 0 {
			origin = "telegram"
		} else if target.DiscordChannelId != "" {
			origin = "discord"
		} else {
			log.Error().Str("text", text).Str("user", target.Username).
				Msg("can't send message, user has no chat ids")
			return nil
		}
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

			if useGroup {
				// send to group instead of the the user
				values.Set("chat_id", strconv.FormatInt(-group.TelegramId, 10))
			} else {
				// send to user
				values.Set("chat_id", strconv.FormatInt(target.TelegramChatId, 10))
			}

			// editing
			canEdit := (callbackQuery != nil && callbackQuery.InlineMessageID != "") ||
				telegramMessage != nil || replyToId != 0

			if edit && canEdit {
				if callbackQuery != nil && callbackQuery.InlineMessageID != "" {
					values.Set("inline_message_id", callbackQuery.InlineMessageID)
					if callbackQuery.Message != nil && justAppend {
						text = callbackQuery.Message.Text + " " + text
					}
				} else if telegramMessage != nil {
					values.Set("chat_id", strconv.FormatInt(telegramMessage.Chat.ID, 10))
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
				if pictureURL == "" && !edit /* can't send pictures when editing */ {
					values.Set("text", text)
				} else if pictureURL != "" {
					values.Set("photo", pictureURL)
					values.Set("caption", text)
				} else if documentURL != "" {
					values.Set("document", documentURL)
					values.Set("caption", text)
				}
			}

			// send message
			resp, err := bot.MakeRequest(method, values)
			if err != nil {
				log.Warn().Str("text", text).Err(err).
					Msg("error sending message to telegram")
				return
			}
			if !resp.Ok {
				// if it failed because of the reply-to-id just try again without it
				if resp.Description == "Bad Request: reply message not found" {
					values.Del("reply_to_message_id")
					resp, err = bot.MakeRequest(method, values)
					if err != nil {
						return nil
					}
				} else {
					log.Warn().Str("text", text).Err(err).
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
				log.Warn().Err(err).Str("emoji", text).
					Msg("failed to react with emoji")
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
			if !spammy && group != nil {
				// send to group instead of the the user
				// TODO(group)
			} else {
				// send to user
				channelId = target.DiscordChannelId
			}

			// send message
			message, err := discord.ChannelMessageSendEmbed(channelId, embed)
			if err != nil {
				log.Warn().Err(err).Str("text", text).
					Msg("failed to send discord message")
			}

			log.Print(discordMessage)

			return discordIDFromMessage(message)
		}
	}

	return nil
}

func removeKeyboardButtons(ctx context.Context) {
	send(ctx, &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{},
	})
}
