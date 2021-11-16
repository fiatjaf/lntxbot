package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/eclair-go"
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
			"Currency":  inv.Currency,
			"Hints":     inv.Route,
			"IsDiscord": ctx.Value("origin").(string) == "discord",
		}

		if ctx.Value("origin").(string) == "discord" {
			if amount == 0 {
				send(ctx, t.ERROR, t.T{"Err": "Amountless invoice. Use `/paynow &lt;invoice&gt; &lt;amount&gt;`"})
				return errors.New("paying amountless on discord")
			}

			sentId := send(ctx, t.PAYPROMPT, payTmplParams)
			if sentId == nil {
				return errors.New("error sending prompt to discord")
			}

			mid := sentId.(DiscordMessageID).Message()
			err = rds.Set(
				fmt.Sprintf("reaction-confirm:%s:%s", payer.DiscordId, mid),
				bolt11, time.Minute*15).Err()

			return err
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

func handlePayReactionConfirm(ctx context.Context, reaction *discordgo.MessageReaction) {
	key := fmt.Sprintf("reaction-confirm:%s:%s", reaction.UserID, reaction.MessageID)
	bolt11, err := rds.Get(key).Result()
	if err != nil {
		log.Warn().Err(err).Str("key", key).Msg("couldn't load payment details")
		return
	}

	// there is a bolt11 invoice, therefore this is actually a confirmation
	// and it comes from the correct user
	u, err := loadDiscordUser(reaction.UserID)
	if err != nil {
		log.Warn().Err(err).Str("id", reaction.UserID).
			Msg("failed to load discord user")
		return
	}

	messageRef := discordIDFromReaction(reaction)

	_, err = u.payInvoice(ctx, bolt11, 0)
	if err == nil {
		inv, _ := decodepay.Decodepay(bolt11)
		hashfirstchars := inv.PaymentHash[0:5]

		send(ctx, messageRef, t.CALLBACKATTEMPT, t.T{"Hash": hashfirstchars})
		send(ctx, messageRef, "✅")
	} else {
		send(ctx, messageRef, t.ERROR, t.T{"Err": err.Error()})
		send(ctx, messageRef, "❌")
	}
}

func handlePayCallback(ctx context.Context) {
	u := ctx.Value("initiator").(User)

	defer removeKeyboardButtons(ctx)
	hashfirstchars := ctx.Value("callbackQuery").(*tgbotapi.CallbackQuery).Data[4:]
	bolt11, err := rds.Get("payinvoice:" + hashfirstchars).Result()
	if err != nil {
		send(ctx, t.CALLBACKEXPIRED)
		return
	}

	send(ctx, t.CALLBACKSENDING)

	_, err = u.payInvoice(ctx, bolt11, 0)
	if err == nil {
		send(ctx, t.CALLBACKATTEMPT, t.T{"Hash": hashfirstchars}, APPEND)
	} else {
		send(ctx, err.Error(), APPEND)
	}

	go u.track("pay confirm", map[string]interface{}{"amountless": false})
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

func paymentHasFailed(ctx context.Context, hash string) {
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
		t.PAYMENTFAILED, t.T{"ShortHash": hash[:5]}, ctx.Value("message"))
}

func checkOutgoingPayment(ctx context.Context, hash string) {
	info, err := ln.Call("getsentinfo", eclair.Params{"paymentHash": hash})
	if err != nil {
		log.Error().Err(err).Str("hash", hash).Msg("failed to getsentinfo")
		return
	}

	if info.Get("#").Int() == 0 {
		// check if this transaction is too old
		var t time.Time
		err := pg.Get(&t,
			"SELECT time FROM lightning.transaction WHERE payment_hash = $1", hash)
		if err == nil &&
			t.Before(time.Now().Add(-time.Hour)) &&
			t.After(time.Now().AddDate(0, -3, 0)) {
			log.Warn().Str("hash", hash).Time("time", t).
				Msg("tx in the range of acceptable cancellation on getsentinfo []")

			go paymentHasFailed(ctx, hash)
		} else {
			log.Warn().Str("hash", hash).Err(err).Time("time", t).
				Msg("getsentinfo returned []")
		}

		return
	}

	failed := true
	for _, attempt := range info.Array() {
		log.Print(hash, " ", attempt.Get("status.type").String())

		switch attempt.Get("status.type").String() {
		case "sent":
			go paymentHasSucceeded(
				ctx,
				info.Get("recipientAmount").Int(),
				info.Get("status.feesPaid").Int(),
				info.Get("status.paymentPreimage").String(),
				"",
				hash,
			)

			// end it here
			return
		case "failed":
			// this one failed, but what about the others?
		case "pending":
			failed = false
		default:
			// what is this?
			failed = false
		}
	}

	if failed {
		go paymentHasFailed(ctx, hash)
		return
	}
}
