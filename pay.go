package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/docopt/docopt-go"
	decodepay "github.com/fiatjaf/ln-decodepay"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/tidwall/gjson"
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
		send(ctx, payer, t.FAILEDDECODE, t.T{"Err": messageFromError(err)})
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

func handlePayVariableAmount(ctx context.Context, msatoshi int64, data gjson.Result) {
	u := ctx.Value("initiator").(User)

	bolt11 := data.Get("bolt11").String()
	_, err := u.payInvoice(ctx, bolt11, msatoshi)
	if err != nil {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()}, ctx.Value("message"))
		return
	}

	go u.track("pay confirm", map[string]interface{}{
		"amountless": true,
		"sats":       msatoshi / 1000,
	})
}
