package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jmoiron/sqlx"
	"github.com/tidwall/gjson"
)

type User struct {
	Id         int    `db:"id"`
	TelegramId int    `db:"telegram_id"`
	Username   string `db:"username"`
	ChatId     int64  `db:"chat_id"`
}

const USERFIELDS = `
  id,
  coalesce(telegram_id, 0) AS telegram_id,
  coalesce(username, '') AS username,
  coalesce(chat_id, 0) AS chat_id
`

func loadUser(id int, telegramId int) (u User, err error) {
	err = pg.Get(&u, `
SELECT `+USERFIELDS+`
FROM telegram.account
WHERE id = $1 OR telegram_id = $2
    `, id, telegramId)
	return
}

func ensureUser(telegramId int, username string) (u User, tcase int, err error) {
	username = strings.ToLower(username)
	var vusername sql.NullString

	if username == "" {
		vusername.Valid = false
	} else {
		vusername.Scan(username)
	}

	var userRows []User
	err = pg.Select(&userRows, `
SELECT `+USERFIELDS+` FROM telegram.account
WHERE telegram_id = $1 OR username = $2
    `, telegramId, username)
	if err != nil && err != sql.ErrNoRows {
		return
	}

	tcase = len(userRows)
	switch tcase {
	case 0:
		// user not registered
		err = pg.Get(&u, `
INSERT INTO telegram.account (telegram_id, username)
VALUES ($1, $2)
RETURNING `+USERFIELDS,
			telegramId, vusername)
		return
	case 1:
		// user registered, update if necessary then leave
		u = userRows[0]
		if u.Username == username && u.TelegramId == telegramId {
			// all is well, just return
		} else if u.Username != username {
			// update username
			err = pg.Get(&u, `
UPDATE telegram.account SET username = $2 WHERE telegram_id = $1
RETURNING `+USERFIELDS,
				telegramId, vusername)
		} else if u.TelegramId != telegramId {
			// update telegram_id
			err = pg.Get(&u, `
UPDATE telegram.account SET telegram_id = $1 WHERE username = $2
RETURNING `+USERFIELDS,
				telegramId, username)
		}
		return
	case 2:
		// user has 2 accounts, one with the username, other with the telegram_id
		var txn *sqlx.Tx
		txn, err = pg.BeginTxx(context.TODO(),
			&sql.TxOptions{Isolation: sql.LevelSerializable})
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
			"DELETE FROM telegram.account WHERE id = $1",
			idToDelete)
		if err != nil {
			return
		}

		err = txn.Get(&u, `
UPDATE telegram.account
SET telegram_id = $2, username = $3
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

		return
	default:
		err = errors.New("odd error with more than 2 rows for the same user.")
		return
	}
}

func (u User) getTransaction(hash string) (txn Transaction, err error) {
	err = pg.Get(&txn, `
SELECT
  time,
  telegram_peer,
  status,
  trigger_message,
  coalesce(description, '') AS description,
  fees::float/1000 AS fees,
  amount::float/1000 AS amount,
  payment_hash,
  coalesce(preimage, '') AS preimage,
  payee_node,
  pending_bolt11
FROM lightning.account_txn
WHERE account_id = $1
  AND substring(payment_hash from 0 for $3) = $2
ORDER BY time
    `, u.Id, hash, len(hash)+1)
	if err != nil {
		return
	}

	txn.Description = escapeHTML(txn.Description)
	return
}

func (u User) AtName() string {
	if u.Username != "" {
		return "@" + u.Username
	}
	return fmt.Sprintf("user:%d", u.TelegramId)
}

func ensureTelegramId(telegram_id int) (u User, err error) {
	err = pg.Get(&u, `
INSERT INTO telegram.account (telegram_id)
VALUES ($1)
ON CONFLICT (telegram_id) DO UPDATE SET telegram_id = $1
RETURNING `+USERFIELDS,
		telegram_id)
	return
}

func ensureUsername(username string) (u User, err error) {
	err = pg.Get(&u, `
INSERT INTO telegram.account (username)
VALUES ($1)
ON CONFLICT (username) DO UPDATE SET username = $1
RETURNING `+USERFIELDS,
		strings.ToLower(username))
	return
}

func (u *User) setChat(id int64) error {
	u.ChatId = id
	_, err := pg.Exec(
		`UPDATE telegram.account SET chat_id = $1 WHERE id = $2`,
		id, u.Id)
	return err
}

func (u *User) unsetChat() {
	pg.Exec(`UPDATE telegram.account SET chat_id = NULL WHERE id = $1`, u.Id)
}

func (u User) notify(msg string) tgbotapi.Message {
	return u.notifyAsReply(msg, 0)
}

func (u User) notifyMarkdown(msg string) tgbotapi.Message {
	return notifyMarkdown(u.ChatId, msg)
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

func (u User) payInvoice(messageId int, bolt11 string, msatoshi int) (err error) {
	inv, err := ln.Call("decodepay", bolt11)
	if err != nil {
		return errors.New("Failed to decode invoice.")
	}

	bot.Send(tgbotapi.NewChatAction(u.ChatId, "Sending payment..."))
	amount := int(inv.Get("msatoshi").Int())
	desc := inv.Get("description").String()
	hash := inv.Get("payment_hash").String()
	payee := inv.Get("payee").String()
	params := map[string]interface{}{
		"bolt11":        bolt11,
		"riskfactor":    6,
		"maxfeepercent": 1,
		"exemptfee":     3,
		"retry_for":     30,
	}

	if amount == 0 {
		// amount is optional, so let's use the provided on the command
		amount = msatoshi
		params["msatoshi"] = msatoshi
	}
	if amount == 0 {
		// if nothing was provided, end here
		return errors.New("no amount provided")
	}

	txn, err := pg.BeginTxx(context.TODO(),
		&sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Debug().Err(err).Msg("database error")
		return errors.New("Database error.")
	}
	defer txn.Rollback()

	fakeLabel := fmt.Sprintf("%s.pay.%s", s.ServiceId, hash)

	var balance int
	_, err = txn.Exec(`
INSERT INTO lightning.transaction
  (from_id, amount, description, payment_hash, label, pending_bolt11, trigger_message, remote_node)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `, u.Id, amount, desc, hash, fakeLabel, bolt11, messageId, payee)
	if err != nil {
		log.Debug().Err(err).Msg("database error")
		return errors.New("Database error.")
	}

	err = txn.Get(&balance, `
SELECT balance::int FROM lightning.balance WHERE account_id = $1
    `, u.Id)
	if err != nil {
		log.Debug().Err(err).Msg("database error")
		return errors.New("Database error.")
	}

	if balance < 0 {
		return fmt.Errorf("Insufficient balance. Needs %.0f sat more.",
			-float64(balance)/1000)
	}

	err = txn.Commit()
	if err != nil {
		log.Debug().Err(err).Msg("database error")
		return errors.New("Database error.")
	}

	// actually send the lightning payment
	go func(u User, messageId int, params map[string]interface{}) {
		success, payment, err := ln.PayAndWaitUntilResolution(params)
		if err != nil {
			log.Warn().Err(err).Interface("p", params).Msg("Unexpected error paying invoice.")
			return
		}

		u.reactToPaymentStatus(success, messageId, payment)
	}(u, messageId, params)

	return nil
}

func (u User) reactToPaymentStatus(success bool, messageId int, payment gjson.Result) {
	if success {
		// if it succeeds we mark the transaction as not pending anymore
		// plus save fees and preimage
		msats := payment.Get("msatoshi").Float()
		fees := payment.Get("msatoshi_sent").Float() - msats
		preimage := payment.Get("payment_preimage").String()
		hash := payment.Get("payment_hash").String()

		_, err = pg.Exec(`
UPDATE lightning.transaction
SET fees = $1, preimage = $2, pending_bolt11 = null
WHERE payment_hash = $3
    `, fees, preimage, hash)
		if err != nil {
			log.Error().Err(err).
				Str("user", u.Username).
				Str("hash", hash).
				Float64("fees", fees).
				Msg("failed to update transaction fees.")
		}

		u.notifyAsReply(fmt.Sprintf(
			"Paid with %d sat (+ %.3f fee). \n\nHash: %s\n\nProof: %s\n\n/tx%s",
			int(msats/1000),
			fees/1000,
			hash,
			preimage,
			hash[:5],
		), messageId)
	} else {
		hash := payment.Get("payment_hash").String()
		log.Warn().
			Str("user", u.Username).
			Str("payment", payment.String()).
			Str("hash", hash).
			Msg("payment failed")
		u.notifyAsReply("Payment failed.", messageId)

		_, err := pg.Exec(
			`DELETE FROM lightning.transaction WHERE payment_hash = $1`, hash)
		if err != nil {
			log.Error().Err(err).Str("hash", hash).
				Msg("failed to cancel transaction after routing failure.")
		}
	}
}

func (u User) sendInternally(
	messageId int,
	target User,
	msats int,
	desc, label interface{},
) (string, error) {
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
  (from_id, to_id, amount, description, label, trigger_message)
VALUES ($1, $2, $3, $4, $5, $6)
    `, u.Id, target.Id, msats, vdesc, vlabel, messageId)
	if err != nil {
		return "Database error.", err
	}

	err = txn.Get(&balance, `
SELECT balance::int FROM lightning.balance WHERE account_id = $1
    `, u.Id)
	if err != nil {
		return "Database error.", err
	}

	if balance < 0 {
		return fmt.Sprintf("Insufficient balance. Needs %.0f sat more.",
				-float64(balance)/1000),
			errors.New("insufficient balance")
	}

	err = txn.Commit()
	if err != nil {
		return "Unable to pay due to internal database error.", err
	}

	return "", nil
}

func (u User) paymentReceived(
	amount int, desc, hash, preimage, label string,
) (err error) {
	_, err = pg.Exec(`
INSERT INTO lightning.transaction
  (to_id, amount, description, payment_hash, preimage, label)
VALUES ($1, $2, $3, $4, $5, $6)
    `, u.Id, amount, desc, hash, preimage, label)
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
    SELECT coalesce(sum(amount), 0)::float/1000 FROM lightning.transaction AS t
    WHERE b.account_id = t.to_id
  ) AS totalrecv,
  (
    SELECT coalesce(sum(amount), 0)::float/1000 FROM lightning.transaction AS t
    WHERE b.account_id = t.from_id
  ) AS totalsent,
  ( 
    SELECT coalesce(sum(fees), 0)::float/1000 FROM lightning.transaction AS t
    WHERE b.account_id = t.from_id
  ) AS fees
FROM lightning.balance AS b
WHERE b.account_id = $1
GROUP BY b.account_id, b.balance
    `, u.Id)
	return
}

func (u User) listTransactions(limit int, offset int) (txns []Transaction, err error) {
	err = pg.Select(&txns, `
SELECT * FROM (
  SELECT
    time,
    telegram_peer,
    status,
    CASE WHEN char_length(coalesce(description, '')) <= 16
      THEN coalesce(description, '')
      ELSE substring(coalesce(description, '') from 0 for 15) || '…'
    END AS description,
    amount::float/1000 AS amount,
    payment_hash
  FROM lightning.account_txn
  WHERE account_id = $1
  ORDER BY time DESC
  LIMIT $2
  OFFSET $3
) AS latest ORDER BY time ASC
    `, u.Id, limit, offset)
	if err != nil {
		return
	}

	for i := range txns {
		txns[i].Description = escapeHTML(txns[i].Description)
	}

	return
}

func (u User) checkBalanceFor(sats int, purpose string) bool {
	if info, err := u.getInfo(); err != nil || int(info.Balance) < sats {
		u.notify(fmt.Sprintf("Insufficient balance for %s. Needs %.0f sat more.",
			purpose, float64(sats)-info.Balance))
		return false
	}
	return true
}

type Info struct {
	AccountId     string  `db:"account_id"`
	Balance       float64 `db:"balance"`
	TotalSent     float64 `db:"totalsent"`
	TotalReceived float64 `db:"totalrecv"`
	TotalFees     float64 `db:"fees"`
}
