package main

import (
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

type User struct {
	Id       int    `db:"id"`
	Username string `db:"username"`
	ChatId   int64  `db:"chat_id"`
}

type Transaction struct {
}

func loadUser(id int) (u User, err error) {
	err = pg.Get(&u, `
SELECT id, username, chat_id
FROM account
WHERE id = $1
    `, id)
	return
}

func ensureUser(id int, username string) (u User, err error) {
	err = pg.Get(&u, `
INSERT INTO account (id, username)
VALUES ($1, $2)
ON CONFLICT (id) DO UPDATE SET username = $2
RETURNING *
    `, id, username)
	return
}

func (u User) setChat(id int64) error {
	_, err := pg.Exec(`UPDATE account SET chat_id = $1 WHERE id = $2`, id, u.Id)
	return err
}

func (u User) notify(msg string) {
	log.Debug().Str("user", u.Username).Str("msg", msg).Msg("notifying user")
	_, err := bot.Send(tgbotapi.NewMessage(u.ChatId, msg))
	if err != nil {
		log.Warn().Str("user", u.Username).Err(err).Msg("error sending message")
	}
}

func (u User) sendImage(image interface{}) {
	_, err := bot.Send(tgbotapi.NewPhotoUpload(u.ChatId, image))
	if err != nil {
		log.Warn().Str("user", u.Username).Err(err).Msg("error sending image")
	}
}

func (u User) payInvoice(invoice string) (err error) {
	// decoded, err := ln.Call("decodepay", lightning.Params{"bolt11": invoice})
	return nil
}
