package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/go-cliche"
	decodepay "github.com/fiatjaf/ln-decodepay"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func handlePay(ctx context.Context, payer User, opts docopt.Opts) error {
	// pay invoice flow
	askConfirmation := true
	if opts["now"].(bool) {
		askConfirmation = false
	}

	bolt11, _ := opts.String("<invoice>")

	// decode invoice
	inv, err := decodepay.Decodepay(bolt11)
	if err != nil {
		send(ctx, payer, t.FAILEDDECODE, t.T{"Err": err.Error()})
		return err
	}

	hash := inv.PaymentHash
	amount := float64(inv.MSatoshi)

	go payer.track("pay", map[string]interface{}{
		"prompt":     askConfirmation,
		"sats":       amount,
		"amountless": amount == 0,
	})

	if askConfirmation {
		// show a button for confirmation
		payTmplParams := t.T{
			"Sats":            amount / 1000,
			"Description":     escapeHTML(inv.Description),
			"DescriptionHash": escapeHTML(inv.DescriptionHash),
			"Hash":            hash,
			"ReceiverName":    extractNameFromDesc(inv.Description),
			"Payee":           inv.Payee,
			"Created": time.Unix(int64(inv.CreatedAt), 0).
				Format("Mon Jan 2 15:04"),
			"Expiry": time.Unix(int64(inv.CreatedAt+inv.Expiry), 0).
				Format("Mon Jan 2 15:04"),
			"Expired": time.Unix(int64(inv.CreatedAt+inv.Expiry), 0).
				Before(time.Now()),
			"Currency": inv.Currency,
			"Hints":    inv.Route,
		}

		if amount == 0 {
			// zero-amount invoice, prompt the user to reply with the desired amount
			sent := send(ctx, ctx.Value("message"),
				&tgbotapi.ForceReply{ForceReply: true},
				t.PAYPROMPT, payTmplParams)
			if sent == nil {
				return nil
			}

			sentId, _ := sent.(int)
			data, _ := json.Marshal(struct {
				Type   string `json:"type"`
				Bolt11 string `json:"bolt11"`
			}{"pay", bolt11})
			rds.Set(fmt.Sprintf("reply:%d:%d", payer.Id, sentId), data, time.Minute*15)
			return nil
		}

		// normal invoice, ask for confirmation
		hashfirstchars := hash[:5]
		rds.Set("payinvoice:"+hashfirstchars, bolt11, s.PayConfirmTimeout)
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					translate(ctx, t.CANCEL),
					fmt.Sprintf("cancel=%d", payer.Id)),
				tgbotapi.NewInlineKeyboardButtonData(
					translateTemplate(ctx, t.PAYAMOUNT, t.T{"Sats": amount / 1000}),
					fmt.Sprintf("pay=%s", hashfirstchars)),
			),
		)

		send(ctx, t.PAYPROMPT, payTmplParams, &keyboard)
	} else {
		// send an "attempting" message
		send(ctx, t.CALLBACKATTEMPT, t.T{"Hash": hash[:5]}, ctx.Value("message"))

		// parse manually specified satoshis if any
		amountToPay, _ := opts.Int("<satoshis>")

		// proceed to pay
		_, err := payer.payInvoice(ctx, bolt11, int64(amountToPay)*1000)
		if err != nil {
			send(ctx, payer, t.ERROR, t.T{"Err": err.Error()}, ctx.Value("message"))
			return err
		}
	}

	return nil
}

func handlePayCallback(ctx context.Context) {
	u := ctx.Value("initiator").(User)
	go u.track("pay confirm", map[string]interface{}{"amountless": false})

	defer removeKeyboardButtons(ctx)
	hashfirstchars := ctx.Value("callbackQuery").(*tgbotapi.CallbackQuery).Data[4:]
	bolt11, err := rds.Get("payinvoice:" + hashfirstchars).Result()
	if err != nil {
		send(ctx, t.CALLBACKEXPIRED)
		return
	}

	send(ctx, t.CALLBACKSENDING)

	_, err = u.payInvoice(ctx, bolt11, 0)
	cb := ctx.Value("callbackQuery").(*tgbotapi.CallbackQuery)
	if err == nil {
		send(ctx, u, t.CALLBACKATTEMPT, t.T{"Hash": hashfirstchars}, cb.Message.MessageID)
	} else {
		send(ctx, u, err.Error(), cb.Message.MessageID)
	}
}

func handlePayVariableAmount(ctx context.Context, msatoshi int64, raw string) {
	u := ctx.Value("initiator").(User)

	var data struct {
		Invoice string `json:"bolt11"`
	}
	json.Unmarshal([]byte(raw), &data)

	_, err := u.payInvoice(ctx, data.Invoice, msatoshi)
	if err != nil {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()}, ctx.Value("message"))
		return
	}

	go u.track("pay confirm", map[string]interface{}{
		"amountless": true,
		"sats":       msatoshi / 1000,
	})
}

func waitPaymentSuccess(hash string) (preimage <-chan string) {
	wait := make(chan string)
	waitingPaymentSuccesses.Upsert(hash, wait,
		func(exists bool, arr interface{}, v interface{}) interface{} {
			if exists {
				return append(arr.([]interface{}), v)
			} else {
				return []interface{}{v}
			}
		},
	)
	return wait
}

func resolveWaitingPaymentSuccess(hash string, preimage string) {
	if chans, ok := waitingPaymentSuccesses.Get(hash); ok {
		for _, ch := range chans.([]interface{}) {
			select {
			case ch.(chan string) <- preimage:
			default:
			}
		}
		waitingPaymentSuccesses.Remove(hash)
	}
}

func paymentHasSucceeded(
	ctx context.Context,
	msatoshi int64,
	feesPaid int64,
	preimage string,
	tag string,
	hash string,
) {
	// if it succeeds we mark the transaction as not pending anymore
	// plus save fees and preimage
	if feesPaid < int64(float64(msatoshi)*0.003) {
		feesPaid = int64(float64(msatoshi) * 0.003)
	}

	// if there's a tag we save that too, otherwise leave it null
	tagn := sql.NullString{String: tag, Valid: tag != ""}

	var res struct {
		UserId         int `db:"from_id"`
		TriggerMessage int `db:"trigger_message"`
	}
	err := pg.Get(&res, `
UPDATE lightning.transaction
SET fees = $1, preimage = $2, pending = false, tag = $4
WHERE payment_hash = $3 AND pending
RETURNING from_id, trigger_message
    `, feesPaid, preimage, hash, tagn)
	if err != nil {
		log.Error().Err(err).Str("hash", hash).
			Int64("fees", feesPaid).Msg("failed to update transaction paid status")
		return
	}

	go resolveWaitingPaymentSuccess(hash, preimage)

	user, err := loadUser(res.UserId)
	if err != nil {
		log.Error().Err(err).Int("id", res.UserId).Msg("no user with id on pay success")
		return
	}

	go user.track("payment sent", map[string]interface{}{
		"sats": msatoshi / 1000,
	})

	send(ctx, user, res.TriggerMessage, t.PAIDMESSAGE, t.T{
		"Sats":      float64(msatoshi) / 1000,
		"Fee":       feesPaid / 1000,
		"Hash":      hash,
		"Preimage":  preimage,
		"ShortHash": hash[:5],
	}, ctx.Value("message"))
}

func paymentHasFailed(ctx context.Context, hash string, failures []string) {
	var res struct {
		UserId         int `db:"from_id"`
		TriggerMessage int `db:"trigger_message"`
	}
	err := pg.Get(&res, `
DELETE FROM lightning.transaction
WHERE payment_hash = $1 AND to_id IS NULL
RETURNING from_id, trigger_message
    `, hash)
	if err != nil {
		log.Error().Err(err).Str("hash", hash).
			Msg("failed to cancel transaction after routing failure")
		return
	}

	rds.Set("hash:"+strconv.Itoa(res.UserId)+":"+hash[0:5], hash, time.Hour*24*2)

	user, err := loadUser(res.UserId)
	if err != nil {
		log.Error().Err(err).Str("hash", hash).Int("id", res.UserId).
			Msg("failed to load user after routing failure")
		return
	}

	send(ctx, user, res.TriggerMessage,
		t.PAYMENTFAILED, t.T{"Hash": hash, "FailureString": strings.Join(failures, "\n")},
		ctx.Value("message"))
}

func checkOutgoingPayment(ctx context.Context, hash string) {
	info, err := ln.CheckPayment(hash)
	if err != nil {
		if strings.Contains(err.Error(),
			fmt.Sprintf("couldn't get payment '%s' from database", hash),
		) {
			// if it's not on cliche's database means it has failed, right?
			// make sure we only do this check for recent payments otherwise we could be
			//   checking for stuff from other node backends
			info = cliche.CheckPaymentResult{
				PaymentInfo: cliche.PaymentInfo{Status: "failed"},
			}
		} else {
			log.Error().Err(err).Str("hash", hash).Msg("failed to check-payment")
			return
		}
	}
	if info.IsIncoming {
		log.Error().Err(err).Str("hash", hash).
			Msg("tried to check outgoing with an incoming invoice")
		return
	}

	switch info.Status {
	case "complete":
		go paymentHasSucceeded(
			ctx,
			info.Msatoshi,
			info.FeeMsatoshi,
			info.Preimage,
			"",
			hash,
		)
	case "failed":
		// check if this transaction is old enough first
		var t time.Time
		err := pg.Get(&t,
			"SELECT time FROM lightning.transaction WHERE payment_hash = $1", hash)
		if err == nil &&
			t.Before(time.Now().Add(-2*time.Hour)) &&
			t.After(time.Now().AddDate(0, -3, 0)) {
			log.Warn().Str("hash", hash).Time("time", t).
				Msg("tx in the range of acceptable cancellation on getsentinfo []")

			go paymentHasFailed(ctx, hash, []string{})
		} else {
			log.Warn().Str("hash", hash).Err(err).Time("time", t).
				Msg("check-invoice says it's failed, but we can't cancel this transaction because it's too new")
		}
	}
}
