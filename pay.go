package main

import (
	"fmt"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/docopt/docopt-go"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

func handlePay(u User, opts docopt.Opts, messageId int, replyToMessage *tgbotapi.Message) {
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
				return
			}
		}
		u.notify(t.NOINVOICE, nil)
		return
	} else {
		bolt11 = ibolt11.(string)
	}

	if askConfirmation {
		// decode invoice and show a button for confirmation
		inv, nodeAlias, err := decodeInvoice(bolt11)
		if err != nil {
			u.notify(t.FAILEDDECODE, t.T{"Err": messageFromError(err)})
			return
		}

		amount := int(inv.Get("msatoshi").Int())
		if amount == 0 {
			u.notify(t.ZEROAMOUNTINVOICE, nil)
			return
		}

		hash := inv.Get("payment_hash").String()
		text := translateTemplate(t.CONFIRMINVOICE, u.Locale, t.T{
			"Sats":  amount / 1000,
			"Desc":  escapeHTML(inv.Get("description").String()),
			"Hash":  hash,
			"Node":  nodeLink(inv.Get("payee").String()),
			"Alias": nodeAlias,
		})

		msgId := sendMessage(u.ChatId, text).MessageID

		hashfirstchars := hash[:5]
		rds.Set("payinvoice:"+hashfirstchars, bolt11, s.PayConfirmTimeout)

		editWithKeyboard(u.ChatId, msgId,
			text+"\n\n"+translate(t.ASKTOCONFIRM, u.Locale),
			tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(
						translate(t.CANCEL, u.Locale),
						fmt.Sprintf("cancel=%d", u.Id)),
					tgbotapi.NewInlineKeyboardButtonData(
						translate(t.YES, u.Locale),
						fmt.Sprintf("pay=%s", hashfirstchars)),
				),
			),
		)
	} else {
		err := u.payInvoice(messageId, bolt11)
		if err != nil {
			u.notifyAsReply(t.ERROR, t.T{"Err": err.Error()}, messageId)
		}
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
