package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jmoiron/sqlx"
	qrcode "github.com/skip2/go-qrcode"
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
  anonymous,
  status,
  trigger_message,
  coalesce(description, '') AS description,
  fees::float/1000 AS fees,
  amount::float/1000 AS amount,
  payment_hash,
  coalesce(preimage, '') AS preimage,
  payee_node
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

func (u User) makeInvoice(
	sats int,
	desc string,
	label string,
	expiry *time.Duration,
	messageId interface{},
	preimage string,
) (bolt11 string, hash string, qrpath string, err error) {
	log.Debug().Str("user", u.Username).Str("desc", desc).Int("sats", sats).
		Msg("generating invoice")

	if preimage == "" {
		preimage, err = randomPreimage()
		if err != nil {
			return
		}
	}

	if label == "" {
		label = makeLabel(u.Id, messageId, preimage)
	}

	var msatoshi interface{}
	if sats == INVOICE_UNDEFINED_AMOUNT {
		msatoshi = "any"
	} else {
		msatoshi = sats * 1000
	}

	var exp time.Duration
	if expiry == nil {
		exp = s.InvoiceTimeout / time.Second
	} else {
		exp = *expiry / time.Second
	}

	// make invoice
	res, err := ln.CallWithCustomTimeout(time.Second*40, "invoice", map[string]interface{}{
		"msatoshi":    msatoshi,
		"label":       label,
		"description": desc + " [" + s.ServiceId + "/" + u.AtName() + "]",
		"expiry":      int(exp),
		"preimage":    preimage,
	})
	if err != nil {
		return
	}

	bolt11 = res.Get("bolt11").String()
	hash = res.Get("payment_hash").String()

	err = qrcode.WriteFile(strings.ToUpper(bolt11), qrcode.Medium, 256, qrImagePath(label))
	if err != nil {
		log.Warn().Err(err).Str("invoice", bolt11).
			Msg("failed to generate qr.")
		err = nil
	} else {
		qrpath = qrImagePath(label)
	}

	return bolt11, hash, qrpath, nil
}

func (u User) payInvoice(messageId int, bolt11 string, msatoshi int) (err error) {
	inv, err := ln.Call("decodepay", bolt11)
	if err != nil {
		return errors.New("Failed to decode invoice.")
	}

	bot.Send(tgbotapi.NewChatAction(u.ChatId, "Sending payment..."))
	payee := inv.Get("payee").String()
	amount := int(inv.Get("msatoshi").Int())
	desc := inv.Get("description").String()
	hash := inv.Get("payment_hash").String()
	params := map[string]interface{}{
		"riskfactor":    3,
		"maxfeepercent": 1,
		"exemptfee":     3,
		"label":         fmt.Sprintf("user=%d", u.Id),
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
		log.Debug().Err(err).Msg("database error starting transaction")
		return errors.New("Database error.")
	}
	defer txn.Rollback()

	fakeLabel := fmt.Sprintf("%s.pay.%s", s.ServiceId, hash)

	var balance int
	_, err = txn.Exec(`
INSERT INTO lightning.transaction
  (from_id, amount, description, payment_hash, label, pending, trigger_message, remote_node)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `, u.Id, amount, desc, hash, fakeLabel, true, messageId, payee)
	if err != nil {
		log.Debug().Err(err).Msg("database error inserting transaction")
		return errors.New("Payment already in course.")
	}

	err = txn.Get(&balance, `
SELECT balance::int FROM lightning.balance WHERE account_id = $1
    `, u.Id)
	if err != nil {
		log.Debug().Err(err).Msg("database error fetching balance")
		return errors.New("Database error.")
	}

	if balance < 0 {
		return fmt.Errorf("Insufficient balance. Needs %.0f sat more.",
			-float64(balance)/1000)
	}

	err = txn.Commit()
	if err != nil {
		log.Debug().Err(err).Msg("database error committing transaction")
		return errors.New("Database error.")
	}

	if payee == s.NodeId {
		// it's an internal invoice. mark as paid internally.
		var txn Transaction
		if err := pg.Get(&txn, `
SELECT payment_hash, preimage, label, amount
FROM lightning.transaction
WHERE payment_hash = $1
  AND label != $2`,
			hash, fakeLabel,
		); err != nil {
			// if it's generated here but is not in the database maybe it's a ticket invoice
			if err == sql.ErrNoRows && strings.HasPrefix(desc, "Ticket for") {
				for label, kickdata := range pendingApproval {
					if kickdata.Hash == hash {
						ticketPaid(label, kickdata)
						handleInvoicePaid(
							-1,
							int64(amount),
							desc,
							hash,
							label,
						)
						u.paymentHasSucceeded(messageId, float64(amount), float64(amount), "", hash)
						break
					}
				}
			} else {
				return err
			}
		} else {
			invpaid, err := ln.Call("listinvoices", txn.Label.String)
			if err != nil {
				return err
			}
			handleInvoicePaid(
				invpaid.Get("pay_index").Int(),
				invpaid.Get("msatoshi_received").Int(),
				invpaid.Get("description").String(),
				invpaid.Get("payment_hash").String(),
				invpaid.Get("label").String(),
			)
			u.paymentHasSucceeded(messageId, txn.Amount, txn.Amount, txn.Preimage.String, txn.Hash)
		}

		ln.Call("delinvoice", txn.Label.String, "unpaid")
	} else {
		// it's an invoice from elsewhere, continue and
		// actually send the lightning payment
		go func(u User, messageId int, params map[string]interface{}) {
			success, payment, tries, err := ln.PayAndWaitUntilResolution(bolt11, params)

			// save payment attempts for future counsultation
			// only save the latest 10 tries for brevity
			from := len(tries) - 10
			if from < 0 {
				from = 0
			}
			if jsontries, err := json.Marshal(tries[from:]); err == nil {
				rds.Set("tries:"+hash[:5], jsontries, time.Hour*24)
			}

			if err != nil {
				log.Warn().Err(err).
					Interface("params", params).
					Interface("tries", tries).
					Msg("Unexpected error paying invoice.")
				return
			}

			if success {
				u.paymentHasSucceeded(messageId,
					payment.Get("msatoshi").Float(),
					payment.Get("msatoshi_sent").Float(),
					payment.Get("payment_preimage").String(),
					payment.Get("payment_hash").String(),
				)
			} else {
				log.Warn().
					Str("user", u.Username).
					Int("user-id", u.Id).
					Interface("params", params).
					Interface("tries", tries).
					Str("payment", payment.String()).
					Str("hash", hash).
					Msg("payment failed")

				u.paymentHasFailed(messageId, payment.Get("payment_hash").String())
			}
		}(u, messageId, params)
	}

	return nil
}

func (u User) paymentHasSucceeded(messageId int,
	msatoshi float64,
	msatoshi_sent float64,
	preimage string,
	hash string,
) {
	// if it succeeds we mark the transaction as not pending anymore
	// plus save fees and preimage
	fees := msatoshi_sent - msatoshi

	_, err = pg.Exec(`
UPDATE lightning.transaction
SET fees = $1, preimage = $2, pending = false
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
		"Paid with <b>%d sat</b> (+ %.3f fee). \n\n<b>Hash:</b> %s\n\n<b>Proof:</b> %s\n\n/tx%s",
		int(msatoshi/1000),
		fees/1000,
		hash,
		preimage,
		hash[:5],
	), messageId)
}

func (u User) paymentHasFailed(messageId int, hash string) {
	u.notifyAsReply(fmt.Sprintf("Payment failed. /log%s", hash[:5]), messageId)

	_, err := pg.Exec(
		`DELETE FROM lightning.transaction WHERE payment_hash = $1`, hash)
	if err != nil {
		log.Error().Err(err).Str("hash", hash).
			Msg("failed to cancel transaction after routing failure.")
	}
}

func (u User) sendInternally(
	messageId int,
	target User,
	anonymous bool,
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

	var balance int64
	_, err = txn.Exec(`
INSERT INTO lightning.transaction
  (from_id, to_id, anonymous, amount, description, label, trigger_message)
VALUES ($1, $2, $3, $4, $5, $6, $7)
    `, u.Id, target.Id, anonymous, msats, vdesc, vlabel, messageId)
	if err != nil {
		return "Database error.", err
	}

	err = txn.Get(&balance, `
SELECT balance::numeric(13) FROM lightning.balance WHERE account_id = $1
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
	amount int,
	desc, hash, preimage, label string,
) (err error) {
	_, err = pg.Exec(`
INSERT INTO lightning.transaction
  (to_id, amount, description, payment_hash, preimage, label)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (payment_hash) DO UPDATE SET to_id = $1
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

type InOut int

const (
	In InOut = iota
	Out
	Both
)

func (u User) listTransactions(limit, offset, descCharLimit int, inOrOut InOut) (txns []Transaction, err error) {
	filterBy := func(inOrOut InOut) string {
		switch inOrOut {
		case In:
			return " AND amount > 0 "
		case Out:
			return " AND amount < 0 "
		case Both:
			return ""
		}
		return ""
	}

	err = pg.Select(&txns, `
SELECT * FROM (
  SELECT
    time,
    telegram_peer,
    anonymous,
    status,
    CASE WHEN char_length(coalesce(description, '')) <= $4
      THEN coalesce(description, '')
      ELSE substring(coalesce(description, '') from 0 for ($4 - 1)) || '…'
    END AS description,
    amount::float/1000 AS amount,
    payment_hash,
    preimage
  FROM lightning.account_txn
  WHERE account_id = $1 `+filterBy(inOrOut)+`
  ORDER BY time DESC
  LIMIT $2
  OFFSET $3
) AS latest ORDER BY time ASC
    `, u.Id, limit, offset, descCharLimit)
	if err != nil {
		return
	}

	for i := range txns {
		txns[i].Description = escapeHTML(txns[i].Description)
	}

	return
}

func (u User) checkBalanceFor(sats int, purpose string) bool {
	if sats < 40 {
		u.notify("That's too small, please start your " + purpose + " with at least 40 sat.")
		return false
	}

	if info, err := u.getInfo(); err != nil || int(info.Balance) < sats {
		u.notify(fmt.Sprintf("Insufficient balance for %s. Needs %.0f sat more.",
			purpose, float64(sats)-info.Balance))
		return false
	}
	return true
}

func fromManyToOne(sats int, toId int, fromIds []int,
	desc, receiverMessage, giverMessage string,
) (receiver User, err error) {
	txn, err := pg.BeginTxx(context.TODO(),
		&sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return
	}
	defer txn.Rollback()

	receiver, _ = loadUser(toId, 0)
	giverNames := make([]string, 0, len(fromIds))

	msats := sats * 1000
	var (
		vdesc  = &sql.NullString{}
		vlabel = &sql.NullString{}
	)
	vdesc.Scan(desc)

	for _, fromId := range fromIds {
		if fromId == toId {
			continue
		}

		_, err = txn.Exec(`
INSERT INTO lightning.transaction
  (from_id, to_id, amount, description, label)
VALUES ($1, $2, $3, $4, $5)
    `, fromId, toId, msats, vdesc, vlabel)
		if err != nil {
			return
		}

		var balance int
		err = txn.Get(&balance, `
SELECT balance::int FROM lightning.balance WHERE account_id = $1
    `, fromId)
		if err != nil {
			return
		}

		if balance < 0 {
			err = errors.New("insufficient balance")
			return
		}

		giver, _ := loadUser(fromId, 0)
		giverNames = append(giverNames, giver.AtName())

		giver.notify(fmt.Sprintf(giverMessage, sats, receiver.AtName()))
	}

	err = txn.Commit()
	if err != nil {
		return
	}

	receiver.notify(
		fmt.Sprintf(receiverMessage,
			sats*len(fromIds), strings.Join(giverNames, " ")),
	)
	return
}

type Info struct {
	AccountId     string  `db:"account_id"`
	Balance       float64 `db:"balance"`
	TotalSent     float64 `db:"totalsent"`
	TotalReceived float64 `db:"totalrecv"`
	TotalFees     float64 `db:"fees"`
}
