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

func (u User) notify(msg string) tgbotapi.Message {
	log.Debug().Str("user", u.Username).Str("msg", msg).Msg("notifying user")
	chattable := tgbotapi.NewMessage(u.ChatId, msg)
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
		return res, "", err
	}
	defer txn.Rollback()

	var balance int
	err = txn.Get(&balance, `
WITH newtx AS (
  INSERT INTO transaction
    (account_id, amount, description, bolt11, payment_hash, label)
  VALUES ($1, $2, $3, $4, $5, $6)
)
SELECT coalesce(sum(amount), 0) - coalesce(sum(fees), 0)
FROM transaction
WHERE account_id = $1
    `, u.Id, -amount, desc, bolt11, hash, label)
	if err != nil {
		return res, "", err
	}

	if balance < 0 {
		return res,
			fmt.Sprintf("Insufficient balance. Needs %d more satoshis.",
				int(-balance/1000)),
			errors.New("insufficient balance")
	}

	res, err = ln.Call("pay", bolt11, strconv.Itoa(amount))
	if err != nil {
		return res, "Routing failed.", err
	}

	// save fees
	fees := res.Get("msatoshi_sent").Float() - res.Get("msatoshi").Float()
	_, err = pg.Exec(`
UPDATE transaction
SET fees = $1
WHERE label = $2
    `, fees, label)
	if err != nil {
		log.Error().Err(err).
			Str("user", u.Username).
			Str("label", label).
			Float64("fees", fees).
			Msg("failed to update transaction fees.")

		txn.Rollback()
		return res, "", err
	}

	err = txn.Commit()
	if err != nil {
		log.Error().Err(err).
			Str("user", u.Username).
			Str("bolt11", bolt11).
			Str("payment_hash", hash).
			Str("label", label).
			Int("amount", amount).
			Float64("fees", fees).
			Msg("failed to commit transaction")

		txn.Rollback()
		return res, "", err
	}

	return res, "", nil
}

func (u User) paymentReceived(
	amount int, desc, bolt11, hash, label string,
) (balance int, err error) {
	err = pg.Get(&balance, `
WITH newtx AS (
  INSERT INTO transaction
    (account_id, amount, description, bolt11, payment_hash, label)
  VALUES ($1, $2, $3, $4, $5, $6)
)
SELECT coalesce(sum(amount), 0) - coalesce(sum(fees), 0)
FROM transaction
WHERE account_id = $1
    `, u.Id, amount, desc, bolt11, hash, label)
	if err != nil {
		log.Error().Err(err).
			Str("user", u.Username).Str("label", label).
			Msg("failed to save payment received.")
	}

	return
}
