package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/lucsky/cuid"
	"github.com/skip2/go-qrcode"
)

func handle(upd tgbotapi.Update) {
	if upd.Message.MessageID > 0 {
		// it's a message
		handleMessage(upd.Message)
	}
}

func handleMessage(message *tgbotapi.Message) {
	u, err := ensureUser(message.From.ID, message.From.UserName)
	if err != nil {
		log.Warn().Err(err).
			Str("username", message.From.UserName).
			Int("id", message.From.ID).
			Msg("failed to ensure user")
		return
	}

	opts, proceed, err := parse(message.Text)
	if !proceed {
		return
	}
	if err != nil {
		log.Warn().Err(err).Str("command", message.Text).
			Msg("Failed to parse command")
		u.notify("Could not understand the command.")
		return
	}

	switch {
	case opts["start"].(bool):
		// create user
		if message.Chat.Type == "private" {
			u.setChat(message.Chat.ID)
		}

		u.notify("Account created successfully.")
		break
	case opts["receive"].(bool):
		amount, err := opts.Int("<amount>")
		if err != nil {
			u.notify("Invalid amount: " + opts["<amount>"].(string))
			return
		}
		var desc string
		if idesc, ok := opts["<description>"]; ok {
			desc = strings.Join(idesc.([]string), " ")
		}
		label := "lntxbot." + cuid.Slug()
		log.Debug().Str("label", label).Str("desc", desc).Int("amt", amount).
			Msg("generating invoice")

		// make invoice
		res, err := ln.Call("invoice", strconv.Itoa(amount*1000),
			label, desc, strconv.Itoa(60*60*5))
		if err != nil {
			u.notify("Failed to create invoice: " + err.Error())
			return
		}
		invoice := res.Get("bolt11").String()

		// generate qr code
		qrfilepath := filepath.Join(os.TempDir(), "lntxbot.invoice."+label+".png")
		err = qrcode.WriteFile(invoice, qrcode.Medium, 256, qrfilepath)
		if err != nil {
			log.Warn().Err(err).Str("invoice", invoice).
				Msg("failed to generate qr.")
		} else {
			// defer os.Remove(qrfilepath)
			u.sendImage(qrfilepath)
		}
		u.notify(invoice)

		break
	case opts["decode"].(bool):
		// just decode invoice
		invoice := opts["<invoice>"].(string)
		handleDecodeInvoice(u, invoice)
		break
	case opts["pay"].(bool):
		// pay invoice
		askConfirmation := true
		if opts["now"].(bool) {
			askConfirmation = false
		}

		invoice := opts["<invoice>"].(string)
		if invoice == "" {
			u.notify("Invoice not provided.")
			return
		}

		if askConfirmation {
			// decode invoice and show a button for confirmation
			handleDecodeInvoice(u, invoice)

			// TODO
		} else {
			err = u.payInvoice(invoice)
			if err != nil {
				u.notify("failed to pay")
				return
			}

			u.notify("paid invoice " + invoice)
		}
	}
}

func handleDecodeInvoice(u User, invoice string) {
	inv, err := decodeInvoice(invoice)
	if err != nil {
		u.notify("Invalid invoice: " + err.Error())
		return
	}

	u.notify(
		fmt.Sprintf("[%s] %d millisatoshi, hash: %s.",
			inv.Get("description").String(),
			inv.Get("msatoshi").Int(),
			inv.Get("payment_hash").String()),
	)
}
