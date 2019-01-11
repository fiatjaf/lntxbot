package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/tidwall/gjson"
)

type User struct {
	Id         int    `db:"id"`
	TelegramId int    `db:"telegram_id"`
	Username   string `db:"username"`
	ChatId     int64  `db:"chat_id"`
}

type Transaction struct {
}

func loadUser(id int, telegramId int) (u User, err error) {
	err = pg.Get(&u, `
SELECT id, telegram_id, username, chat_id
FROM telegram.account
WHERE id = $1 OR telegram_id = $2
    `, id, telegramId)
	return
}

func ensureUser(telegramId int, username string) (u User, err error) {
	err = pg.Get(&u, `
INSERT INTO telegram.account (telegram_id, username)
VALUES ($1, $2)
ON CONFLICT (telegram_id) DO UPDATE SET username = $2
RETURNING id, telegram_id, username
    `, telegramId, username)
	return
}

func (u *User) setChat(id int64) error {
	u.ChatId = id
	_, err := pg.Exec(
		`UPDATE telegram.account SET chat_id = $1 WHERE id = $2`,
		id, u.Id)
	return err
}

func (u User) notify(msg string) tgbotapi.Message {
	return u.notifyAsReply(msg, 0)
}

func (u User) notifyAsReply(msg string, replyToId int) tgbotapi.Message {
	log.Debug().Str("user", u.Username).Str("msg", msg).Msg("notifying user")
	chattable := tgbotapi.NewMessage(u.ChatId, msg)
	chattable.BaseChat.ReplyToMessageID = replyToId
	message, err := bot.Send(chattable)
	if err != nil {
		log.Warn().Str("user", u.Username).Err(err).Msg("error sending message")
	}
	return message
}

func (u User) payInvoice(
	bolt11, label string, msatoshi int,
) (res gjson.Result, errMsg string, err error) {
	inv, err := ln.Call("decodepay", bolt11)
	if err != nil {
		return res, "Failed to decode invoice.", err
	}

	log.Print("sending payment")
	bot.Send(tgbotapi.NewChatAction(u.ChatId, "Sending payment..."))
	amount := int(inv.Get("msatoshi").Int())
	desc := inv.Get("description").String()
	hash := inv.Get("payment_hash").String()

	if amount == 0 {
		// amount is optional, so let's use the provided on the command
		amount = msatoshi
	}
	if amount == 0 {
		// if nothing was provided, end here
		return res, "No amount provided.", errors.New("no amount provided")
	}

	txn, err := pg.BeginTxx(context.TODO(),
		&sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return res, "Database error.", err
	}
	defer txn.Rollback()

	var balance int
	_, err = txn.Exec(`
WITH newtx AS (
  INSERT INTO lightning.transaction
    (from_id, amount, description, payment_hash, label)
  VALUES ($1, $2, $3, $4, $5)
)
    `, u.Id, amount, desc, hash, label)
	if err != nil {
		return res, "Database error.", err
	}

	txn.Get(&balance, `
SELECT balance FROM lightning.balance WHERE account_id = $1
    `, u.Id)
	if err != nil {
		return res, "Database error.", err
	}

	if balance < 0 {
		return res,
			fmt.Sprintf("Insufficient balance. Needs %d more satoshis.",
				int(-balance/1000)),
			errors.New("insufficient balance")
	}

	err = txn.Commit()
	if err != nil {
		return res, "Unable to pay due to internal database error.", err
	}

	res, err = ln.Call("pay", bolt11, strconv.Itoa(amount))
	if err != nil {
		return res, "Routing failed.", err
	}

	// save fees and preimage
	fees := res.Get("msatoshi_sent").Float() - res.Get("msatoshi").Float()
	preimage := res.Get("payment_preimage")
	_, err = pg.Exec(`
UPDATE transaction
SET fees = $1, preimage = $2
WHERE label = $3
    `, fees, preimage, label)
	if err != nil {
		log.Error().Err(err).
			Str("user", u.Username).
			Str("label", label).
			Float64("fees", fees).
			Msg("failed to update transaction fees.")
	}

	return res, "", nil
}

func (u User) payInternally(
	target User, bolt11, label string, msatoshi int,
) (msats int, hash string, errMsg string, err error) {
	inv, err := ln.Call("decodepay", bolt11)
	if err != nil {
		return 0, "", "Failed to decode invoice.", err
	}

	log.Print("making internal payment")
	bot.Send(tgbotapi.NewChatAction(u.ChatId, "Sending payment..."))
	msats = int(inv.Get("msatoshi").Int())
	hash = inv.Get("payment_hash").String()
	desc := inv.Get("description").String()

	if msats == 0 {
		// amount is optional, so let's use the provided on the command
		msats = msatoshi
	}

	errMsg, err = u.sendInternally(target, msats, desc, hash, label)
	if err != nil {
		return 0, "", errMsg, err
	}

	return msats, hash, "", nil
}

func (u User) sendInternally(target User, msats int, desc, hash, label interface{}) (string, error) {
	if msats == 0 {
		// if nothing was provided, end here
		return "No amount provided.", errors.New("no amount provided")
	}

	var (
		vdesc  = &sql.NullString{}
		vhash  = &sql.NullString{}
		vlabel = &sql.NullString{}
	)

	vdesc.Scan(desc)
	vhash.Scan(hash)
	vlabel.Scan(label)

	txn, err := pg.BeginTxx(context.TODO(),
		&sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return "Database error.", err
	}
	defer txn.Rollback()

	var balance int
	_, err = txn.Exec(`
  INSERT INTO lightning.transaction
    (from_id, to_id, amount, description, payment_hash, label)
  VALUES ($1, $2, $3, $4, $5, $6)
    `, u.Id, target.Id, msats, vdesc, vhash, vlabel)
	if err != nil {
		return "Database error.", err
	}

	txn.Get(&balance, `
SELECT balance FROM lightning.balance WHERE account_id = $1
    `, u.Id)
	if err != nil {
		return "Database error.", err
	}

	log.Print("BALANCE", balance)
	if balance < 0 {
		return fmt.Sprintf("Insufficient balance. Needs %d more satoshis.",
				int(-balance/1000)),
			errors.New("insufficient balance")
	}

	err = txn.Commit()
	if err != nil {
		return "Unable to pay due to internal database error.", err
	}

	return "", nil
}

func (u User) paymentReceived(
	amount int, desc, hash, label string,
) (balance int, err error) {
	err = pg.Get(&balance, `
WITH newtx AS (
  INSERT INTO lightning.transaction
    (to_id, amount, description, payment_hash, label)
  VALUES ($1, $2, $3, $4, $5)
)
SELECT balance FROM lightning.balance WHERE account_id = $1
    `, u.Id, amount, desc, hash, label)
	if err != nil {
		log.Error().Err(err).
			Str("user", u.Username).Str("label", label).
			Msg("failed to save payment received.")
	}

	return
}
