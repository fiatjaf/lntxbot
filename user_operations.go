package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/bwmarrin/discordgo"
	"github.com/fiatjaf/eclair-go"
	decodepay "github.com/fiatjaf/ln-decodepay"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jmoiron/sqlx/types"
)

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

func (u User) invoicePrivateKey() *btcec.PrivateKey {
	// derive custom private key for this user
	seedhash := sha256.Sum256(
		[]byte(fmt.Sprintf("invoicekeyseed:%d:%s", u.Id, s.TelegramBotToken)))
	sk, _ := btcec.PrivKeyFromBytes(btcec.S256(), seedhash[:])
	return sk
}

func (u User) makeInvoice(
	ctx context.Context,
	args *MakeInvoiceArgs,
) (bolt11 string, hash string, err error) {
	msatoshi := args.Msatoshi
	log.Debug().Stringer("user", &u).Str("desc", args.Description).Int64("msats", msatoshi).
		Msg("generating invoice")

	if args.Expiry == nil {
		args.Expiry = &s.InvoiceTimeout
	}

	preimage := make([]byte, 32)
	if _, err := rand.Read(preimage); err != nil {
		return "", "", fmt.Errorf("can't create random preimage: %w", err)
	}

	// hide the user id inside the preimage (first 4 bytes)
	binary.BigEndian.PutUint32(preimage, uint32(u.Id))

	params := eclair.Params{
		"expireIn":        int((*args.Expiry).Seconds()),
		"paymentPreimage": hex.EncodeToString(preimage),
	}

	// only have the amountMsat parameter if given (i.e., not "any")
	if msatoshi > 0 {
		params["amountMsat"] = msatoshi
	}

	if args.DescriptionHash == "" {
		params["description"] = args.Description
	} else {
		params["descriptionHash"] = args.DescriptionHash
	}

	inv, err := ln.Call("createinvoice", params)
	if err != nil {
		return "", "", fmt.Errorf("failed to create invoice: %w", err)
	}

	var messageId interface{}
	if message := ctx.Value("message"); message != nil {
		switch m := message.(type) {
		case *tgbotapi.Message:
			messageId = m.MessageID
		case *discordgo.Message:
			messageId = m.ID
		}
	}
	hash = inv.Get("paymentHash").String()
	bolt11 = inv.Get("serialized").String()

	saveInvoiceData(hash, InvoiceData{
		UserId:    u.Id,
		Origin:    ctx.Value("origin").(string),
		MessageId: messageId,
		Preimage:  hex.EncodeToString(preimage),

		MakeInvoiceArgs: args,
	})

	if args.BlueWallet {
		encodedinv, _ := json.Marshal(map[string]interface{}{
			"hash":   hash,
			"bolt11": bolt11,
			"desc":   args.Description,
			"amount": msatoshi / 1000,
			"expiry": int((*args.Expiry)),
		})
		rds.Set("justcreatedbluewalletinvoice:"+strconv.Itoa(u.Id), string(encodedinv), time.Minute*10)
	}

	return bolt11, hash, nil
}

func (u User) payInvoice(
	ctx context.Context,
	bolt11 string,
	manuallySpecifiedMsatoshi int64,
) (hash string, err error) {
	inv, err := decodepay.Decodepay(bolt11)
	if err != nil {
		return "", errors.New("Failed to decode invoice: " + err.Error())
	}

	if u.TelegramChatId != 0 {
		bot.Send(tgbotapi.NewChatAction(u.TelegramChatId, "Sending payment..."))
	}

	amount := inv.MSatoshi
	hash = inv.PaymentHash

	if amount == 0 {
		amount = manuallySpecifiedMsatoshi
		if amount == 0 {
			return hash, errors.New("Can't send 0.")
		}
	}

	if inv.Payee == s.NodeId {
		data, err := loadInvoiceData(inv.PaymentHash)
		if err != nil {
			log.Debug().Err(err).Interface("invoice", inv).
				Msg("no invoice stored for this hash, not a bot invoice?")
			return hash, errors.New("Can't pay internal invoice that isn't from the bot.")
		}

		// it's an internal invoice. mark as paid internally.
		err = u.addInternalPendingInvoice(
			ctx,
			data.UserId,
			data.Msatoshi,
			hash,
			data.Description,
		)
		if err != nil {
			return hash, err
		}

		if data.Msatoshi > amount {
			return hash,
				fmt.Errorf("Invoice is for %d, can't pay less.", data.Msatoshi)
		} else if amount > data.Msatoshi*2 {
			return hash,
				fmt.Errorf("Invoice is for %d, can't pay more than the double.",
					data.Msatoshi)
		}

		go paymentReceived(ctx, hash, data.Msatoshi)
		go paymentHasSucceeded(ctx, amount, 0, data.Preimage, data.Tag, hash)

		return hash, nil
	} else {
		// it's an invoice from elsewhere, continue and
		// actually send the lightning payment
		err = u.actuallySendExternalPayment(ctx, bolt11, inv, amount)
		if err != nil {
			return hash, err
		}

		return hash, nil
	}
}

func (u User) actuallySendExternalPayment(
	ctx context.Context,
	bolt11 string,
	inv decodepay.Bolt11,
	msatoshi int64,
) (err error) {
	hash := inv.PaymentHash

	// insert payment as pending
	txn, err := pg.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		log.Debug().Err(err).Msg("database error starting transaction")
		return ErrDatabase
	}
	defer txn.Rollback()

	// if not tg this will just be ignored
	// TODO
	var tgMessageId int
	if message := ctx.Value("message"); message != nil {
		if m, ok := message.(*tgbotapi.Message); ok {
			tgMessageId = m.MessageID
		}
	}

	fee_reserve := float64(msatoshi) * 0.005
	if msatoshi < 1000000 {
		fee_reserve += 5000 // account for exemptfee
	}

	_, err = txn.Exec(`
INSERT INTO lightning.transaction
  (from_id, amount, fees, description, payment_hash, pending,
   trigger_message, remote_node)
VALUES ($1, $2, $3, $4, $5, true, $6, $7)
    `, u.Id, msatoshi, int64(fee_reserve), inv.Description,
		hash, tgMessageId, inv.Payee)
	if err != nil {
		log.Debug().Err(err).Int64("msatoshi", msatoshi).
			Msg("database error inserting transaction")
		return errors.New("Payment already in course.")
	}

	if balance := getBalance(txn, u.Id); balance < 0 {
		return errors.New("Insufficient balance.")
	}

	err = txn.Commit()
	if err != nil {
		log.Debug().Err(err).Msg("database error committing transaction")
		return ErrDatabase
	}

	// set common params
	params := eclair.Params{
		"invoice":     bolt11,
		"maxAttempts": 20,
		"maxFeePct":   0.5,
		"externalId":  fmt.Sprintf("lntxbot:user=%d", u.Id),
	}

	if inv.MSatoshi == 0 {
		// amountless invoice, so send the number of satoshis previously specified
		params["amountMsat"] = msatoshi
	}

	// perform payment
	go func() {
		_, err := ln.Call("payinvoice", params)
		if err != nil {
			send(ctx, t.ERROR, t.T{"Err": err.Error()})
		}
	}()

	return nil
}

func (u User) addInternalPendingInvoice(
	ctx context.Context,
	targetId int,
	msats int64,
	hash string,
	desc interface{},
) (err error) {
	// insert payment as pending
	txn, err := pg.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		log.Debug().Err(err).Msg("database error starting transaction")
		return ErrDatabase
	}
	defer txn.Rollback()

	// if not tg this will just be ignored
	// TODO maybe remove trigger_message from the database
	var tgMessageId int
	if message := ctx.Value("message"); message != nil {
		if m, ok := message.(*tgbotapi.Message); ok {
			tgMessageId = m.MessageID
		}
	}

	_, err = txn.Exec(`
INSERT INTO lightning.transaction
  (from_id, to_id, amount, description, payment_hash, pending, trigger_message)
VALUES ($1, $2, $3, $4, $5, true, $6)
    `, u.Id, targetId, msats, desc, hash, tgMessageId)
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
	ctx context.Context,
	target User,
	anonymous bool,
	msats int64,
	fees int64,
	desc string,
	hash string,
	tag string,
) error {
	if target.Id == u.Id {
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

	txn, err := pg.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return ErrDatabase
	}
	defer txn.Rollback()

	// if not tg this will just be ignored
	// TODO maybe remove trigger_message from the database
	var tgMessageId int
	if message := ctx.Value("message"); message != nil {
		if m, ok := message.(*tgbotapi.Message); ok {
			tgMessageId = m.MessageID
		}
	}

	_, err = txn.Exec(`
INSERT INTO lightning.transaction (
  from_id,
  to_id,
  anonymous,
  amount,
  fees,
  description,
  tag,
  payment_hash,
  trigger_message
)
VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  $6,
  $7,
  CASE WHEN $8::text IS NOT NULL
    THEN $8::text
    ELSE md5(random()::text) || md5(random()::text)
  END,
  $9
)
    `, u.Id, target.Id, anonymous, msats, fees, descn, tagn, hashn, tgMessageId)
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
	ctx context.Context,
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
	txn, err := pg.BeginTxx(ctx, &sql.TxOptions{})
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

func (u User) setAppData(appname string, data interface{}) (err error) {
	j, err := json.Marshal(data)
	if err != nil {
		return
	}

	_, err = pg.Exec(`
UPDATE account AS u
SET appdata = jsonb_set(u.appdata, ARRAY[$2], $3, true)
WHERE id = $1
    `, u.Id, appname, types.JSONText(j))
	return
}

func (u User) getAppData(appname string, data interface{}) (err error) {
	var j types.JSONText
	err = pg.Get(&j, `
SELECT coalesce(appdata -> $2, '{}'::jsonb)
FROM account
WHERE id = $1
    `, u.Id, appname)
	if err != nil {
		return err
	}

	err = j.Unmarshal(data)
	return
}
