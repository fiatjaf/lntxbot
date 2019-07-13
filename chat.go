package main

import (
	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

/*
   ALL GROUP CHAT TELEGRAM IDS ARE NEGATIVE
*/

type GroupChat struct {
	TelegramId int64  `db:"telegram_id"`
	Locale     string `db:"locale"`
	Spammy     bool   `db:"spammy"`
	Ticket     int    `db:"ticket"`
}

const GROUPCHATFIELDS = "telegram_id, locale, spammy, ticket"

func ensureGroup(telegramId int64, locale string) (g GroupChat, err error) {
	err = pg.Get(&g, `
INSERT INTO telegram.chat AS g (telegram_id, locale)
VALUES (
  $1,
  CASE WHEN $2 != '' THEN $2 ELSE DEFAULT END
)
ON CONFLICT (telegram_id)
  DO UPDATE
    SET locale = CASE WHEN $2 != '' THEN $2 ELSE g.locale END
  RETURNING `+GROUPCHATFIELDS+`
    `, -telegramId, locale)
	return
}

func loadGroup(telegramId int64) (g GroupChat, err error) {
	err = pg.Get(&g, "SELECT "+GROUPCHATFIELDS+" FROM telegram.chat WHERE telegram_id = $1", -telegramId)
	return
}

var spammy_cache = map[int64]bool{}

func toggleSpammy(telegramId int64) (spammy bool, err error) {
	err = pg.Get(&spammy, `
UPDATE telegram.chat AS g SET spammy = NOT g.spammy
WHERE telegram_id = $1
RETURNING spammy
    `, -telegramId)

	spammy_cache[-telegramId] = spammy

	return
}

func isSpammy(telegramId int64) (spammy bool) {
	if spammy, ok := spammy_cache[-telegramId]; ok {
		return spammy
	}

	err := pg.Get(&spammy, "SELECT spammy FROM telegram.chat WHERE telegram_id = $1", -telegramId)
	if err != nil {
		return false
	}

	spammy_cache[-telegramId] = spammy

	return
}

type KickData struct {
	InvoiceMessage   tgbotapi.Message          `json:"invoice_message"`
	NotifyMessage    tgbotapi.Message          `json:"notify_message"`
	JoinMessage      tgbotapi.Message          `json:"join_message"`
	ChatMemberConfig tgbotapi.ChatMemberConfig `json:"chat_member_config"`
	NewMember        tgbotapi.User             `json:"new_member"`
	Hash             string                    `json:"hash"`
}

func setTicketPrice(telegramId int64, sat int) (err error) {
	_, err = pg.Exec(`
UPDATE telegram.chat SET ticket = $2
WHERE telegram_id = $1
    `, -telegramId, sat)
	return
}

func (g GroupChat) notify(key t.Key, templateData t.T) tgbotapi.Message {
	return g.notifyAsReply(key, templateData, 0)
}

func (g GroupChat) notifyAsReply(key t.Key, templateData t.T, replyToId int) tgbotapi.Message {
	log.Debug().Int64("chat", g.TelegramId).Str("key", string(key)).Interface("data", templateData).Msg("posting to group")
	msg := translateTemplate(key, g.Locale, templateData)
	return sendMessageAsReply(-g.TelegramId, msg, replyToId)
}
