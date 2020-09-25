package main

import (
	"errors"
	"fmt"
	"strconv"

	cmap "github.com/orcaman/concurrent-map"
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

func (g *GroupChat) String() string {
	if g == nil {
		return "null"
	}
	return fmt.Sprintf("%d", g.TelegramId)
}

func ensureGroup(telegramId int64, locale string) (g GroupChat, err error) {
	err = pg.Get(&g, `
INSERT INTO telegram.chat AS g (telegram_id, locale)
VALUES (
  $1,
  CASE WHEN $2 != '' THEN $2 ELSE 'en' END
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

var spammy_cache = cmap.New()

func toggleSpammy(telegramId int64) (spammy bool, err error) {
	err = pg.Get(&spammy, `
UPDATE telegram.chat AS g SET spammy = NOT g.spammy
WHERE telegram_id = $1
RETURNING spammy
    `, -telegramId)

	spammy_cache.Set(strconv.FormatInt(-telegramId, 10), spammy)

	return
}

func isSpammy(telegramId int64) (spammy bool) {
	if spammy, ok := spammy_cache.Get(strconv.FormatInt(-telegramId, 10)); ok {
		return spammy.(bool)
	}

	err := pg.Get(&spammy, "SELECT spammy FROM telegram.chat WHERE telegram_id = $1", -telegramId)
	if err != nil {
		return false
	}

	spammy_cache.Set(strconv.FormatInt(-telegramId, 10), spammy)
	return
}

func toggleCoinflips(telegramId int64) (enabled bool, err error) {
	err = pg.Get(&enabled, `
UPDATE telegram.chat AS g SET coinflips = NOT g.coinflips
WHERE telegram_id = $1
RETURNING coinflips
    `, -telegramId)
	return
}

func areCoinflipsEnabled(telegramId int64) (enabled bool) {
	err := pg.Get(&enabled, "SELECT coinflips FROM telegram.chat WHERE telegram_id = $1", -telegramId)
	if err != nil {
		return true
	}
	return
}

func setTicketPrice(telegramId int64, sat int) (err error) {
	_, err = pg.Exec(`
UPDATE telegram.chat SET ticket = $2
WHERE telegram_id = $1
    `, -telegramId, sat)
	return
}

func setRenamablePrice(telegramId int64, sat int) (err error) {
	_, err = pg.Exec(`
UPDATE telegram.chat SET renamable = $2
WHERE telegram_id = $1
    `, -telegramId, sat)
	return
}

func setLanguage(chatId int64, lang string) (err error) {
	if _, languageAvailable := bundle.Translations[lang]; !languageAvailable {
		return errors.New("language not available.")
	}

	table := "account"
	field := "chat_id"
	id := chatId
	taint := ", manual_locale = true"
	if chatId < 0 {
		table = "telegram.chat"
		field = "telegram_id"
		id = -chatId
		taint = ""
	}

	_, err = pg.Exec("UPDATE "+table+" SET locale = $2"+taint+" WHERE "+field+" = $1", id, lang)
	return
}