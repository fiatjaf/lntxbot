package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec"
	lightning "github.com/fiatjaf/lightningd-gjson-rpc"
	decodepay "github.com/fiatjaf/ln-decodepay"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/types"
	"github.com/msingleton/amplitude-go"
	"github.com/skip2/go-qrcode"
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
	Desc                   string
	DescHash               string
	Msatoshi               int64
	Expiry                 *time.Duration
	MessageId              int
	Tag                    string
	Extra                  map[string]interface{}
	BlueWallet             bool
	SkipQR                 bool
	IgnoreInvoiceSizeLimit bool
}

func (u User) makeInvoice(
	args makeInvoiceArgs,
) (bolt11 string, hash string, qrpath string, err error) {
	msatoshi := args.Msatoshi

	// limit number of small invoices people can make every day
	if !args.IgnoreInvoiceSizeLimit {
		if msatoshi != 0 {
			if msatoshi <= 100000 {
				invoicespamkey := "invspam:" + strconv.Itoa(u.Id)
				spam := rds.HGetAll(invoicespamkey).Val()
				if spam != nil {
					for _, limit := range INVOICESPAMLIMITS {
						if msatoshi <= limit.EqualOrSmallerThan {
							ns, _ := spam[limit.Key]
							n, _ := strconv.Atoi(ns)

							go rds.HSet(invoicespamkey, limit.Key, n+1)

							// expire this at the end of the day
							t := time.Now().AddDate(0, 0, 1)
							t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
							go rds.ExpireAt(invoicespamkey, t)

							if n >= limit.PerDay {
								return "", "", "",
									fmt.Errorf("The issuance of invoices smaller than %dmsat is restricted to %d per day.", limit.EqualOrSmallerThan, limit.PerDay)
							}
						}
					}
				}
			}
		}
	}

	log.Debug().Str("user", u.Username).Str("desc", args.Desc).Int64("msats", msatoshi).
		Msg("generating invoice")

	var exp *time.Duration
	if args.Expiry == nil {
		exp = &s.InvoiceTimeout
	} else {
		exp = args.Expiry
	}

	preimage := make([]byte, 32)
	_, err = rand.Read(preimage)
	if err != nil {
		return "", "", "", fmt.Errorf("can't create random bytes: %w", err)
	}

	extra := args.Extra
	if extra == nil {
		extra = make(map[string]interface{})
	}

	shadowData := ShadowChannelData{
		UserId:    u.Id,
		MessageId: args.MessageId,
		Tag:       args.Tag,
		Msatoshi:  msatoshi,
		// Description: added next
		Preimage: hex.EncodeToString(preimage),
		Extra:    extra,
	}

	var descriptionOrDescriptionHash interface{}
	if args.DescHash == "" {
		descriptionOrDescriptionHash = args.Desc
		shadowData.Description = args.Desc
	} else {
		descriptionOrDescriptionHash, err = hex.DecodeString(args.DescHash)
		if err != nil {
			return "", "", "", fmt.Errorf("invalid description_hash: %w", err)
		}
		shadowData.DescriptionHash = args.DescHash
	}

	// derive custom private key for this user
	seedhash := sha256.Sum256(
		[]byte(fmt.Sprintf("invoicekeyseed:%d:%s", u.Id, s.BotToken)))
	sk, _ := btcec.PrivKeyFromBytes(btcec.S256(), seedhash[:])

	bolt11, hash, err = ln.InvoiceWithShadowRoute(
		msatoshi,
		descriptionOrDescriptionHash,
		&preimage,
		&sk,
		exp,
		0,
		0,
		9,
		makeShadowChannelId(shadowData),
	)
	if err != nil {
		return "", "", "", fmt.Errorf("error making invoice: %w", err)
	}

	if args.BlueWallet {
		encodedinv, _ := json.Marshal(map[string]interface{}{
			"hash":   hash,
			"bolt11": bolt11,
			"desc":   args.Desc,
			"amount": msatoshi / 1000,
			"expiry": int(*exp),
		})
		rds.Set("justcreatedbluewalletinvoice:"+strconv.Itoa(u.Id), string(encodedinv), time.Minute*10)
	}

	if !args.SkipQR {
		err = qrcode.WriteFile(strings.ToUpper(bolt11), qrcode.Medium, 256,
			qrImagePath(hash))
		if err == nil {
			qrpath = qrImagePath(hash)
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
	invd, err := decodepay.Decodepay(bolt11)
	if err != nil {
		return "", errors.New("Failed to decode invoice: " + err.Error())
	}
	inv := Invoice{
		Bolt11:   invd,
		Preimage: "",
	}

	bot.Send(tgbotapi.NewChatAction(u.ChatId, "Sending payment..."))

	amount := inv.MSatoshi
	hash = inv.PaymentHash

	if amount == 0 {
		amount = manuallySpecifiedMsatoshi
		if amount == 0 {
			return hash, errors.New("Can't send 0.")
		}
	}

	if len(inv.Route) == 1 && inv.Route[0][0].PubKey == s.NodeId {
		// it's an internal invoice. mark as paid internally.
		bscid, err := decodeShortChannelId(inv.Route[0][0].ShortChannelId)
		if err != nil {
			return hash, errors.New("Failed to decode short_channel_id: " + err.Error())
			log.Debug().Str("hash", hash).
				Msg("what is this? an internal payment unrecognized")
		}
		shadowData, ok := extractDataFromShadowChannelId(bscid)
		if !ok {
			log.Debug().Str("hash", hash).Str("scid", inv.Route[0][0].ShortChannelId).
				Msg("what is this? an internal payment unrecognized")
			return hash, errors.New("Failed to identity internal invoice.")
		}

		err = u.addInternalPendingInvoice(
			shadowData.MessageId,
			shadowData.UserId,
			shadowData.Msatoshi,
			hash,
			shadowData.Description,
		)
		if err != nil {
			return hash, err
		}

		if shadowData.Msatoshi > amount {
			return hash,
				fmt.Errorf("Invoice is for %d, can't pay less.", shadowData.Msatoshi)
		} else if amount > shadowData.Msatoshi*2 {
			return hash,
				fmt.Errorf("Invoice is for %d, can't pay more than the double.",
					shadowData.Msatoshi)
		}

		inv.Preimage = shadowData.Preimage

		go handleInvoicePaid(hash, shadowData)
		go resolveWaitingInvoice(hash, inv)
		go paymentHasSucceeded(u, messageId, float64(amount), float64(amount),
			shadowData.Preimage, shadowData.Tag, hash)
	} else {
		// it's an invoice from elsewhere, continue and
		// actually send the lightning payment

		err := u.actuallySendExternalPayment(
			messageId, bolt11, inv, amount,
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
	inv Invoice,
	msatoshi int64,
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
	hash := inv.PaymentHash

	// insert payment as pending
	txn, err := pg.Beginx()
	if err != nil {
		log.Debug().Err(err).Msg("database error starting transaction")
		return ErrDatabase
	}
	defer txn.Rollback()

	fee_reserve := float64(msatoshi) * 0.01
	if msatoshi < 5000000 {
		fee_reserve = 0
	}

	_, err = txn.Exec(`
INSERT INTO lightning.transaction
  (from_id, amount, fees, description, payment_hash, pending, trigger_message, remote_node)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `, u.Id, msatoshi, int64(fee_reserve), inv.Description,
		hash, true, messageId, inv.Payee)
	if err != nil {
		log.Debug().Err(err).Int64("msatoshi", msatoshi).
			Msg("database error inserting transaction")
		return errors.New("Payment already in course.")
	}

	balance := getBalance(txn, u.Id)
	if balance < 0 {
		return errors.New("Insufficient balance.")
	}

	err = txn.Commit()
	if err != nil {
		log.Debug().Err(err).Msg("database error committing transaction")
		return ErrDatabase
	}

	exemptfee := 3000
	if balance > 10000 {
		exemptfee = 7000
	}

	// set common params
	params := map[string]interface{}{
		"bolt11":        bolt11,
		"riskfactor":    3,
		"maxfeepercent": 0.4,
		"exemptfee":     exemptfee,
		"label":         fmt.Sprintf("user=%d", u.Id),
		"use_shadow":    false,
	}

	if inv.MSatoshi == 0 {
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

				if i >= 9 {
					break
				}
			}
		} else {
			// no 'paystatus' for this payment: it has failed before any attempt
			// let's use the error message returned from 'pay'
			tries = append(tries, fallbackError)
		}

		// save the payment tries here
		if jsontries, err := json.Marshal(tries); err == nil {
			rds.Set("tries:"+hash[:5], jsontries, time.Hour*24)
		}

		// check success (from the 'pay' call)
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

		// check failure (by checking the 'listpays' command)
		// that should be enough for us to be 100% sure
		listpays, _ := ln.Call("listpays", bolt11)
		payattempts := listpays.Get("pays.#").Int()
		if payattempts == 1 {
			status := listpays.Get("pays.0.status").String()

			switch status {
			case "failed":
				go u.track("payment failed", map[string]interface{}{
					"sats":  msatoshi / 1000,
					"payee": inv.Payee,
				})
				log.Warn().
					Str("user", u.Username).
					Int("user-id", u.Id).
					Interface("params", params).
					Interface("tries", tries).
					Str("bolt11", bolt11).
					Str("hash", hash).
					Msg("payment failed according to listpays")
					// give the money back to the user
				onFailure(u, messageId, hash)
			case "success":
				log.Debug().Str("bolt11", bolt11).
					Msg("listpays success. we shouldn't reach this code ever.")
				return
			default:
				// not a failure -- but also not a success
				// we don't know what happened, maybe it's pending,
				// so don't do anything
				log.Debug().Str("bolt11", bolt11).
					Msg("we don't know what happened with this payment")
				return
			}
		}

		// the payment wasn't even tried -- so it's a failure
		if payattempts == 0 {
			go u.track("payment failed", map[string]interface{}{
				"sats":  msatoshi / 1000,
				"payee": inv.Payee,
			})
			log.Warn().
				Str("user", u.Username).
				Int("user-id", u.Id).
				Interface("params", params).
				Interface("tries", tries).
				Str("hash", hash).
				Msg("payment wasn't even tried")
				// give the money back to the user
			onFailure(u, messageId, hash)
		}
	}()

	return nil
}

func (u User) addInternalPendingInvoice(
	messageId int,
	targetId int,
	msats int64,
	hash string,
	desc interface{},
) (err error) {
	// insert payment as pending
	txn, err := pg.Beginx()
	if err != nil {
		log.Debug().Err(err).Msg("database error starting transaction")
		return ErrDatabase
	}
	defer txn.Rollback()

	_, err = txn.Exec(`
INSERT INTO lightning.transaction
  (from_id, to_id, amount, description, payment_hash, pending, trigger_message)
VALUES ($1, $2, $3, $4, $5, $6, $7)
    `, u.Id, targetId, msats, desc, hash, true, messageId)
	if err != nil {
		log.Debug().Err(err).Msg("database error inserting transaction")
		return errors.New("Payment already in course.")
	}

	balance := getBalance(txn, u.Id)
	if balance < 0 {
		return ErrInsufficientBalance
	}

	err = txn.Commit()
	if err != nil {
		log.Debug().Err(err).Msg("database error committing transaction")
		return ErrDatabase
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
) error {
	if target.Id == u.Id || target.Username == u.Username || target.TelegramId == u.TelegramId {
		return errors.New("Can't pay yourself.")
	}

	if msats == 0 {
		// if nothing was provided, end here
		return ErrInvalidAmount
	}

	var (
		descn = sql.NullString{String: desc, Valid: desc != ""}
		tagn  = sql.NullString{String: tag, Valid: tag != ""}
		hashn = sql.NullString{String: hash, Valid: hash != ""}
	)

	txn, err := pg.Beginx()
	if err != nil {
		return ErrDatabase
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
		return ErrDatabase
	}

	balance := getBalance(txn, u.Id)
	if balance < 0 {
		return ErrInsufficientBalance
	}

	err = txn.Commit()
	if err != nil {
		return ErrDatabase
	}

	return nil
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
	tag string,
) (err error) {
	tagn := sql.NullString{String: tag, Valid: tag != ""}

	_, err = pg.Exec(`
INSERT INTO lightning.transaction
  (to_id, amount, description, payment_hash, preimage, tag)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (payment_hash) DO UPDATE SET to_id = $1
    `, u.Id, amount, desc, hash, preimage, tagn)
	if err != nil {
		log.Error().Err(err).
			Str("user", u.Username).Str("hash", hash).
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
		err = nil
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

func (u User) listTransactions(limit, offset, descCharLimit int, tag string, inOrOut InOut) (txns []Transaction, err error) {
	var filter string
	switch inOrOut {
	case In:
		filter += " AND amount > 0 "
	case Out:
		filter += " AND amount < 0 "
	case Both:
		filter += ""
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
    fees::float/1000 AS fees,
    amount::float/1000 AS amount,
    payment_hash,
    preimage
  FROM lightning.account_txn
  WHERE account_id = $1 `+filter+` AND (CASE WHEN $5 != '' THEN tag = $5 ELSE true END)
  ORDER BY time DESC
  LIMIT $2
  OFFSET $3
) AS latest ORDER BY time ASC
    `, u.Id, limit, offset, descCharLimit, tag)
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
