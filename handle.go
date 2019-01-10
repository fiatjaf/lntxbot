package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/kballard/go-shellquote"
	"github.com/tidwall/gjson"
)

func handle(upd tgbotapi.Update) {
	if upd.Message != nil {
		handleMessage(upd.Message)
	} else if upd.CallbackQuery != nil {
		handleCallback(upd.CallbackQuery)
	} else if upd.InlineQuery != nil {
		handleInlineQuery(upd.InlineQuery)
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

	log.Debug().Str("t", message.Text).Msg("got message")

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
		sats, err := opts.Int("<amount>")
		if err != nil {
			u.notify("Invalid amount: " + opts["<amount>"].(string))
			break
		}
		var desc string
		if idesc, ok := opts["<description>"]; ok {
			desc = strings.Join(idesc.([]string), " ")
		}

		label := makeLabel(u.ChatId, message.MessageID)

		bolt11, qrpath, err := makeInvoice(u, label, sats, desc)
		if err != nil {
			u.notify("Failed to generate invoice.")
			return
		}

		if qrpath == "" {
			u.notify(bolt11)
		} else {
			defer os.Remove(qrpath)
			photo := tgbotapi.NewPhotoUpload(u.ChatId, qrpath)
			photo.Caption = bolt11
			_, err := bot.Send(photo)
			if err != nil {
				log.Warn().Str("user", u.Username).Err(err).
					Msg("error sending photo")

					// send just the bolt11
				u.notify(bolt11)
			}
		}

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

			rds.Set("payinvoice:"+invlabel, bolt11, s.PayConfirmTimeout)
			rds.Set("payinvoice:"+invlabel+":msats", optmsats, s.PayConfirmTimeout)

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
	case opts["help"].(bool):
		u.notify(strings.Replace(USAGE, "  c ", "  /", -1))
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
			bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, ""))
			return
		} else if strings.HasPrefix(cb.Data, "pay:yes=") {
			invlabel := strings.Split(cb.Data, "=")[1]
			bolt11, err := rds.Get("payinvoice:" + invlabel).Result()
			if err != nil {
				bot.AnswerCallbackQuery(
					tgbotapi.NewCallback(
						cb.ID,
						"The payment confirmation button has expired.",
					),
				)
				return
			}

			bot.AnswerCallbackQuery(
				tgbotapi.NewCallbackWithAlert(
					cb.ID,
					"Sending payment.",
				),
			)

			optmsats, _ := rds.Get("payinvoice:" + invlabel + ":msats").Int64()
			handlePayInvoice(u, bolt11, invlabel, int(optmsats))
		}
	}
}

func handleInlineQuery(q *tgbotapi.InlineQuery) {
	u, err := loadUser(int(q.From.ID))
	if err != nil {
		log.Debug().Err(err).
			Str("username", q.From.UserName).
			Int("id", q.From.ID).
			Msg("unregistered user trying to use inline query")

		bot.AnswerInlineQuery(tgbotapi.InlineConfig{
			InlineQueryID: q.ID,
			Results:       []interface{}{},
			SwitchPMText:  "Create an account first",
		})

		return
	}

	text := strings.TrimSpace(q.Query)
	log.Print(text)

	switch {
	case strings.HasPrefix(text, "invoice "):
		argv, err := shellquote.Split(text)
		if err != nil || len(argv) < 2 {
			goto answerEmpty
		}

		label := makeLabel(u.ChatId, q.ID)

		sats, err := strconv.Atoi(argv[1])
		if err != nil {
			goto answerEmpty
		}

		bolt11, qrpath, err := makeInvoice(u, label, sats, "inline-"+q.ID)
		if err != nil {
			log.Warn().Err(err).Msg("error making invoice on inline query.")
			goto answerEmpty
		}

		qrurl := s.ServiceURL + "/qr/" + qrpath

		result := tgbotapi.NewInlineQueryResultPhoto("inv-"+argv[1], qrurl)
		result.Title = argv[1] + " satoshis"
		result.Description = "Payment request for " + argv[1] + " satoshis"
		result.ThumbURL = qrurl
		result.Caption = bolt11

		res, err := bot.AnswerInlineQuery(tgbotapi.InlineConfig{
			InlineQueryID: q.ID,
			Results:       []interface{}{result},
			IsPersonal:    true,
		})

		go func(qrpath string) {
			time.Sleep(30 * time.Second)
			os.Remove(qrpath)
		}(qrpath)

		if err != nil || !res.Ok {
			log.Warn().Err(err).Str("bolt11", bolt11).Str("qr", qrpath).
				Str("resp", res.Description).
				Msg("error answering inline query")
		}

		return
	case strings.HasPrefix(text, "tip "):
		return
	}

answerEmpty:
	bot.AnswerInlineQuery(tgbotapi.InlineConfig{
		InlineQueryID: q.ID,
		Results:       []interface{}{},
	})
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
		fmt.Sprintf("[%s] \n%d satoshis. \nhash: %s.",
			inv.Get("description").String(),
			amount/1000,
			inv.Get("payment_hash").String()),
	)
}

func handlePayInvoice(u User, bolt11, invlabel string, optmsats int) {
	// check if this is an internal invoice (it will have a different label)
	if label := rds.Get("recinvoice.internal:" + bolt11).String(); label != "" {
		// this is an internal invoice. do not pay.
		// delete it and just transfer balance.
		rds.Del("recinvoice.internal:" + bolt11)
		ln.Call("delinvoice", label, "unpaid")

		targetId, err := rds.Get("recinvoice:" + label + ":creator").Int64()
		if err != nil {
			log.Warn().Err(err).
				Str("label", label).
				Msg("failed to get internal invoice target from redis")
			u.notify("Failed to find invoice payee")
			return
		}

		amount, hash, errMsg, err := u.payInternally(int(targetId), bolt11, label, optmsats)
		if err != nil {
			log.Warn().Err(err).
				Str("label", label).
				Msg("failed to pay pay internally")
			u.notify("Failed to pay: " + errMsg)
			return
		}

		// internal payment succeeded
		target, err := loadUser(int(targetId))
		if err == nil {
			target.notifyAsReply(
				fmt.Sprintf("Payment received: %d. \n\nhash: %s.", amount/1000, hash),
				messageIdFromLabel(label),
			)
		}

		return
	}

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
	userId, _ := rds.Get("recinvoice:" + label + ":creator").Int64()
	u, err := loadUser(int(userId))
	if err != nil {
		log.Warn().Err(err).
			Int64("userid", userId).Str("label", label).Int64("index", index).
			Msg("couldn't load user who created this invoice.")
		return
	}

	msats := res.Get("msatoshi_received").Int()
	desc := res.Get("description").String()
	hash := res.Get("payment_hash").String()
	bolt11 := res.Get("bolt11").String()

	_, err = u.paymentReceived(
		int(msats),
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

	u.notifyAsReply(
		fmt.Sprintf("Payment received: %d. \n\nhash: %s.", msats/1000, hash),
		messageIdFromLabel(label),
	)
}
