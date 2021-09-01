package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	cmap "github.com/orcaman/concurrent-map"
)

func interceptMessage(message *tgbotapi.Message) (proceed bool) {
	ctx := context.Background()

	// check if ticket payments is pending
	joinKey := fmt.Sprintf("%d:%d", message.From.ID, message.Chat.ID)
	if _, isPending := pendingApproval.Get(joinKey); isPending {
		log.Debug().Str("user", message.From.String()).Msg("user pending, can't speak")
		return false
	}

	// check expensiveness
	if sats := isExpensive(message.Chat.ID, message.Text); sats != 0 {
		// take money out of the poor guy who sent the message
		u, err := loadTelegramUser(message.From.ID)
		if err != nil {
			return false
		}
		if !u.checkBalanceFor(ctx, int64(sats)*1000, "expensive chat") {
			return false
		}

		owner, err := getChatOwner(message.Chat.ID)
		if err != nil {
			return true
		}

		if owner.Id == u.Id {
			return true
		}

		link := fmt.Sprintf("https://t.me/c/%s/%d",
			strconv.FormatInt(message.Chat.ID, 10)[4:], message.MessageID)

		err = u.sendInternally(ctx, owner, false, int64(sats)*1000, 0,
			fmt.Sprintf("Expensive %s.", link), "", "expensive")
		if err == nil {
			send(ctx, u, t.EXPENSIVENOTIFICATION, t.T{
				"Link":   link,
				"Price":  sats,
				"Sender": true,
			})

			send(ctx, owner, t.EXPENSIVENOTIFICATION, t.T{
				"Link":   link,
				"Price":  sats,
				"Sender": false,
			})

			return true
		}

		return false
	}

	return true
}

/*
   ALL GROUP CHAT TELEGRAM IDS ARE NEGATIVE
*/

type GroupChat struct {
	DiscordGuildId string `db:"discord_guild_id"`
	TelegramId     int64  `db:"telegram_id"`
	Locale         string `db:"locale"`
	Spammy         bool   `db:"spammy"`
	Ticket         int    `db:"ticket"`
}

const GROUPCHATFIELDS = "coalesce(discord_guild_id, '') AS discord_guild_id, coalesce(telegram_id, 0) AS telegram_id, locale, spammy, ticket"

func (g *GroupChat) String() string {
	if g == nil {
		return "null"
	}
	if g.TelegramId != 0 {
		return fmt.Sprintf("tg:%d", g.TelegramId)
	}
	if g.DiscordGuildId != "" {
		return fmt.Sprintf("dgg:%s", g.DiscordGuildId)
	}
	return "unknown"
}

func ensureTelegramGroup(telegramId int64, locale string) (g GroupChat, err error) {
	err = pg.Get(&g, `
INSERT INTO groupchat AS g (telegram_id, locale)
VALUES (
  $1,
  CASE WHEN $2 != '' THEN $2 ELSE 'en' END
)
ON CONFLICT (telegram_id)
  DO UPDATE
    SET locale = CASE WHEN $2 != '' THEN $2 ELSE g.locale END
  RETURNING `+GROUPCHATFIELDS+`
    `, telegramId, locale)
	return
}

func loadTelegramGroup(telegramId int64) (g GroupChat, err error) {
	err = pg.Get(&g,
		"SELECT "+GROUPCHATFIELDS+" FROM groupchat WHERE telegram_id = $1", telegramId)
	return
}

var spammy_cache = cmap.New()

func (g GroupChat) toggleSpammy() (spammy bool, err error) {
	err = pg.Get(&spammy, `
UPDATE groupchat AS g SET spammy = NOT g.spammy
WHERE telegram_id = $1
RETURNING spammy
    `, g.TelegramId)

	spammy_cache.Set(strconv.FormatInt(g.TelegramId, 10), spammy)
	return
}

func (g GroupChat) toggleCoinflips() (enabled bool, err error) {
	err = pg.Get(&enabled, `
UPDATE groupchat AS g SET coinflips = NOT g.coinflips
WHERE telegram_id = $1
RETURNING coinflips
    `, g.TelegramId)
	return
}

func (g GroupChat) areCoinflipsEnabled() (enabled bool) {
	err := pg.Get(&enabled,
		"SELECT coinflips FROM groupchat WHERE telegram_id = $1", g.TelegramId)
	if err != nil {
		return true
	}
	return
}

func (g GroupChat) setTicketPrice(sat int) (err error) {
	_, err = pg.Exec(`
UPDATE groupchat SET ticket = $2
WHERE telegram_id = $1
    `, g.TelegramId, sat)
	return
}

type expensiveness struct {
	Price        int    `db:"expensive_price"`
	Pattern      string `db:"expensive_pattern"`
	patternRegex *regexp.Regexp
}

var expensive_cache = cmap.New()

func (g GroupChat) setExpensive(sat int, pattern string) (err error) {
	_, err = pg.Exec(`
UPDATE groupchat SET expensive_price = $2, expensive_pattern = $3
WHERE telegram_id = $1
    `, g.TelegramId, sat, pattern)
	if err != nil {
		return err
	}

	regex, _ := regexp.Compile(pattern)
	expensive_cache.Set(strconv.FormatInt(g.TelegramId, 10), expensiveness{
		sat, pattern, regex,
	})
	return
}

func isExpensive(groupTelegramId int64, text string) (price int) {
	var expensive expensiveness
	if iexpensive, ok := expensive_cache.Get(strconv.FormatInt(groupTelegramId, 10)); ok {
		expensive = iexpensive.(expensiveness)
	} else {
		pg.Get(&expensive, `
SELECT expensive_price, expensive_pattern
FROM groupchat
WHERE telegram_id = $1
        `, groupTelegramId)

		regex, _ := regexp.Compile(expensive.Pattern)
		expensive.patternRegex = regex
		expensive_cache.Set(strconv.FormatInt(groupTelegramId, 10), expensive)
	}

	if expensive.Price == 0 {
		return 0
	}

	if expensive.patternRegex.MatchString(strings.ToLower(text)) {
		return expensive.Price
	}

	return 0
}

func (g GroupChat) setRenamePrice(sat int) (err error) {
	_, err = pg.Exec(`
UPDATE groupchat SET renamable = $2
WHERE telegram_id = $1
    `, g.TelegramId, sat)
	return
}

func (g GroupChat) getRenamePrice() (sat int) {
	pg.Get(&sat,
		"SELECT renamable FROM groupchat WHERE telegram_id = $1",
		g.TelegramId)
	return
}

func (g GroupChat) isSpammy() (spammy bool) {
	if spammy, ok := spammy_cache.Get(strconv.FormatInt(g.TelegramId, 10)); ok {
		return spammy.(bool)
	}

	err := pg.Get(&spammy,
		"SELECT spammy FROM groupchat WHERE telegram_id = $1", g.TelegramId)
	if err != nil {
		return false
	}

	spammy_cache.Set(strconv.FormatInt(g.TelegramId, 10), spammy)
	return
}

func setLanguage(chatId int64, lang string) (err error) {
	if _, languageAvailable := bundle.Translations[lang]; !languageAvailable {
		return errors.New("language not available.")
	}

	table := "account"
	field := "telegram_chat_id"
	id := chatId
	taint := ", manual_locale = true"
	if chatId < 0 {
		table = "groupchat"
		field = "telegram_id"
		taint = ""
	}

	_, err = pg.Exec("UPDATE "+table+" SET locale = $2"+taint+" WHERE "+field+" = $1", id, lang)
	return
}
