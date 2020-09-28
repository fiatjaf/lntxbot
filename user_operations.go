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
	lightning "github.com/fiatjaf/lightningd-gjson-rpc"
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

type makeInvoiceArgs struct {
	Desc                   string
	DescHash               string
	Msatoshi               int64
	Expiry                 *time.Duration
	Tag                    string
	Extra                  map[string]interface{}
	BlueWallet             bool
	IgnoreInvoiceSizeLimit bool
}

func (u User) makeInvoice(
	ctx context.Context,
	args makeInvoiceArgs,
) (bolt11 string, hash string, err error) {
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
								return "", "", fmt.Errorf("The issuance of invoices smaller than %dmsat is restricted to %d per day.", limit.EqualOrSmallerThan, limit.PerDay)
							}
						}
					}
				}
			}
		}
	}

	log.Debug().Stringer("user", &u).Str("desc", args.Desc).Int64("msats", msatoshi).
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

	extra := args.Extra
	if extra == nil {
		extra = make(map[string]interface{})
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

	shadowData := ShadowChannelData{
		UserId:    u.Id,
		Origin:    ctx.Value("origin").(string),
		MessageId: messageId,
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
			return "", "", fmt.Errorf("invalid description_hash: %w", err)
		}
		shadowData.DescriptionHash = args.DescHash
	}

	// derive custom private key for this user
	seedhash := sha256.Sum256(
		[]byte(fmt.Sprintf("invoicekeyseed:%d:%s", u.Id, s.TelegramBotToken)))
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
		return "", "", fmt.Errorf("error making invoice: %w", err)
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

	return bolt11, hash, nil
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
			goto externalinvoice
		}

		err = u.addInternalPendingInvoice(
			ctx,
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

		go onInvoicePaid(ctx, hash, shadowData)
		go resolveWaitingInvoice(hash, inv)
		go paymentHasSucceeded(ctx, u, float64(amount), float64(amount),
			shadowData.Preimage, shadowData.Tag, hash)
	}

externalinvoice:
	// it's an invoice from elsewhere, continue and
	// actually send the lightning payment

	err = u.actuallySendExternalPayment(ctx, bolt11, inv, amount,
		paymentHasSucceeded, paymentHasFailed)
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
	onSuccess func(
		ctx context.Context,
		u User,
		msatoshi float64,
		msatoshi_sent float64,
		preimage string,
		tag string,
		hash string,
	),
	onFailure func(
		ctx context.Context,
		u User,
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

	// if not tg this will just be ignored
	// TODO maybe remove trigger_message from the database
	var tgMessageId int
	if message := ctx.Value("message"); message != nil {
		if m, ok := message.(*tgbotapi.Message); ok {
			tgMessageId = m.MessageID
		}
	}

	maxfeepercent := 0.5
	exemptfee := 5000
	fee_reserve := float64(msatoshi) * 0.005
	if msatoshi < 1000000 {
		fee_reserve += 5000 // account for exemptfee
	}

	_, err = txn.Exec(`
INSERT INTO lightning.transaction
  (from_id, amount, fees, description, payment_hash, pending,
   trigger_message, remote_node)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `, u.Id, msatoshi, int64(fee_reserve), inv.Description,
		hash, true, tgMessageId, inv.Payee)
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
	params := map[string]interface{}{
		"bolt11":        bolt11,
		"riskfactor":    3,
		"maxfeepercent": maxfeepercent,
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
				ctx,
				u,
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
				log.Warn().Stringer("user", &u).
					Interface("params", params).Interface("tries", tries).
					Str("bolt11", bolt11).Str("hash", hash).
					Msg("payment failed according to listpays")
					// give the money back to the user
				onFailure(ctx, u, hash)
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
			log.Warn().Stringer("user", &u).
				Interface("params", params).Interface("tries", tries).Str("hash", hash).
				Msg("payment wasn't even tried")
				// give the money back to the user
			onFailure(ctx, u, hash)
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
	msats int,
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
    `, u.Id, target.Id, anonymous, msats, descn, tagn, hashn, tgMessageId)
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

func paymentHasSucceeded(
	ctx context.Context,
	u User,
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

	// TODO some fancy reaction for discord

	_, err = pg.Exec(`
UPDATE lightning.transaction
SET fees = $1, preimage = $2, pending = false, tag = $4
WHERE payment_hash = $3
    `, fees, preimage, hash, tagn)
	if err != nil {
		log.Error().Err(err).Stringer("user", &u).Str("hash", hash).
			Float64("fees", fees).Msg("failed to update transaction fees.")
		send(ctx, u, t.DBERROR, nil, ctx.Value("message"))
	}

	send(ctx, u, t.PAIDMESSAGE, t.T{
		"Sats":      float64(msatoshi) / 1000,
		"Fee":       fees / 1000,
		"Hash":      hash,
		"Preimage":  preimage,
		"ShortHash": hash[:5],
	}, ctx.Value("message"))
}

func paymentHasFailed(ctx context.Context, u User, hash string) {
	send(ctx, u, t.PAYMENTFAILED, t.T{"ShortHash": hash[:5]}, ctx.Value("message"))

	_, err := pg.Exec(
		`DELETE FROM lightning.transaction WHERE payment_hash = $1`, hash)
	if err != nil {
		log.Error().Err(err).Str("hash", hash).
			Msg("failed to cancel transaction after routing failure")
	}
}
