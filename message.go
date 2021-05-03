package main

import (
	"context"
	"encoding/json"
	"errors"
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

	log := log.With().Interface("origin", ctx.Value("origin")).Logger()

	// defaults from ctx
	var origin string
	if iorigin := ctx.Value("origin"); iorigin != nil {
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
	var forceSpammy bool
	var hasExplicitTarget bool
	var spammy bool
	if ispammy := ctx.Value("spammy"); ispammy != nil {
		spammy = ispammy.(bool)
	}
	var locale string
	if ilocale := ctx.Value("locale"); ilocale != nil {
		locale = ilocale.(string)
	}

	// only telegram
	var chatId int64
	var keyboard *tgbotapi.InlineKeyboardMarkup
	var mustSendAnActualMessage bool
	var forceReply *tgbotapi.ForceReply
	var replyToId int                     // will be sent in reply to this -- or if editing will edit this
	var telegramMessage *tgbotapi.Message // unless this is provided, this has precedence in edition priotiry
	var alert bool
	var callbackQuery *tgbotapi.CallbackQuery

	if icb := ctx.Value("callbackQuery"); icb != nil {
		callbackQuery = icb.(*tgbotapi.CallbackQuery)
		origin = "telegram"
	}

	// only discord
	var linkTo DiscordMessageID
	var discordMessage *discordgo.Message
	var emojiReaction string

	for _, ithing := range things {
		switch thing := ithing.(type) {
		case *User:
			target = thing
			mustSendAnActualMessage = true
			hasExplicitTarget = true
		case User:
			target = &thing
			mustSendAnActualMessage = true
			hasExplicitTarget = true
		case *GroupChat:
			group = thing
			mustSendAnActualMessage = true
		case GroupChat:
			group = &thing
			mustSendAnActualMessage = true
		case int64:
			chatId = thing
			mustSendAnActualMessage = true
		case t.Key:
			template = thing
		case t.T:
			templateData = thing
		case string:
			if utf8.RuneCountInString(text) == 1 {
				emojiReaction = thing
				origin = "discord"
			} else {
				text = thing
			}
		case *tgbotapi.CallbackQuery:
			callbackQuery = thing
			origin = "telegram"
		case *url.URL:
			spl := strings.Split(thing.Path, ".")
			ext := spl[len(spl)-1]

			if strings.HasPrefix(thing.Path, "/qr/") ||
				ext == "png" || ext == "jpg" || ext == "jpeg" {
				pictureURL = thing.String()
			} else {
				documentURL = thing.String()
			}
		case *tgbotapi.InlineKeyboardMarkup:
			keyboard = thing
			// if telegram, this will be ignored
		case *tgbotapi.ForceReply:
			forceReply = thing
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
				forceSpammy = true
			case EDIT:
				edit = true
			case APPEND:
				edit = true
				justAppend = true
			}
		case nil:
			// ignore
		default:
			log.Debug().Interface("param", ithing).Msg("unrecognized param on send()")
		}
	}

	log = log.With().Str("key", string(template)).Stringer("user", target).
		Bool("alert", alert).Bool("spammy", spammy).Bool("edit", edit).
		Bool("append", justAppend).Bool("keyboard", keyboard != nil).
		Bool("cb", callbackQuery != nil).Stringer("group", group).
		Logger()

	// get origin from user if not present
	if origin == "" && target != nil {
		if origin == "telegram" &&
			target.TelegramChatId == 0 && target.DiscordChannelId != "" {
			origin = "discord"
		} else if origin == "discord" &&
			target.DiscordChannelId == "" && target.TelegramChatId != 0 {
			origin = "telegram"
		}
	}

	// build text with params
	if text == "" && template != "" {
		// fallback locale to user or group
		if locale == "" && group != nil {
			locale = group.Locale
		}
		if locale == "" && target != nil {
			locale = target.Locale
		}

		ctx = context.WithValue(ctx, "locale", locale)
		text = translateTemplate(ctx, template, templateData)
		text = strings.TrimSpace(text)
	}

	// either a user or a group must be a target (or there should be a callback)
	if target == nil && group == nil && callbackQuery == nil {
		log.Error().Msg("no target user or group for message")
		return nil
	}

	// determine if we're going to send to the group or in private
	var groupId = chatId // may be zero if not given
	if group != nil {
		groupId = group.TelegramId
	}
	var useGroup = (spammy && !hasExplicitTarget && groupId != 0) ||
		(forceSpammy && groupId != 0) ||
		(groupId != 0 && target == nil)

	// origin can be "api", "background", "external"
	if origin != "telegram" && origin != "discord" {
		// here we try to determine where to notify the user
		// this is only used when he is not interacting with the bot directly
		if useGroup {
			origin = "telegram"
			// TODO discord group
		} else if target.TelegramChatId != 0 {
			origin = "telegram"
		} else if target.DiscordChannelId != "" {
			origin = "discord"
		} else {
			log.Error().Msg("can't send message, user has no chat ids")
			return nil
		}
	}

	// build the message to send
	switch origin {
	case "telegram":
		if callbackQuery != nil && !edit && !mustSendAnActualMessage {
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
				values.Set("reply_markup", string(jkeyboard))
			} else if forceReply != nil {
				jforceReply, _ := json.Marshal(forceReply)
				values.Set("reply_markup", string(jforceReply))
			}

			if useGroup {
				// send to group instead of the the user
				values.Set("chat_id", strconv.FormatInt(groupId, 10))
			} else {
				// send to user
				values.Set("chat_id", strconv.FormatInt(target.TelegramChatId, 10))
			}

			// editing
			canEdit := (telegramMessage != nil) || (replyToId != 0) ||
				(callbackQuery != nil && callbackQuery.InlineMessageID != "") ||
				(callbackQuery != nil && callbackQuery.Message.MessageID != 0)

			var method string
			if edit && canEdit {
				if callbackQuery != nil {
					if callbackQuery.Message != nil {
						values.Set("chat_id", strconv.FormatInt(
							callbackQuery.Message.Chat.ID, 10))
						values.Set("message_id", strconv.Itoa(
							callbackQuery.Message.MessageID))

						if justAppend || text == "" {
							if messageHasCaption(callbackQuery.Message) {
								text = callbackQuery.Message.Caption + " " + text
								method = "editMessageCaption"
								values.Set("caption", text)
							} else {
								text = callbackQuery.Message.Text + " " + text
								method = "editMessageText"
								values.Set("text", text)
							}
						} else if messageHasCaption(callbackQuery.Message) {
							method = "editMessageCaption"
							values.Set("caption", text)
						} else if text == "" && values.Get("reply_markup") != "" {
							method = "editMessageReplyMarkup"
						}
					} else if callbackQuery.InlineMessageID != "" {
						values.Del("chat_id")
						values.Set("inline_message_id", callbackQuery.InlineMessageID)
						if callbackQuery.Message != nil && justAppend {
							text = callbackQuery.Message.Text + " " + text
						}
						if text == "" && values.Get("reply_markup") != "" {
							method = "editMessageReplyMarkup"
						}
					}
				} else if telegramMessage != nil {
					values.Set("chat_id", strconv.FormatInt(
						telegramMessage.Chat.ID, 10))
					values.Set("message_id", strconv.Itoa(telegramMessage.MessageID))

					if justAppend || text == "" {
						if messageHasCaption(telegramMessage) {
							text = telegramMessage.Caption + " " + text
							method = "editMessageCaption"
							values.Set("caption", text)
						} else {
							text = telegramMessage.Text + " " + text
							method = "editMessageText"
							values.Set("text", text)
						}
					} else if messageHasCaption(telegramMessage) {
						method = "editMessageCaption"
						values.Set("caption", text)
					} else if text == "" && values.Get("reply_markup") != "" {
						method = "editMessageReplyMarkup"
					}
				} else if replyToId != 0 {
					values.Set("message_id", strconv.Itoa(replyToId))
					if justAppend {
						log.Error().Msg("can't append to a message with only its id")
						return
					}
				}

				if method == "" {
					method = "editMessageText"
					values.Set("text", text)
				}
			} else {
				// not editing, can add pictures and reply_to targets
				if replyToId != 0 {
					values.Set("reply_to_message_id", strconv.Itoa(replyToId))
				} else if telegramMessage != nil {
					values.Set("reply_to_message_id", strconv.Itoa(
						telegramMessage.MessageID))
				}

				if pictureURL == "" && documentURL == "" {
					method = "sendMessage"
					values.Set("text", text)
				} else if pictureURL != "" {
					values.Set("photo", pictureURL)
					values.Set("caption", text)
					method = "sendPhoto"
				} else if documentURL != "" {
					method = "sendDocument"
					values.Set("document", documentURL)
					values.Set("caption", text)
				}
			}

			log = log.With().Str("method", method).Str("chat_id", values.Get("chat_id")).
				Bool("using-group", useGroup).
				Logger()

			// send message
			resp, err := bot.MakeRequest(method, values)
			if err == nil && !resp.Ok {
				err = errors.New(resp.Description)
			}
			if err != nil {
				if err.Error() == "Bad Request: replied message not found" {
					values.Del("reply_to_message_id")
					resp, err = bot.MakeRequest(method, values)
				}
				if err != nil {
					log.Warn().Err(err).Msg("error sending message to telegram")
					return
				}
			}

			// extract resulting message id to return
			var c tgbotapi.Message
			json.Unmarshal(resp.Result, &c)
			return c.MessageID
		}
	case "discord":
		var reference = linkTo
		if reference == "" && discordMessage != nil {
			reference = discordIDFromMessage(discordMessage)
		}

		if emojiReaction != "" {
			// we're sending an emoji reaction
			if reference == "" {
				log.Error().Msg("trying to send a reaction without a reference")
				return
			}

			// send emoji
			err := discord.MessageReactionAdd(
				reference.Channel(), reference.Message(), emojiReaction)
			if err != nil {
				log.Warn().Err(err).Str("emoji", text).
					Msg("failed to react with emoji")
				return
			}
			return linkTo
		} else if documentURL != "" {
			// for documentURLs we behave differently as embeds won't work
			// TODO
		} else {
			// we're sending a message or edit
			text = convertToDiscord(text)
			if linkTo != "" {
				text += "\n" + linkTo.URL()
			}

			// build the embed object to send
			// TODO use simple messages if there just one line of text?
			embed := &discordgo.MessageEmbed{
				Description: text,
			}
			if pictureURL != "" {
				embed.Image = &discordgo.MessageEmbedImage{URL: pictureURL}
			}
			if commandName := ctx.Value("command"); commandName != nil {
				embed.Title = commandName.(string)
			}

			if edit && reference != "" {
				// editing
				if justAppend {
					if discordMessage != nil {
						text = discordMessage.Embeds[0].Description + " " + text
					} else {
						log.Error().Msg("can't append to a message with only its id")
						return
					}
				}

				// send edit
				message, err := discord.ChannelMessageEditEmbed(
					linkTo.Channel(), linkTo.Message(), embed)
				if err != nil {
					log.Warn().Err(err).Msg("failed to send discord message")
				}

				return discordIDFromMessage(message)
			} else {
				var channelId string
				if group != nil {
					spamChannelId, _ := getGuildMetadata(group.DiscordGuildId)
					message := ctx.Value("message").(*discordgo.Message)

					if spammy {
						// send to the same channel it came from
						channelId = message.ChannelID
					} else if (message.ChannelID == spamChannelId) ||
						(target.DiscordChannelId == "" && spamChannelId != "") {
						// send to the #commands or #lntxbot
						channelId = spamChannelId
					} else {
						// send to user privately
						channelId = target.DiscordChannelId
					}
				} else {
					// send to user privately
					channelId = target.DiscordChannelId
				}

				// send message
				message, err := discord.ChannelMessageSendEmbed(channelId, embed)
				if err != nil {
					log.Warn().Err(err).Msg("failed to send discord message")
					return
				}

				return discordIDFromMessage(message)
			}
		}
	}

	return nil
}

func removeKeyboardButtons(ctx context.Context) {
	send(ctx, EDIT, &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{},
	})
}
