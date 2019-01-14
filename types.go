package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/tidwall/gjson"
)

type User struct {
	Id         int    `db:"id"`
	TelegramId int    `db:"telegram_id"`
	Username   string `db:"username"`
	ChatId     int64  `db:"chat_id"`
}

func loadUser(id int, telegramId int) (u User, err error) {
	err = pg.Get(&u, `
SELECT id, telegram_id, username, coalesce(chat_id, 0) AS chat_id
FROM telegram.account
WHERE id = $1 OR telegram_id = $2
    `, id, telegramId)
	return
}

func ensureUser(telegramId int, username string) (u User, err error) {
	err = pg.Get(&u, `
INSERT INTO telegram.account (telegram_id, username)
VALUES ($1, $2)
RETURNING id, telegram_id, username, coalesce(chat_id, 0) AS chat_id
    `, telegramId, strings.ToLower(username))
	if err == nil {
		return
	}

	err = pg.Get(&u, `
UPDATE telegram.account
SET telegram_id = $1, username = $2
WHERE username = $2 OR telegram_id = $1
RETURNING id, telegram_id, username, coalesce(chat_id, 0) AS chat_id
    `, telegramId, strings.ToLower(username))
	return
}

func ensureTelegramId(telegram_id int) (u User, err error) {
	err = pg.Get(&u, `
INSERT INTO telegram.account (telegram_id)
VALUES ($1)
ON CONFLICT (telegram_id) DO UPDATE SET telegram_id = $1
RETURNING
   id,
   telegram_id,
   coalesce(username, '') AS username,
   coalesce(chat_id, 0) AS chat_id
    `, telegram_id)
	return
}

func ensureUsername(username string) (u User, err error) {
	log.Print(username)
	err = pg.Get(&u, `
INSERT INTO telegram.account (username)
VALUES ($1)
ON CONFLICT (username) DO UPDATE SET username = $1
RETURNING
   id,
   coalesce(telegram_id, 0) AS telegram_id,
   username,
   coalesce(chat_id, 0) AS chat_id
    `, strings.ToLower(username))
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
	if u.ChatId == 0 {
		log.Info().Str("user", u.Username).Str("msg", msg).
			Msg("can't notify user as it hasn't started a chat with the bot.")
		return tgbotapi.Message{}
	}
	log.Debug().Str("user", u.Username).Str("msg", msg).Msg("notifying user")
	return notifyAsReply(u.ChatId, msg, replyToId)
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
INSERT INTO lightning.transaction
  (from_id, amount, description, payment_hash, label)
VALUES ($1, $2, $3, $4, $5)
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

	res, err = ln.CallWithCustomTimeout("pay", time.Second*20, bolt11, strconv.Itoa(amount))
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

	errMsg, err = u.sendInternally(target, msats, desc, label)
	if err != nil {
		return 0, "", errMsg, err
	}

	return msats, hash, "", nil
}

func (u User) sendInternally(target User, msats int, desc, label interface{}) (string, error) {
	if target.Id == u.Id || target.Username == u.Username || target.TelegramId == u.TelegramId {
		return "Can't pay yourself.", errors.New("user trying to pay itself")
	}

	if msats == 0 {
		// if nothing was provided, end here
		return "No amount provided.", errors.New("no amount provided")
	}

	var (
		vdesc  = &sql.NullString{}
		vlabel = &sql.NullString{}
	)

	vdesc.Scan(desc)
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
  (from_id, to_id, amount, description, label)
VALUES ($1, $2, $3, $4, $5)
    `, u.Id, target.Id, msats, vdesc, vlabel)
	if err != nil {
		return "Database error.", err
	}

	txn.Get(&balance, `
SELECT balance FROM lightning.balance WHERE account_id = $1
    `, u.Id)
	if err != nil {
		return "Database error.", err
	}

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
) (err error) {
	_, err = pg.Exec(`
INSERT INTO lightning.transaction
  (to_id, amount, description, payment_hash, label)
VALUES ($1, $2, $3, $4, $5)
    `, u.Id, amount, desc, hash, label)
	if err != nil {
		log.Error().Err(err).
			Str("user", u.Username).Str("label", label).
			Msg("failed to save payment received.")
	}

	return
}

func (u User) getInfo() (info Info, err error) {
	err = pg.Get(&info, `
SELECT
  b.account_id,
  b.balance/1000 AS balance,
  (
    SELECT coalesce(sum(amount), 0)/1000 FROM lightning.transaction AS t
    WHERE b.account_id = t.to_id
  ) AS totalrecv,
  (
    SELECT coalesce(sum(amount), 0)/1000 FROM lightning.transaction AS t
    WHERE b.account_id = t.from_id
  ) AS totalsent,
  ( 
    SELECT coalesce(sum(fees), 0)/1000 FROM lightning.transaction AS t
    WHERE b.account_id = t.from_id
  ) AS fees
FROM lightning.balance AS b
WHERE b.account_id = $1
GROUP BY b.account_id, b.balance
    `, u.Id)
	return
}

func (u User) listTransactions() (txns []Transaction, err error) {
	err = pg.Select(&txns, `
SELECT time, telegram_peer, status, amount/1000 AS amount, payment_hash
FROM lightning.account_txn
WHERE account_id = $1
ORDER BY time
    `, u.Id)
	return
}

func (u User) getTransaction(hash string) (txn Transaction, err error) {
	hashsize := len(hash)
	err = pg.Get(&txn, `
SELECT
  time,
  telegram_peer,
  status,
  fees/1000 AS fees,
  amount/1000 AS amount,
  payment_hash,
  coalesce(preimage, '') AS preimage
FROM lightning.account_txn
WHERE account_id = $1 AND substring(payment_hash from 0 for $3) = $2
ORDER BY time
    `, u.Id, hash, hashsize+1)
	return
}

type Transaction struct {
	Time         time.Time `db:"time"`
	Status       string    `db:"status"`
	TelegramPeer *string   `db:"telegram_peer"`
	Amount       float64   `db:"amount"`
	Fees         float64   `db:"fees"`
	Hash         string    `db:"payment_hash"`
	Preimage     string    `db:"preimage"`
	Description  string    `db:"description"`
}

func (t Transaction) PeerActionDescription() string {
	if t.Status == "RECEIVED" {
		return "from @" + *t.TelegramPeer
	} else {
		return "to @" + *t.TelegramPeer
	}
}

func (t Transaction) PaddedStatus() string {
	return fmt.Sprintf("%-8s", t.Status)
}

func (t Transaction) HasPreimage() bool {
	return t.Preimage != ""
}

func (t Transaction) TimeFormat() string {
	return t.Time.Format("2 Jan 2006 3:04PM")
}

func (t Transaction) TimeFormatSmall() string {
	return t.Time.Format("2 Jan 3:04PM")
}

func (t Transaction) Satoshis() string {
	return fmt.Sprintf("%.3f", t.Amount)
}

func (t Transaction) PaddedSatoshis() string {
	return fmt.Sprintf("%8.1f", t.Amount)
}

func (t Transaction) FeeSatoshis() string {
	return fmt.Sprintf("%.3f", t.Fees)
}

func (t Transaction) HashReduced() string {
	return t.Hash[:5]
}

type Info struct {
	AccountId     string  `db:"account_id"`
	Balance       float64 `db:"balance"`
	TotalSent     float64 `db:"totalsent"`
	TotalReceived float64 `db:"totalrecv"`
	TotalFees     float64 `db:"fees"`
}
