package main

import (
	"fmt"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/tidwall/gjson"
)

func handle(upd tgbotapi.Update) {
	if upd.Message != nil {
		handleMessage(upd.Message)
	} else if upd.CallbackQuery != nil {
		handleCallback(upd.CallbackQuery)
	} else if upd.InlineQuery != nil {
		handleInlineQuery(upd.InlineQuery)
	} else if upd.EditedMessage != nil {
	}
}

func handleHelp(u User) {
	helpString := strings.Replace(s.Usage, "  c ", "  /", -1)
	u.notifyMarkdown("```\n" + helpString + "\n```")
}

func decodeNotifyBolt11(chatId int64, replyTo int, bolt11 string, optmsats int) (id int, text string, err error) {
	inv, err := decodeInvoice(bolt11)
	if err != nil {
		errMsg := messageFromError(err, "Failed to decode invoice")
		notify(chatId, errMsg)
		return
	}

	amount := int(inv.Get("msatoshi").Int())
	if amount == 0 {
		amount = optmsats
	}

	text = fmt.Sprintf(`
%d satoshis
<i>%s</i>
<b>Hash</b>: %s
<b>Node</b>: %s
        `,
		amount/1000,
		escapeHTML(inv.Get("description").String()),
		inv.Get("payment_hash").String(),
		inv.Get("payee").String(),
	)

	msg := notifyAsReply(chatId, text, replyTo)
	id = msg.MessageID
	return
}

func handleInvoicePaid(res gjson.Result) {
	index := res.Get("pay_index").Int()
	rds.Set("lastinvoiceindex", index, 0)

	label := res.Get("label").String()

	// use the label to get the user that created this invoice
	userId, _ := rds.Get("recinvoice:" + label + ":creator").Int64()
	u, err := loadUser(int(userId), 0)
	if err != nil {
		log.Warn().Err(err).
			Int64("userid", userId).Str("label", label).Int64("index", index).
			Msg("couldn't load user who created this invoice.")
		return
	}

	msats := res.Get("msatoshi_received").Int()
	desc := res.Get("description").String()
	hash := res.Get("payment_hash").String()

	// the preimage should be on redis
	preimage := rds.Get("recinvoice:" + label + ":preimage").String()

	err = u.paymentReceived(
		int(msats),
		desc,
		hash,
		preimage,
		label,
	)
	if err != nil {
		u.notify(
			"Payment received, but failed to save on database. Please report this issue: " + label + ".",
		)
	}

	u.notifyAsReply(
		fmt.Sprintf("Payment received: %d. /tx%s.", msats/1000, hash[:5]),
		messageIdFromLabel(label),
	)
}
