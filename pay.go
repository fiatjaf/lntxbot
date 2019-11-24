package main

import (
	"errors"
	"fmt"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/ln-decodepay/gjson"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

func handlePay(u User, opts docopt.Opts, messageId int, replyToMessage *tgbotapi.Message) (paid bool, err error) {
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
				u.notify(t.NOINVOICE, nil)
				return false, errors.New("invalid invoice")
			}
		}
		u.notify(t.NOINVOICE, nil)
		return false, errors.New("invalid invoice")
	} else {
		bolt11 = ibolt11.(string)
	}

	if askConfirmation {
		// decode invoice and show a button for confirmation
		inv, err := decodepay_gjson.Decodepay(bolt11)
		if err != nil {
			u.notify(t.FAILEDDECODE, t.T{"Err": messageFromError(err)})
			return false, err
		}
		if inv.Get("code").Int() != 0 {
			u.notify(t.FAILEDDECODE, t.T{"Err": inv.Get("message").String()})
			return false, err
		}

		nodeAlias := getNodeAlias(inv.Get("payee").String())

		amount := int(inv.Get("msatoshi").Int())
		if amount == 0 {
			u.notify(t.ZEROAMOUNTINVOICE, nil)
			return false, err
		}

		hash := inv.Get("payment_hash").String()
		hashfirstchars := hash[:5]
		rds.Set("payinvoice:"+hashfirstchars, bolt11, s.PayConfirmTimeout)
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					translate(t.CANCEL, u.Locale),
					fmt.Sprintf("cancel=%d", u.Id)),
				tgbotapi.NewInlineKeyboardButtonData(
					translate(t.YES, u.Locale),
					fmt.Sprintf("pay=%s", hashfirstchars)),
			),
		)

		u.notifyWithKeyboard(t.CONFIRMINVOICE, t.T{
			"Sats":  amount / 1000,
			"Desc":  escapeHTML(inv.Get("description").String()),
			"Hash":  hash,
			"Node":  nodeLink(inv.Get("payee").String()),
			"Alias": nodeAlias,
		}, &keyboard, 0)
		return false, nil
	} else {
		err := u.payInvoice(messageId, bolt11)
		if err != nil {
			u.notifyAsReply(t.ERROR, t.T{"Err": err.Error()}, messageId)
			return false, err
		}
		return true, nil
	}
}

func handlePayCallback(u User, messageId int, locale string, cb *tgbotapi.CallbackQuery) {
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

	bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, translate(t.CALLBACKSENDING, locale)))

	err = u.payInvoice(messageId, bolt11)
	if err == nil {
		appendTextToMessage(cb, translate(t.CALLBACKATTEMPT, locale))
	} else {
		appendTextToMessage(cb, err.Error())
	}
	removeKeyboardButtons(cb)
}
