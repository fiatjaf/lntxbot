package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"

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

type TelegramCopyMessage struct {
	ChatID    int64
	MessageID int
}

func send(ctx context.Context, things ...interface{}) (id interface{}) {
	var (
		edit         bool
		text         string
		template     t.Key
		pictureURL   string
		justAppend   bool
		documentURL  string
		templateData t.T
	)

	// defaults from ctx
	var target *User
	if itarget := ctx.Value("initiator"); itarget != nil {
		if ftarget, ok := itarget.(*User); ok {
			target = ftarget
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
	var spammy bool = true
	if ispammy := ctx.Value("spammy"); ispammy != nil {
		spammy = ispammy.(bool)
	}
	var locale string
	if ilocale := ctx.Value("locale"); ilocale != nil {
		locale = ilocale.(string)
	}

	// only telegram
	var (
		alert                   bool
		chatId                  int64
		keyboard                *tgbotapi.InlineKeyboardMarkup
		replyToId               int // will be sent in reply to this -- or if editing will edit this
		forceReply              *tgbotapi.ForceReply
		copyMessage             *TelegramCopyMessage
		callbackQuery           *tgbotapi.CallbackQuery
		telegramMessage         *tgbotapi.Message // unless this is provided, this has precedence in edition priotiry
		mustSendAnActualMessage bool
	)

	if icb := ctx.Value("callbackQuery"); icb != nil {
		callbackQuery = icb.(*tgbotapi.CallbackQuery)
	}

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
			text = thing
		case *tgbotapi.CallbackQuery:
			callbackQuery = thing
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
		case int:
			replyToId = thing
		case *tgbotapi.Message:
			telegramMessage = thing
		case *TelegramCopyMessage:
			copyMessage = thing
		case TelegramCopyMessage:
			copyMessage = &thing
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

	log := log.With().Str("key", string(template)).Stringer("user", target).
		Bool("alert", alert).Bool("spammy", spammy).Bool("edit", edit).
		Bool("append", justAppend).Bool("keyboard", keyboard != nil).
		Bool("cb", callbackQuery != nil).Stringer("group", group).
		Logger()

	// build text with params
	if text == "" && template != "" {
		// fallback locale to user
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
	groupId := chatId // may be zero if not given
	if group != nil {
		groupId = group.TelegramId
	}
	useGroup := (spammy && !hasExplicitTarget && groupId != 0) ||
		(forceSpammy && groupId != 0) ||
		(groupId != 0 && target == nil)

		// use group locale then
	if useGroup && locale == "" && group != nil {
		locale = group.Locale
	}

	// build the message to send
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

			if copyMessage != nil {
				method = "copyMessage"
				values.Set("from_chat_id", strconv.FormatInt(
					copyMessage.ChatID, 10))
				values.Set("message_id", strconv.Itoa(copyMessage.MessageID))
			} else if pictureURL == "" && documentURL == "" {
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

	return nil
}

func removeKeyboardButtons(ctx context.Context) {
	send(ctx, EDIT, &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{},
	})
}
