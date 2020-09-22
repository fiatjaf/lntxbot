package main

import (
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

func handlePay(
	u User,
	opts docopt.Opts,
	messageId int,
	replyToMessage *tgbotapi.Message,
) error {
	// pay invoice flow
	askConfirmation := true
	if opts["now"].(bool) {
		askConfirmation = false
	}

	var bolt11 string
	// when paying, the invoice could be in the message this is replying to
	if ibolt11, ok := opts["<invoice>"]; !ok || ibolt11 == nil {
		if replyToMessage != nil {
			bolt11, _, ok = searchForInvoice(u, *replyToMessage)
			if !ok || bolt11 == "" {
				handleHelp(u, "pay")
				return errors.New("invalid invoice")
			}
		}
		handleHelp(u, "pay")
		return errors.New("invalid invoice")
	} else {
		bolt11 = ibolt11.(string)
	}

	// decode invoice
	inv, err := decodepay.Decodepay(bolt11)
	if err != nil {
		u.notify(t.FAILEDDECODE, t.T{"Err": messageFromError(err)})
		return err
	}

	hash := inv.PaymentHash
	amount := float64(inv.MSatoshi)

	go u.track("pay", map[string]interface{}{
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
			"IsDiscord": u.isDiscord(),
		}

		if u.isDiscord() {
			if amount == 0 {
				u.notify(t.ERROR, t.T{"Err": "Amountless invoice, use `/paynow &lt;invoice&gt; &lt;amount&gt;`"})
				return errors.New("paying amountless on discord")
			}

			var messageId string
			switch v := u.notify(t.PAYPROMPT, payTmplParams).(type) {
			case DiscordMessage:
				messageId = v.MessageID
			case string:
				messageId = v
			}

			rds.Set(
				fmt.Sprintf("reaction-confirm:%s:%s", u.DiscordId, messageId),
				bolt11, time.Minute*15)
			return nil
		}

		if amount == 0 {
			// zero-amount invoices, prompt the user to reply with the desired amount
			chattable := tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:           u.TelegramChatId,
					ReplyToMessageID: messageId,
					ReplyMarkup:      tgbotapi.ForceReply{ForceReply: true},
				},
				ParseMode:             "HTML",
				DisableWebPagePreview: true,
				Text:                  translateTemplate(t.PAYPROMPT, u.Locale, payTmplParams),
			}
			sent, err := tgsend(chattable)
			if err != nil {
				log.Warn().Err(err).Msg("error sending pay prompt amountless")
				return err
			}
			data, _ := json.Marshal(struct {
				Type   string `json:"type"`
				Bolt11 string `json:"bolt11"`
			}{"pay", bolt11})
			rds.Set(fmt.Sprintf("reply:%d:%d", u.Id, sent.MessageID), data, time.Minute*15)
			return nil
		}

		// normal invoice, ask for confirmation
		hashfirstchars := hash[:5]
		rds.Set("payinvoice:"+hashfirstchars, bolt11, s.PayConfirmTimeout)
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					translate(t.CANCEL, u.Locale),
					fmt.Sprintf("cancel=%d", u.Id)),
				tgbotapi.NewInlineKeyboardButtonData(
					translateTemplate(t.PAYAMOUNT, u.Locale, t.T{"Sats": amount / 1000}),
					fmt.Sprintf("pay=%s", hashfirstchars)),
			),
		)

		u.notifyWithKeyboard(t.PAYPROMPT, payTmplParams, &keyboard, 0)
	} else {
		amountToPay, _ := opts.Int("<satoshis>")
		_, err := u.payInvoice(messageId, bolt11, int64(amountToPay)*1000)
		if err != nil {
			u.notifyAsReply(t.ERROR, t.T{"Err": err.Error()}, messageId)
			return err
		}
	}

	return nil
}

func handlePayReactionConfirm(reaction *discordgo.MessageReaction) {
	key := fmt.Sprintf("reaction-confirm:%s:%s", reaction.UserID, reaction.MessageID)
	bolt11, err := rds.Get(key).Result()
	if err != nil {
		return
	}

	// there is a bolt11 invoice, therefore this is actually a confirmation
	// and it comes from the correct user
	u, err := loadDiscordUser(reaction.UserID)
	if err != nil {
		log.Warn().Err(err).Str("id", reaction.UserID).Msg("failed to load discord user")
		return
	}

	_, err = u.payInvoice(reaction.MessageID, bolt11, 0)
	if err == nil {
		inv, _ := decodepay.Decodepay(bolt11)
		hashfirstchars := inv.PaymentHash[0:5]

		appendToDiscordMessage(reaction.ChannelID, reaction.MessageID,
			translateTemplate(t.CALLBACKATTEMPT, u.Locale, t.T{
				"Hash": hashfirstchars,
			}))
		discord.MessageReactionAdd(reaction.ChannelID, reaction.MessageID, "✅")
	} else {
		appendToDiscordMessage(reaction.ChannelID, reaction.MessageID, err.Error())
		discord.MessageReactionAdd(reaction.ChannelID, reaction.MessageID, "❌")
	}
}

func handlePayCallback(u User, messageId int, locale string, cb *tgbotapi.CallbackQuery) {
	defer removeKeyboardButtons(cb)

	hashfirstchars := cb.Data[4:]
	bolt11, err := rds.Get("payinvoice:" + hashfirstchars).Result()
	if err != nil {
		bot.AnswerCallbackQuery(
			tgbotapi.NewCallback(
				cb.ID,
				translate(t.CALLBACKEXPIRED, locale),
			),
		)
		return
	}

	bot.AnswerCallbackQuery(
		tgbotapi.NewCallback(cb.ID, translate(t.CALLBACKSENDING, locale)))

	_, err = u.payInvoice(messageId, bolt11, 0)
	if err == nil {
		appendToTelegramMessage(cb, translateTemplate(t.CALLBACKATTEMPT, locale, t.T{
			"Hash": hashfirstchars,
		}))
	} else {
		appendToTelegramMessage(cb, err.Error())
	}

	go u.track("pay confirm", map[string]interface{}{"amountless": false})
}

func handlePayVariableAmount(u User, msatoshi int64, data gjson.Result, messageId int) {
	bolt11 := data.Get("bolt11").String()
	_, err := u.payInvoice(messageId, bolt11, msatoshi)
	if err != nil {
		u.notifyAsReply(t.ERROR, t.T{"Err": err.Error()}, messageId)
		return
	}

	go u.track("pay confirm", map[string]interface{}{
		"amountless": true,
		"sats":       msatoshi / 1000,
	})
}
