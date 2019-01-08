package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/skip2/go-qrcode"
	"github.com/tidwall/gjson"
)

func handle(upd tgbotapi.Update) {
	if upd.Message != nil {
		handleMessage(upd.Message)
	} else if upd.CallbackQuery != nil {
		handleCallback(upd.CallbackQuery)
	} else if upd.InlineQuery != nil {
	} else if upd.ChosenInlineResult != nil {
	} else if upd.EditedMessage != nil {
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
			break
		}
		var desc string
		if idesc, ok := opts["<description>"]; ok {
			desc = strings.Join(idesc.([]string), " ")
		}
		label := makeLabel(u.ChatId, message.MessageID)
		log.Debug().Str("label", label).Str("desc", desc).Int("amt", amount).
			Msg("generating invoice")

		// save invoice creator on redis
		rds.Set("invoicecreator:"+label, u.Id, s.InvoiceTimeout)

		// make invoice
		res, err := ln.Call("invoice", strconv.Itoa(amount*1000),
			label, desc, strconv.Itoa(int(s.InvoiceTimeout/time.Second)))
		if err != nil {
			u.notify("Failed to create invoice: " + err.Error())
			break
		}
		invoice := res.Get("bolt11").String()

		// generate qr code
		qrfilepath := filepath.Join(os.TempDir(), "lntxbot.invoice."+label+".png")
		err = qrcode.WriteFile(invoice, qrcode.Medium, 256, qrfilepath)
		if err != nil {
			log.Warn().Err(err).Str("invoice", invoice).
				Msg("failed to generate qr.")
		} else {
			defer os.Remove(qrfilepath)
			photo := tgbotapi.NewPhotoUpload(u.ChatId, qrfilepath)
			photo.Caption = invoice
			_, err := bot.Send(photo)
			if err != nil {
				log.Warn().Str("user", u.Username).Err(err).
					Msg("error sending photo")
			} else {
				// break so we don't send the invoice again
				break
			}
		}
		u.notify(invoice)

		break
	case opts["decode"].(bool):
		// just decode invoice
		bolt11 := opts["<invoice>"].(string)
		handleDecodeInvoice(u, bolt11, 0)
		break
	case opts["pay"].(bool):
		// pay invoice
		askConfirmation := true
		if opts["now"].(bool) {
			askConfirmation = false
		}

		bolt11 := opts["<invoice>"].(string)
		if bolt11 == "" {
			u.notify("Invoice not provided.")
			break
		}
		optsats, _ := opts.Int("<satoshis>")
		optmsats := optsats * 1000

		invlabel := makeLabel(u.ChatId, message.MessageID)

		if askConfirmation {
			// decode invoice and show a button for confirmation
			message := handleDecodeInvoice(u, bolt11, optmsats)

			rds.Set("invoice:"+invlabel, bolt11, s.PayConfirmTimeout)
			rds.Set("invoice:"+invlabel+":msats", optmsats, s.PayConfirmTimeout)

			bot.Send(tgbotapi.NewEditMessageText(u.ChatId, message.MessageID,
				message.Text+"\n\nPay the invoice described above?"))
			bot.Send(tgbotapi.NewEditMessageReplyMarkup(u.ChatId, message.MessageID,
				tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("No, cancel", "pay:no"),
						tgbotapi.NewInlineKeyboardButtonData("Yes", "pay:yes="+invlabel),
					),
				),
			))
		} else {
			handlePayInvoice(u, bolt11, invlabel, optmsats)
		}
	}
}

func handleCallback(cb *tgbotapi.CallbackQuery) {
	u, err := ensureUser(cb.From.ID, cb.From.UserName)
	if err != nil {
		log.Warn().Err(err).
			Str("username", cb.From.UserName).
			Int("id", cb.From.ID).
			Msg("failed to ensure user")
		return
	}

	if cb.Message != nil {
		// a confirmation from a /pay command
		log.Print(cb.Data)
		log.Print(cb.Message.Text)
		if cb.Data == "pay:no" {
			// cleanup the keyboard
			bot.Send(tgbotapi.NewEditMessageReplyMarkup(u.ChatId,
				cb.Message.MessageID,
				tgbotapi.NewInlineKeyboardMarkup(),
			))
			return
		} else if strings.HasPrefix(cb.Data, "pay:yes=") {
			invlabel := strings.Split(cb.Data, "=")[1]
			bolt11, err := rds.Get("invoice:" + invlabel).Result()
			if err != nil {
				u.notify("The payment confirmation button has expired.")
				return
			}
			optmsats, _ := rds.Get("invoice:" + invlabel + ":msats").Int64()
			handlePayInvoice(u, bolt11, invlabel, int(optmsats))
		}
	}
}

func handleDecodeInvoice(u User, bolt11 string, optmsats int) (_ tgbotapi.Message) {
	inv, err := decodeInvoice(bolt11)
	if err != nil {
		u.notify("Invalid bolt11: " + err.Error())
		return
	}

	amount := int(inv.Get("msatoshi").Int())
	if amount == 0 {
		amount = optmsats
	}

	return u.notify(
		fmt.Sprintf("[%s] \n%d millisatoshis. \nhash: %s.",
			inv.Get("description").String(),
			amount,
			inv.Get("payment_hash").String()),
	)
}

func handlePayInvoice(u User, bolt11, invlabel string, optmsats int) {
	pay, errMsg, err := u.payInvoice(bolt11, invlabel, optmsats)
	if err != nil {
		log.Warn().Err(err).
			Str("user", u.Username).
			Str("bolt11", bolt11).
			Str("label", invlabel).
			Msg("couldn't pay invoice")
		u.notify("Failed to pay: " + errMsg)
		return
	}

	log.Print("paid: " + pay.String())

	u.notify(fmt.Sprintf(
		"Paid with %d satoshis (+ %f fee). \n\nHash: %s\n\nProof: %s",
		int(pay.Get("msatoshi").Float()/1000),
		pay.Get("msatoshi_sent").Float()-pay.Get("msatoshi").Float(),
		pay.Get("payment_hash").String(),
		pay.Get("payment_preimage").String(),
	))
}

func handleInvoicePaid(res gjson.Result) {
	index := res.Get("pay_index").Int()
	rds.Set("lastinvoiceindex", index, 0)

	label := res.Get("label").String()

	// use the label to get the user that created this invoice
	userId, _ := rds.Get("invoicecreator:" + label).Int64()
	u, err := loadUser(int(userId))
	if err != nil {
		log.Warn().Err(err).
			Int64("userid", userId).Str("label", label).Int64("index", index).
			Msg("couldn't load user who created this invoice.")
		return
	}

	amount := res.Get("msatoshi_received").Int()
	desc := res.Get("description").String()
	hash := res.Get("payment_hash").String()
	bolt11 := res.Get("bolt11").String()

	balance, err := u.paymentReceived(
		int(amount),
		desc,
		bolt11,
		hash,
		label,
	)
	if err != nil {
		u.notify(
			"Payment received, but failed to save on database. Please report this issue: " + label + ".",
		)
	}

	if desc != "" {
		desc = " " + desc
	}

	chattable := tgbotapi.NewMessage(
		u.ChatId,
		fmt.Sprintf("Payment received: %d%s. \n\nhash: %s \n\nYour balance is now %d satoshis.", amount/1000, desc, hash, balance/1000),
	)
	chattable.BaseChat.ReplyToMessageID = messageIdFromLabel(label)
	_, err = bot.Send(chattable)
	if err != nil {
		log.Warn().Str("user", u.Username).Err(err).Msg("error sending message")
	}
}
