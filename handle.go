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

func decodeNotifyBolt11(
	chatId int64, replyTo int, bolt11 string, optmsats int) (id int, text, hash string, err error) {
	inv, nodeAlias, usd, err := decodeInvoice(bolt11)
	if err != nil {
		errMsg := messageFromError(err, "Failed to decode invoice")
		notify(chatId, errMsg)
		return
	}

	amount := int(inv.Get("msatoshi").Int())
	if amount == 0 {
		amount = optmsats
	}

	hash = inv.Get("payment_hash").String()

	text = fmt.Sprintf(`
%d sat (%s)
<i>%s</i>
<b>Hash</b>: %s
<b>Node</b>: %s (%s)
        `,
		amount/1000,
		usd,
		escapeHTML(inv.Get("description").String()),
		hash,
		inv.Get("payee").String(),
		nodeAlias,
	)

	msg := notifyAsReply(chatId, text, replyTo)
	id = msg.MessageID
	return
}

func handleInvoicePaid(res gjson.Result) {
	index := res.Get("pay_index").Int()
	rds.Set("lastinvoiceindex", index, 0)

	msats := res.Get("msatoshi_received").Int()
	desc := res.Get("description").String()
	hash := res.Get("payment_hash").String()
	label := res.Get("label").String()

	// extract user id and preimage from label
	messageId, userId, preimage, ok := parseLabel(label)
	u, err := loadUser(userId, 0)
	if !ok || err != nil {
		log.Warn().Err(err).
			Int("userid", userId).Str("label", label).Int64("index", index).
			Msg("failed to parse label for received payment or loading user")
		return
	}

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

	u.notifyAsReply(fmt.Sprintf("Payment received: %d. /tx%s.", msats/1000, hash[:5]), messageId)
}
