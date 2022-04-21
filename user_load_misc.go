package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jmoiron/sqlx"
	"github.com/msingleton/amplitude-go"
)

type User struct {
	Id             int    `db:"id"`
	Username       string `db:"username"`
	TelegramId     int64  `db:"telegram_id"`
	TelegramChatId int64  `db:"telegram_chat_id"`
	Password       string `db:"password"`
	Locale         string `db:"locale"`

	// this is here just to accomodate a special query made on bitclouds.go routine
	// it can be used to other similar things in the future
	// if no other better solution is found
	Extra string `db:"extra"`
}

const USERFIELDS = `
  id,
  coalesce(telegram_username, '') AS username,
  locale,
  password,
  coalesce(telegram_id, 0) AS telegram_id,
  coalesce(telegram_chat_id, 0) AS telegram_chat_id
`

func (u *User) String() string {
	if u == nil {
		return "null"
	}

	if u.Username != "" {
		return fmt.Sprintf("%s(%d)", u.Username, u.Id)
	} else {
		return fmt.Sprintf("(%d)", u.Id)
	}
}

func (u User) hasPrivateChat() bool {
	return u.TelegramChatId != 0
}

func loadUser(id int) (u User, err error) {
	err = pg.Get(&u, `
SELECT `+USERFIELDS+`
FROM account
WHERE id = $1
    `, id)
	return
}

func loadTelegramUsername(username string) (u User, err error) {
	err = pg.Get(&u, `
SELECT `+USERFIELDS+`
FROM account
WHERE telegram_username = $1
    `, username)
	return
}

func loadTelegramUser(telegramId int) (u User, err error) {
	err = pg.Get(&u, `
SELECT `+USERFIELDS+`
FROM account
WHERE telegram_id = $1
    `, telegramId)
	return
}

func ensureTelegramUser(message *tgbotapi.Message) (u User, tcase int, err error) {
	var username string
	var telegramId int64
	var locale = "en"

	switch isChannelOrGroupUser(message.From) {
	case true:
		telegramId = message.Chat.ID
		username = strings.ToLower(message.Chat.UserName)
	case false:
		telegramId = int64(message.From.ID)
		username = strings.ToLower(message.From.UserName)
		locale = message.From.LanguageCode
	}

	vusername := sql.NullString{String: username, Valid: username != ""}
	var userRows []User

	// always update locale while selecting user
	// unless it was set manually or isn't available
	err = pg.Select(&userRows, `
UPDATE account AS u
SET locale = CASE WHEN u.manual_locale OR $3 = '' THEN u.locale ELSE $3 END
WHERE u.telegram_id = $1 OR u.telegram_username = $2
RETURNING `+USERFIELDS,
		telegramId, username, locale)
	if err != nil && err != sql.ErrNoRows {
		return
	}

	tcase = len(userRows)
	switch tcase {
	case 0:
		// user not registered
		err = pg.Get(&u, `
INSERT INTO account (telegram_id, telegram_username)
VALUES ($1, $2)
RETURNING `+USERFIELDS,
			telegramId, vusername)
	case 1:
		// user registered, update if necessary then leave
		u = userRows[0]
		if u.Username == username && u.TelegramId == telegramId {
			// all is well, just return
		} else if u.Username != username {
			// update username
			err = pg.Get(&u, `
UPDATE account SET telegram_username = $2 WHERE telegram_id = $1
RETURNING `+USERFIELDS,
				telegramId, vusername)
		} else if u.TelegramId != telegramId {
			// update telegram_id
			err = pg.Get(&u, `
UPDATE account SET telegram_id = $1 WHERE telegram_username = $2
RETURNING `+USERFIELDS,
				telegramId, username)
		}
	case 2:
		// user has 2 accounts, one with the username, other with the telegram_id
		var txn *sqlx.Tx
		txn, err = pg.BeginTxx(context.Background(), &sql.TxOptions{})
		if err != nil {
			return
		}
		defer txn.Rollback()

		idToDelete := userRows[1].Id
		idToRemain := userRows[0].Id

		_, err = txn.Exec(
			"UPDATE lightning.transaction SET to_id = $1 WHERE to_id = $2",
			idToRemain, idToDelete)
		if err != nil {
			return
		}

		_, err = txn.Exec(
			"UPDATE lightning.transaction SET from_id = $1 WHERE from_id = $2",
			idToRemain, idToDelete)
		if err != nil {
			return
		}

		_, err = txn.Exec(
			"DELETE FROM account WHERE id = $1",
			idToDelete)
		if err != nil {
			return
		}

		err = txn.Get(&u, `
UPDATE account
SET telegram_id = $2, telegram_username = $3
WHERE id = $1
RETURNING `+USERFIELDS,
			idToRemain, telegramId, vusername)
		if err != nil {
			return
		}

		err = txn.Commit()
		if err != nil {
			return
		}
	default:
		err = errors.New("odd error with more than 2 rows for the same user.")
	}

	return
}

func ensureTelegramId(telegram_id int) (u User, err error) {
	err = pg.Get(&u, `
INSERT INTO account (telegram_id)
VALUES ($1)
ON CONFLICT (telegram_id) DO UPDATE SET telegram_id = $1
RETURNING `+USERFIELDS,
		telegram_id)
	return
}

func ensureTelegramUsername(username string) (u User, err error) {
	err = pg.Get(&u, `
INSERT INTO account (telegram_username)
VALUES ($1)
ON CONFLICT (telegram_username) DO UPDATE SET telegram_username = $1
RETURNING `+USERFIELDS,
		strings.ToLower(username))
	return
}

func (u *User) setChat(id int64) error {
	u.TelegramChatId = id
	_, err := pg.Exec(
		`UPDATE account SET telegram_chat_id = $1 WHERE id = $2`,
		id, u.Id)

	if err != nil {
		log.Warn().Err(err).Stringer("user", u).Int64("chat", id).
			Msg("failed to set chat id")
	}

	return err
}

func (u *User) unsetChat() {
	pg.Exec(`UPDATE account SET telegram_chat_id = NULL WHERE id = $1`, u.Id)
}

func (u User) updatePassword() (newpassword string, err error) {
	err = pg.Get(&newpassword, `
UPDATE account
SET password = DEFAULT WHERE id = $1
RETURNING password;                            
    `, u.Id)
	return
}

func (u User) saveBalanceCheckURL(service string, balanceCheckURL string) error {
	log := log.With().Str("service", service).
		Stringer("user", &u).Str("url", balanceCheckURL).
		Logger()

	if b, err := url.Parse(balanceCheckURL); err == nil && b.Host == service {
		_, err := pg.Exec(`
INSERT INTO balance_check (service, account, url)
VALUES ($1, $2, $3)
ON CONFLICT (service, account) DO UPDATE SET url = $3
        `, service, u.Id, balanceCheckURL)
		if err == nil {
			log.Info().Msg("saved balance_check")
		} else {
			log.Error().Err(err).Msg("failed to save balance_check")
		}
		return err
	} else {
		_, err := pg.Exec(`
DELETE FROM balance_check
WHERE service = $1 AND account = $2
         `, service, u.Id)
		if err == nil {
			log.Info().Msg("deleted balance_check")
		} else {
			log.Error().Err(err).Msg("failed to delete balance_check")
		}
		return err
	}
}

func (u User) loadBalanceCheckURL(service string) (rawurl string, err error) {
	err = pg.Get(&rawurl, `
SELECT url
FROM balance_check
WHERE account = $1 AND service = $2
    `, u.Id, service)
	if err == sql.ErrNoRows {
		err = nil
	}
	return
}

func (u User) track(event string, eventProperties map[string]interface{}) {
	amp.Event(amplitude.Event{
		UserId:          strconv.Itoa(u.Id),
		EventType:       event,
		EventProperties: eventProperties,
	})
}

func (u User) AtName(ctx context.Context) string {
	if u.Username != "" {
		return "@" + u.Username
	}
	return fmt.Sprintf(`<a href="tg://user?id=%d">user %d</a>`,
		u.TelegramId, u.TelegramId)
}
