package main

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	lightning "github.com/fiatjaf/lightningd-gjson-rpc"
	decodepay_gjson "github.com/fiatjaf/ln-decodepay/gjson"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/types"
	"github.com/msingleton/amplitude-go"
	"github.com/skip2/go-qrcode"
	"github.com/tidwall/gjson"
)

type User struct {
	Id         int    `db:"id"`
	TelegramId int    `db:"telegram_id"`
	Username   string `db:"username"`
	ChatId     int64  `db:"chat_id"`
	Password   string `db:"password"`
	Locale     string `db:"locale"`

	Extra string `db:"extra"`
}

const USERFIELDS = `
  id,
  coalesce(telegram_id, 0) AS telegram_id,
  coalesce(username, '') AS username,
  coalesce(chat_id, 0) AS chat_id,
  password,
  locale
`

func loadUser(id int, telegramId int) (u User, err error) {
	err = pg.Get(&u, `
SELECT `+USERFIELDS+`
FROM telegram.account
WHERE id = $1 OR telegram_id = $2
    `, id, telegramId)
	return
}

func ensureUser(telegramId int, username string, locale string) (u User, tcase int, err error) {
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
UPDATE telegram.account AS u
SET locale = CASE WHEN u.manual_locale OR $3 = '' THEN u.locale ELSE $3 END
WHERE u.telegram_id = $1 OR u.username = $2
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

func (u User) updatePassword() (newpassword string, err error) {
	err = pg.Get(&newpassword, `
UPDATE telegram.account
SET password = DEFAULT WHERE id = $1
RETURNING password;                            
    `, u.Id)
	return
}

func (u User) getTransaction(hash string) (txn Transaction, err error) {
	err = pg.Get(&txn, `
SELECT
  time,
  telegram_peer,
  anonymous,
  status,
  trigger_message,
  tag,
  label,
  coalesce(description, '') AS description,
  fees::float/1000 AS fees,
  amount::float/1000 AS amount,
  payment_hash,
  coalesce(preimage, '') AS preimage,
  payee_node
FROM lightning.account_txn
WHERE account_id = $1
  AND payment_hash LIKE $2 || '%'
ORDER BY time, amount DESC
LIMIT 1
    `, u.Id, hash)
	if err != nil {
		return
	}

	txn.Description = escapeHTML(txn.Description)

	// handle case in which it was paid internally and so two results were returned
	if txn.Preimage.Valid == false {
		txn.Preimage = sql.NullString{String: "internal_payment", Valid: true}
	}

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

func (u User) notify(key t.Key, templateData t.T) tgbotapi.Message {
	return u.notifyWithKeyboard(key, templateData, nil, 0)
}

func (u User) notifyAsReply(key t.Key, templateData t.T, replyToId int) tgbotapi.Message {
	return u.notifyWithKeyboard(key, templateData, nil, replyToId)
}

func (u User) notifyWithKeyboard(key t.Key, templateData t.T, keyboard *tgbotapi.InlineKeyboardMarkup, replyToId int) tgbotapi.Message {
	if u.ChatId == 0 {
		log.Info().Str("user", u.Username).Str("key", string(key)).
			Msg("can't notify user as it hasn't started a chat with the bot.")
		return tgbotapi.Message{}
	}
	log.Debug().Str("user", u.Username).Str("key", string(key)).Interface("data", templateData).
		Msg("notifying user")

	msg := translateTemplate(key, u.Locale, templateData)
	return sendMessageWithKeyboard(u.ChatId, msg, keyboard, replyToId)
}

func (u User) alert(cb *tgbotapi.CallbackQuery, key t.Key, templateData t.T) (tgbotapi.APIResponse, error) {
	return bot.AnswerCallbackQuery(tgbotapi.NewCallbackWithAlert(cb.ID, translateTemplate(key, u.Locale, templateData)))
}

type makeInvoiceArgs struct {
	Desc       string
	DescHash   string
	Msatoshi   int64
	Label      string
	Preimage   string
	Expiry     *time.Duration
	MessageId  interface{}
	Tag        string
	BlueWallet bool
	SkipQR     bool
}

func (u User) makeInvoice(
	args makeInvoiceArgs,
) (bolt11 string, hash string, qrpath string, err error) {
	msatoshi := args.Msatoshi
	label := args.Label

	log.Debug().Str("user", u.Username).Str("desc", args.Desc).Int64("msats", msatoshi).
		Msg("generating invoice")

	if label == "" {
		label = makeLabel(u.Id, args.MessageId, args.Tag)
	}

	var exp time.Duration
	if args.Expiry == nil {
		exp = s.InvoiceTimeout / time.Second
	} else {
		exp = *args.Expiry / time.Second
	}

	// make normal invoice
	if args.DescHash == "" {
		params := map[string]interface{}{
			"msatoshi":    msatoshi,
			"label":       label,
			"description": args.Desc,
			"expiry":      int(exp),
		}

		if args.Preimage != "" {
			params["preimage"] = args.Preimage
		}

		var res gjson.Result
		res, err = ln.CallWithCustomTimeout(time.Second*40, "invoice", params)
		bolt11 = res.Get("bolt11").String()
		hash = res.Get("payment_hash").String()
	} else {
		// make invoice with description_hash
		var hhash []byte
		hhash, err = hex.DecodeString(args.DescHash)
		if err != nil {
			return "", "", "", fmt.Errorf("invalid description_hash: %w", err)
		}

		var ppreimage *[]byte
		if args.Preimage != "" {
			if preimage, err := hex.DecodeString(args.Preimage); err != nil {
				return "", "", "", fmt.Errorf("invalid preimage: %w", err)
			} else {
				ppreimage = &preimage
			}
		}

		expiry := time.Duration(exp) * time.Second
		bolt11, err = ln.InvoiceWithDescriptionHash(
			label,
			msatoshi,
			hhash,
			ppreimage,
			&expiry,
		)
		res, _ := ln.Call("decodepay", bolt11)
		hash = res.Get("payment_hash").String()
	}
	if err != nil {
		return "", "", "", fmt.Errorf("error making invoice: %w", err)
	}

	if args.BlueWallet {
		encodedinv, _ := json.Marshal(map[string]interface{}{
			"hash":   hash,
			"bolt11": bolt11,
			"desc":   args.Desc,
			"amount": msatoshi / 1000,
			"expiry": int(exp),
		})
		rds.Set("justcreatedbluewalletinvoice:"+strconv.Itoa(u.Id), string(encodedinv), time.Minute*10)
	}

	if !args.SkipQR {
		err = qrcode.WriteFile(strings.ToUpper(bolt11), qrcode.Medium, 256,
			qrImagePath(label))
		if err == nil {
			qrpath = qrImagePath(label)
		} else {
			log.Warn().Err(err).Str("invoice", bolt11).Msg("failed to generate qr.")
			err = nil
		}
	}

	return bolt11, hash, qrpath, nil
}

func (u User) payInvoice(
	messageId int,
	bolt11 string,
	manuallySpecifiedMsatoshi int64,
) (hash string, err error) {
	inv, err := decodepay_gjson.Decodepay(bolt11)
	if err != nil {
		return "", errors.New("Failed to decode invoice: " + err.Error())
	}

	bot.Send(tgbotapi.NewChatAction(u.ChatId, "Sending payment..."))
	amount := inv.Get("msatoshi").Int()
	desc := inv.Get("description").String()
	hash = inv.Get("payment_hash").String()

	if amount == 0 {
		amount = manuallySpecifiedMsatoshi
		if amount == 0 {
			return "", errors.New("Can't send 0.")
		}
	}

	fakeLabel := fmt.Sprintf("%s.pay.%s", s.ServiceId, hash)

	if inv.Get("payee").String() == s.NodeId {
		// it's an internal invoice. mark as paid internally.

		// handle ticket invoices
		if strings.HasPrefix(desc, "ticket for") {
			for tuple := range pendingApproval.IterBuffered() {
				label := tuple.Key
				kickdata := tuple.Val.(KickData)
				if kickdata.Hash == hash {
					var target User
					target, err = chatOwnerFromTicketLabel(label)
					if err != nil {
						return
					}

					err = u.addInternalPendingInvoice(
						0,
						target.Id,
						amount,
						hash,
						desc,
						label,
					)
					if err != nil {
						return
					}

					ticketPaid(label, kickdata)
					handleInvoicePaid(
						-1,
						amount,
						desc,
						hash,
						"",
						label,
					)
					paymentHasSucceeded(u, messageId, float64(amount), float64(amount), "", "", hash)
					break
				}
			}
		}

		// search the invoices list
		invoice, ok := findInvoiceOnNode(hash, "")
		if !ok {
			err = errors.New("Couldn't find internal invoice.")
			return
		}

		label := invoice.Get("label").String()
		messageId, targetId, _, ok := parseLabel(label)
		if ok {
			err = u.addInternalPendingInvoice(
				messageId,
				targetId,
				amount,
				hash,
				desc,
				label,
			)
			if err != nil {
				return
			}

			handleInvoicePaid(
				0,
				amount,
				desc,
				hash,
				"",
				label,
			)
			paymentHasSucceeded(u, messageId, float64(amount), float64(amount), "", "", hash)
			ln.Call("delinvoice", label, "unpaid")
		} else {
			log.Debug().Str("label", label).Msg("what is this? an internal payment unrecognized")
		}
	} else {
		// it's an invoice from elsewhere, continue and
		// actually send the lightning payment

		err := u.actuallySendExternalPayment(
			messageId, bolt11, inv, amount, fakeLabel,
			paymentHasSucceeded, paymentHasFailed,
		)
		if err != nil {
			return hash, err
		}
	}

	return hash, nil
}

func (u User) actuallySendExternalPayment(
	messageId int,
	bolt11 string,
	inv gjson.Result,
	msatoshi int64,
	label string,
	onSuccess func(
		u User,
		messageId int,
		msatoshi float64,
		msatoshi_sent float64,
		preimage string,
		tag string,
		hash string,
	),
	onFailure func(
		u User,
		messageId int,
		hash string,
	),
) (err error) {
	hash := inv.Get("payment_hash").String()

	// insert payment as pending
	txn, err := pg.Beginx()
	if err != nil {
		log.Debug().Err(err).Msg("database error starting transaction")
		return errors.New("Database error.")
	}
	defer txn.Rollback()

	fee_reserve := float64(msatoshi) * 0.01
	if msatoshi < 5000000 {
		fee_reserve = 0
	}

	_, err = txn.Exec(`
INSERT INTO lightning.transaction
  (from_id, amount, fees, description, payment_hash, label, pending, trigger_message, remote_node)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `, u.Id, msatoshi, int64(fee_reserve), inv.Get("description").String(),
		hash, label, true, messageId, inv.Get("payee").String())
	if err != nil {
		log.Debug().Err(err).Int64("msatoshi", msatoshi).
			Msg("database error inserting transaction")
		return errors.New("Payment already in course.")
	}

	balance := getBalance(txn, u.Id)
	if balance < 0 {
		return fmt.Errorf("Amount too big. Usable balance is %.3f sat.",
			(float64(balance+msatoshi)+fee_reserve)/1000)
	}

	err = txn.Commit()
	if err != nil {
		log.Debug().Err(err).Msg("database error committing transaction")
		return errors.New("Database error.")
	}

	// set common params
	params := map[string]interface{}{
		"bolt11":        bolt11,
		"riskfactor":    3,
		"maxfeepercent": 0.4,
		"exemptfee":     3000,
		"label":         fmt.Sprintf("user=%d", u.Id),
		"use_shadow":    false,
	}

	if inv.Get("msatoshi").Int() == 0 {
		// amountless invoice, so send the number of satoshis previously specified
		params["msatoshi"] = msatoshi
	}

	// perform payment
	go func() {
		var tries []Try
		var fallbackError Try

		payment, err := ln.CallWithCustomTimeout(time.Hour*24*30, "pay", params)
		if errw, ok := err.(lightning.ErrorCommand); ok {
			fallbackError = Try{
				Success: false,
				Error:   errw.Message,
				Route:   []Hop{},
			}
		}

		// save payment attempts for future counsultation
		// only save the latest 10 tries for brevity
		if status, _ := ln.Call("paystatus", bolt11); status.Get("pay.0").Exists() {
			for i, attempt := range status.Get("pay.0.attempts").Array() {
				var errorMessage string
				if attempt.Get("failure").Exists() {
					if attempt.Get("failure.data").Exists() {
						errorMessage = fmt.Sprintf("%s %s/%d %s",
							attempt.Get("failure.data.failcodename").String(),
							attempt.Get("failure.data.erring_channel").String(),
							attempt.Get("failure.data.erring_direction").Int(),
							attempt.Get("failure.data.erring_node").String(),
						)
					} else {
						errorMessage = attempt.Get("failure.message").String()
					}
				}

				route := attempt.Get("route").Array()
				hops := make([]Hop, len(route))
				for i, routehop := range route {
					hops[i] = Hop{
						Peer:      routehop.Get("id").String(),
						Channel:   routehop.Get("channel").String(),
						Direction: routehop.Get("direction").Int(),
						Msatoshi:  routehop.Get("msatoshi").Int(),
						Delay:     routehop.Get("delay").Int(),
					}
				}

				tries = append(tries, Try{
					Success: attempt.Get("success").Exists(),
					Error:   errorMessage,
					Route:   hops,
				})
			}
		}

		if payment.Get("status").String() == "complete" {
			// payment successful!
			go u.track("payment sent", map[string]interface{}{
				"sats": msatoshi / 1000,
			})
			onSuccess(
				u,
				messageId,
				payment.Get("msatoshi").Float(),
				payment.Get("msatoshi_sent").Float(),
				payment.Get("payment_preimage").String(),
				"",
				payment.Get("payment_hash").String(),
			)
			return
		}

		// call listpays to check failure
		if listpays, _ := ln.Call("listpays", bolt11); listpays.Get("pays.#").Int() == 1 && listpays.Get("pays.0.status").String() != "failed" {
			// not a failure -- but also not a success
			// we don't know what happened, maybe it's pending, so don't do anything
			log.Debug().Str("bolt11", bolt11).
				Msg("we don't know what happened with this payment")
			return
		}

		// if we reached this point then it's a failure
	failure:
		from = len(tries) - 10
		if from < 0 {
			from = 0
		}
		if jsontries, err := json.Marshal(tries[from:]); err == nil {
			rds.Set("tries:"+hash[:5], jsontries, time.Hour*24)
		}

		go u.track("payment failed", map[string]interface{}{
			"sats":  msatoshi / 1000,
			"payee": inv.Get("payee").String(),
		})
		log.Warn().
			Str("user", u.Username).
			Int("user-id", u.Id).
			Interface("params", params).
			Interface("tries", tries).
			Str("hash", hash).
			Msg("payment failed")
			// give the money back to the user
		onFailure(u, messageId, hash)
	}()

	return nil
}

func (u User) addInternalPendingInvoice(
	messageId int,
	targetId int,
	msats int64,
	hash string,
	desc, label interface{},
) (err error) {
	// insert payment as pending
	txn, err := pg.Beginx()
	if err != nil {
		log.Debug().Err(err).Msg("database error starting transaction")
		return errors.New("Database error.")
	}
	defer txn.Rollback()

	_, err = txn.Exec(`
INSERT INTO lightning.transaction
  (from_id, to_id, amount, description, payment_hash, label, pending, trigger_message)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `, u.Id, targetId, msats, desc, hash, label, true, messageId)
	if err != nil {
		log.Debug().Err(err).Msg("database error inserting transaction")
		return errors.New("Payment already in course.")
	}

	balance := getBalance(txn, u.Id)
	if balance < 0 {
		return fmt.Errorf("Insufficient balance. Needs %.0f sat more.",
			-float64(balance)/1000)
	}

	err = txn.Commit()
	if err != nil {
		log.Debug().Err(err).Msg("database error committing transaction")
		return errors.New("Database error.")
	}

	return nil
}

func (u User) sendInternally(
	messageId int,
	target User,
	anonymous bool,
	msats int,
	desc string,
	hash string,
	tag string,
) (string, error) {
	if target.Id == u.Id || target.Username == u.Username || target.TelegramId == u.TelegramId {
		return "Can't pay yourself.", errors.New("user trying to pay itself")
	}

	if msats == 0 {
		// if nothing was provided, end here
		return "No amount provided.", errors.New("no amount provided")
	}

	var (
		descn = sql.NullString{String: desc, Valid: desc != ""}
		tagn  = sql.NullString{String: tag, Valid: tag != ""}
		hashn = sql.NullString{String: hash, Valid: hash != ""}
	)

	txn, err := pg.Beginx()
	if err != nil {
		return "Database error.", err
	}
	defer txn.Rollback()

	_, err = txn.Exec(`
INSERT INTO lightning.transaction
  (from_id, to_id, anonymous, amount, description, tag, payment_hash, trigger_message)
VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  $6,
  CASE WHEN $7::text IS NOT NULL
    THEN $7::text
    ELSE md5(random()::text) || md5(random()::text)
  END,
  $8
)
    `, u.Id, target.Id, anonymous, msats, descn, tagn, hashn, messageId)
	if err != nil {
		return "Database error.", err
	}

	balance := getBalance(txn, u.Id)
	if balance < 0 {
		return fmt.Sprintf("Insufficient balance. Needs %.3f sat more.",
				-float64(balance)/1000),
			errors.New("insufficient balance")
	}

	err = txn.Commit()
	if err != nil {
		return "Unable to pay due to internal database error.", err
	}

	return "", nil
}

func (u User) sendThroughProxy(
	// these must be unique across payments that must be combined, otherwise different
	sourcehash string,
	targethash string,
	// ~ this is very important
	sourceMessageId int,
	targetMessageId int,
	target User,
	msats int,
	sourcedesc string,
	targetdesc string,
	pending bool,
	tag string,
) (string, error) {
	var (
		tagn        = sql.NullString{String: tag, Valid: tag != ""}
		sourcedescn = sql.NullString{String: sourcedesc, Valid: sourcedesc != ""}
		targetdescn = sql.NullString{String: targetdesc, Valid: targetdesc != ""}
	)

	// start transaction
	txn, err := pg.Beginx()
	if err != nil {
		return "Database error.", err
	}
	defer txn.Rollback()

	// send transaction source->proxy, then proxy->target
	// both are updated if exist
	_, err = txn.Exec(`
INSERT INTO lightning.transaction AS t
  (payment_hash, from_id, to_id, amount, description, tag, trigger_message)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (payment_hash) DO UPDATE SET
  amount = t.amount + $4,
  description = $5,
  tag = $6,
  trigger_message = $7
    `, sourcehash, u.Id, s.ProxyAccount, msats, sourcedescn, tagn, sourceMessageId)
	if err != nil {
		return "Database error.", err
	}

	_, err = txn.Exec(`
INSERT INTO lightning.transaction AS t
  (proxied_with, payment_hash, from_id, to_id, amount,
   description, tag, trigger_message, pending)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `, sourcehash, targethash, s.ProxyAccount, target.Id, msats,
		targetdescn, tagn, targetMessageId, pending)
	if err != nil {
		return "Database error.", err
	}

	// check balance
	balance := getBalance(txn, u.Id)
	if balance < 0 {
		return fmt.Sprintf("Insufficient balance. Needs %.3f sat more.",
				-float64(balance)/1000),
			errors.New("insufficient balance")
	}

	// check proxy balance (should be always zero)
	if err := checkProxyBalance(txn); err != nil {
		log.Error().Err(err).Msg("proxy balance check")
		return "Database error.", err
	}

	err = txn.Commit()
	if err != nil {
		return "Unable to pay due to internal database error.", err
	}

	return "", nil
}

func (u User) paymentReceived(
	amount int,
	desc string,
	hash string,
	preimage string,
	label string,
	tag string,
) (err error) {
	tagn := sql.NullString{String: tag, Valid: tag != ""}

	_, err = pg.Exec(`
INSERT INTO lightning.transaction
  (to_id, amount, description, payment_hash, preimage, tag, label)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (payment_hash) DO UPDATE SET to_id = $1
    `, u.Id, amount, desc, hash, preimage, tagn, label)
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
  CASE
    WHEN b.balance > 5000000 THEN b.balance * 0.99009 / 1000
    ELSE b.balance/1000
  END AS usable,
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
	if err == sql.ErrNoRows {
		info = Info{
			AccountId:     strconv.Itoa(u.Id),
			Balance:       0,
			UsableBalance: 0,
			TotalSent:     0,
			TotalReceived: 0,
			TotalFees:     0,
		}
	}

	return
}

func (u User) getTaggedBalances() (balances []TaggedBalance, err error) {
	err = pg.Select(&balances, `
SELECT
  tag,
  sum(amount)::float/1000 AS balance
FROM lightning.account_txn
WHERE account_id = $1 AND tag IS NOT NULL
GROUP BY tag
    `, u.Id)
	return
}

type InOut string

const (
	In   InOut = "in"
	Out  InOut = "out"
	Both InOut = ""
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
    tag,
    label,
    fees::float/1000 AS fees,
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

func (u User) checkBalanceFor(sats int, purpose string, cb *tgbotapi.CallbackQuery) bool {
	notify := func(key t.Key, templateData t.T) {
		if cb == nil {
			u.notify(key, templateData)
		} else {
			u.alert(cb, key, templateData)
		}
	}

	if info, err := u.getInfo(); err != nil || int(info.Balance) < sats {
		notify(t.INSUFFICIENTBALANCE, t.T{
			"Purpose": purpose,
			"Sats":    float64(sats) - info.Balance,
		})
		return false
	}
	return true
}

func (u User) setAppData(appname string, data interface{}) (err error) {
	j, err := json.Marshal(data)
	if err != nil {
		return
	}

	_, err = pg.Exec(`
UPDATE telegram.account AS u
SET appdata = jsonb_set(u.appdata, ARRAY[$2], $3, true)
WHERE id = $1
    `, u.Id, appname, types.JSONText(j))
	return
}

func (u User) getAppData(appname string, data interface{}) (err error) {
	var j types.JSONText
	err = pg.Get(&j, `
SELECT coalesce(appdata -> $2, '{}'::jsonb)
FROM telegram.account
WHERE id = $1
    `, u.Id, appname)
	if err != nil {
		return err
	}

	err = j.Unmarshal(data)
	return
}

func (u User) track(event string, eventProperties map[string]interface{}) {
	amp.Event(amplitude.Event{
		UserId:          strconv.Itoa(u.Id),
		EventType:       event,
		EventProperties: eventProperties,
	})
}

func paymentHasSucceeded(
	u User,
	messageId int,
	msatoshi float64,
	msatoshi_sent float64,
	preimage string,
	tag string,
	hash string,
) {
	// if it succeeds we mark the transaction as not pending anymore
	// plus save fees and preimage
	fees := msatoshi_sent - msatoshi

	// if there's a tag we save that too, otherwise leave it null
	tagn := sql.NullString{String: tag, Valid: tag != ""}

	_, err = pg.Exec(`
UPDATE lightning.transaction
SET fees = $1, preimage = $2, pending = false, tag = $4
WHERE payment_hash = $3
    `, fees, preimage, hash, tagn)
	if err != nil {
		log.Error().Err(err).
			Str("user", u.Username).
			Str("hash", hash).
			Float64("fees", fees).
			Msg("failed to update transaction fees.")
		u.notifyAsReply(t.DBERROR, nil, messageId)
	}

	u.notifyAsReply(t.PAIDMESSAGE, t.T{
		"Sats":      float64(msatoshi) / 1000,
		"Fee":       fees / 1000,
		"Hash":      hash,
		"Preimage":  preimage,
		"ShortHash": hash[:5],
	}, messageId)
}

func paymentHasFailed(u User, messageId int, hash string) {
	u.notifyAsReply(t.PAYMENTFAILED, t.T{"ShortHash": hash[:5]}, messageId)

	_, err := pg.Exec(
		`DELETE FROM lightning.transaction WHERE payment_hash = $1`, hash)
	if err != nil {
		log.Error().Err(err).Str("hash", hash).
			Msg("failed to cancel transaction after routing failure")
	}
}

type Info struct {
	AccountId     string  `db:"account_id"`
	Balance       float64 `db:"balance"`
	UsableBalance float64 `db:"usable"`
	TotalSent     float64 `db:"totalsent"`
	TotalReceived float64 `db:"totalrecv"`
	TotalFees     float64 `db:"fees"`
}

type TaggedBalance struct {
	Tag     string  `db:"tag"`
	Balance float64 `db:"balance"`
}
