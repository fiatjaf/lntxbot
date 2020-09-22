package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/msingleton/amplitude-go"
)

type User struct {
	Id               int    `db:"id"`
	Username         string `db:"username"`
	TelegramId       int    `db:"telegram_id"`
	DiscordId        string `db:"discord_id"`
	TelegramChatId   int64  `db:"telegram_chat_id"`
	DiscordChannelId string `db:"discord_channel_id"`
	Password         string `db:"password"`
	Locale           string `db:"locale"`

	// this is here just to accomodate a special query made on bitclouds.go routine
	// it can be used to other similar things in the future
	// if no other better solution is found
	Extra string `db:"extra"`

	ctx context.Context
}

const USERFIELDS = `
  id,
  coalesce(telegram_id, 0) AS telegram_id,
  coalesce(telegram_username, coalesce(discord_username, '')) AS username,
  coalesce(telegram_chat_id, 0) AS telegram_chat_id,
  coalesce(discord_channel_id, '') AS discord_channel_id,
  password,
  locale
`

func loadUser(id int) (u User, err error) {
	err = pg.Get(&u, `
SELECT `+USERFIELDS+`
FROM account
WHERE id = $1
    `, id)
	u.ctx = context.Background()
	return
}

func loadTelegramUser(telegramId int) (u User, err error) {
	err = pg.Get(&u, `
SELECT `+USERFIELDS+`
FROM account
WHERE telegram_id = $1
    `, telegramId)
	u.ctx = context.WithValue(context.Background(), "origin", "telegram")
	return
}

func loadDiscordUser(discordId string) (u User, err error) {
	err = pg.Get(&u, `
SELECT `+USERFIELDS+`
FROM account
WHERE discord_id = $1
    `, discordId)
	u.ctx = context.WithValue(context.Background(), "origin", "discord")
	return
}

func ensureTelegramUser(telegramId int, username string, locale string) (u User, tcase int, err error) {
	username = strings.ToLower(username)
	var vusername sql.NullString

	if username == "" {
		vusername.Valid = false
	} else {
		vusername.Scan(username)
	}

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
		txn, err = pg.Beginx()
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

	u.ctx = context.WithValue(context.Background(), "origin", "telegram")
	return
}

func ensureDiscordUser(discordId, username, locale string) (u User, tcase int, err error) {
	username = strings.ToLower(username)
	var vusername sql.NullString

	if username == "" {
		vusername.Valid = false
	} else {
		vusername.Scan(username)
	}

	var userRows []User

	// always update locale while selecting user unless it was set manually or isn't available
	err = pg.Select(&userRows, `
UPDATE account AS u
SET locale = CASE WHEN u.manual_locale OR $3 = '' THEN u.locale ELSE $3 END
WHERE u.discord_id = $1 OR u.discord_username = $2
RETURNING `+USERFIELDS,
		discordId, username, locale)
	if err != nil && err != sql.ErrNoRows {
		return
	}

	tcase = len(userRows)
	switch tcase {
	case 0:
		// user not registered
		err = pg.Get(&u, `
INSERT INTO account (discord_id, discord_username)
VALUES ($1, $2)
RETURNING `+USERFIELDS,
			discordId, vusername)
	case 1:
		// user registered, update if necessary then leave
		u = userRows[0]
		if u.Username == username && u.DiscordId == discordId {
			// all is well, just return
		} else if u.Username != username {
			// update username
			err = pg.Get(&u, `
UPDATE account SET discord_username = $2 WHERE discord_id = $1
RETURNING `+USERFIELDS,
				discordId, vusername)
		} else if u.DiscordId != discordId {
			// update discord_id
			err = pg.Get(&u, `
UPDATE account SET discord_id = $1 WHERE discord_username = $2
RETURNING `+USERFIELDS,
				discordId, username)
		}
	case 2:
		// user has 2 accounts, one with the username, other with the telegram_id
		var txn *sqlx.Tx
		txn, err = pg.Beginx()
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
SET discord_id = $2, discord_username = $3
WHERE id = $1
RETURNING `+USERFIELDS,
			idToRemain, discordId, vusername)
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

	u.ctx = context.WithValue(context.Background(), "origin", "discord")
	return
}

func ensureTelegramId(telegram_id int) (u User, err error) {
	err = pg.Get(&u, `
INSERT INTO account (telegram_id)
VALUES ($1)
ON CONFLICT (telegram_id) DO UPDATE SET telegram_id = $1
RETURNING `+USERFIELDS,
		telegram_id)
	u.ctx = context.WithValue(context.Background(), "origin", "telegram")
	return
}

func ensureTelegramUsername(username string) (u User, err error) {
	err = pg.Get(&u, `
INSERT INTO account (telegram_username)
VALUES ($1)
ON CONFLICT (telegram_username) DO UPDATE SET telegram_username = $1
RETURNING `+USERFIELDS,
		strings.ToLower(username))
	u.ctx = context.WithValue(context.Background(), "origin", "telegram")
	return
}

func (u *User) setChat(id int64) error {
	u.TelegramChatId = id
	_, err := pg.Exec(
		`UPDATE account SET telegram_chat_id = $1 WHERE id = $2`,
		id, u.Id)
	return err
}

func (u *User) unsetChat() {
	pg.Exec(`UPDATE account SET telegram_chat_id = NULL WHERE id = $1`, u.Id)
}

func (u *User) setChannel(id string) error {
	u.DiscordChannelId = id
	_, err := pg.Exec(
		`UPDATE account SET discord_channel_id = $1 WHERE id = $2`,
		id, u.Id)
	return err
}

func (u *User) unsetChannel() {
	pg.Exec(`UPDATE account SET discord_channel_id = NULL WHERE id = $1`, u.Id)
}

func (u User) updatePassword() (newpassword string, err error) {
	err = pg.Get(&newpassword, `
UPDATE account
SET password = DEFAULT WHERE id = $1
RETURNING password;                            
    `, u.Id)
	return
}

func (u User) track(event string, eventProperties map[string]interface{}) {
	amp.Event(amplitude.Event{
		UserId:          strconv.Itoa(u.Id),
		EventType:       event,
		EventProperties: eventProperties,
	})
}

func (u User) AtName() string {
	if u.isTelegram() {
		if u.Username != "" {
			return "@" + u.Username
		}
		return fmt.Sprintf("user:%d", u.TelegramId)
	} else if u.IsDiscord() {
		return fmt.Sprintf("<@!%s>", u.DiscordId)
	} else {
		return "<unknown_user?err>"
	}
}

func (u User) origin() string {
	val := u.ctx.Value("origin")
	if val == nil {
		return ""
	}
	return val.(string)
}

func (u User) isTelegram() bool {
	origin := u.origin()
	if origin == "" {
		return u.TelegramChatId != 0
	} else {
		return origin == "telegram"
	}
}

func (u User) isDiscord() bool {
	origin := u.origin()
	if origin == "" {
		return u.DiscordChannelId != ""
	} else {
		return origin == "discord"
	}
}
