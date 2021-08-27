package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/bwmarrin/discordgo"
	"github.com/fiatjaf/eclair-go"
	decodepay "github.com/fiatjaf/ln-decodepay"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jmoiron/sqlx/types"
	cmap "github.com/orcaman/concurrent-map"
	"gopkg.in/antage/eventsource.v1"
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
	args makeInvoiceArgs,
) (bolt11 string, hash string, err error) {
	msatoshi := args.Msatoshi

	// limit number of small invoices people can make every day
	if !args.IgnoreInvoiceSizeLimit && msatoshi != 0 && s.RateBucketKey != "" {
		for key, limit := range INVOICESPAMLIMITS {
			if msatoshi <= limit {
				if !checkInvoiceRateLimit(key, u.Id) {
					return "", "", errors.New(
						"Creating too many small invoices, please wait one hour.")
				}
			}
		}
	}

	log.Debug().Stringer("user", &u).Str("desc", args.Description).Int64("msats", msatoshi).
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
		return "", "", fmt.Errorf("can't create random bytes: %w", err)
	}

	params := eclair.Params{
		"amountMsat":      msatoshi,
		"expireIn":        exp.Seconds(),
		"paymentPreimage": hex.EncodeToString(preimage),
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

		makeInvoiceArgs: args,
	})

	if args.BlueWallet {
		encodedinv, _ := json.Marshal(map[string]interface{}{
			"hash":   hash,
			"bolt11": bolt11,
			"desc":   args.Description,
			"amount": msatoshi / 1000,
			"expiry": int(*exp),
		})
		rds.Set("justcreatedbluewalletinvoice:"+strconv.Itoa(u.Id), string(encodedinv), time.Minute*10)
	}

	return bolt11, hash, nil
}

// what happens when a payment is received
var userPaymentStream = cmap.New() // make(map[int]eventsource.EventSource)

func (u User) onReceivedInvoicePayment(ctx context.Context, hash string, data InvoiceData) {
	u.track("got payment", map[string]interface{}{
		"sats": float64(data.Msatoshi) / 1000,
	})

	// send to user stream if the user is listening
	if ies, ok := userPaymentStream.Get(strconv.Itoa(u.Id)); ok {
		go ies.(eventsource.EventSource).SendEventMessage(
			`{"payment_hash": "`+hash+`", "msatoshi": `+
				strconv.FormatInt(data.Msatoshi, 10)+`}`,
			"payment-received", "")
	}

	// is there a comment associated with this?
	go func() {
		time.Sleep(2 * time.Second)
		if comment, ok := data.Extra["comment"]; ok && comment != "" {
			send(ctx, u, t.LNURLPAYCOMMENT, t.T{
				"Text":           comment,
				"HashFirstChars": hash[:5],
			})
		}
	}()

	// proceed to compute an incoming payment for this user
	if err := u.paymentReceived(
		int(data.Msatoshi),
		data.Description,
		hash,
		data.Preimage,
		data.Tag,
	); err != nil {
		send(ctx, u, t.FAILEDTOSAVERECEIVED, t.T{"Hash": hash}, data.MessageId)
		if dmi, ok := data.MessageId.(DiscordMessageID); ok {
			discord.MessageReactionAdd(dmi.Channel(), dmi.Message(), "✅")
		}
		return
	}

	send(ctx, u, t.PAYMENTRECEIVED, t.T{
		"Sats": data.Msatoshi / 1000,
		"Hash": hash[:5],
	})

	if dmi, ok := data.MessageId.(DiscordMessageID); ok {
		discord.MessageReactionAdd(dmi.Channel(), dmi.Message(), "⚠️")
	}
}

func (u User) payInvoice(
	ctx context.Context,
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

		inv.Preimage = data.Preimage

		receiver, err := loadUser(data.UserId)
		if err != nil {
			log.Warn().Err(err).Interface("data", data).
				Msg("failed to load receiver on internal invoice")
			return hash,
				errors.New("Internal error loading the receiver for this invoice.")
		}

		go receiver.onReceivedInvoicePayment(ctx, hash, data)
		go resolveWaitingInvoice(hash, inv)
		go paymentHasSucceeded(ctx, amount, 0, data.Preimage, data.Tag, hash)

		return hash, nil
	}

	// it's an invoice from elsewhere, continue and
	// actually send the lightning payment
	err = u.actuallySendExternalPayment(ctx, bolt11, inv, amount)
	if err != nil {
		return hash, err
	}

	return hash, nil
}

func (u User) actuallySendExternalPayment(
	ctx context.Context,
	bolt11 string,
	inv Invoice,
	msatoshi int64,
) (err error) {
	hash := inv.PaymentHash

	// insert payment as pending
	txn, err := pg.Beginx()
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
VALUES ($1, $2, $3, $4, $5, true, $7, $8)
    `, u.Id, msatoshi, int64(fee_reserve), inv.Description,
		hash, tgMessageId, inv.Payee)
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

	// set common params
	params := eclair.Params{
		"invoice":         bolt11,
		"maxAttempts":     20,
		"feeThresholdSat": 5,
		"maxFeePct":       0.05,
		"externalId":      fmt.Sprintf("lntxbot:user=%d", u.Id),
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
	txn, err := pg.Beginx()
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
VALUES ($1, $2, $3, $4, $5, $6, $7)
    `, u.Id, targetId, msats, desc, hash, true, tgMessageId)
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

	txn, err := pg.Beginx()
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
		log.Print(err)
		return ErrDatabase
	}

	balance := getBalance(txn, u.Id)
	if balance < 0 {
		return ErrInsufficientBalance
	}

	err = txn.Commit()
	if err != nil {
		log.Print(err)
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
			Stringer("user", &u).Str("hash", hash).
			Msg("failed to save payment received.")
	}

	return
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
