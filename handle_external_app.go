package main

import (
	"fmt"
	"strings"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/docopt/docopt-go"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

func handleExternalApp(u User, opts docopt.Opts, messageId int) {
	switch {
	case opts["microbet"].(bool):
		if opts["bet"].(bool) {
			// list available bets as actionable buttons
			bets, err := getMicrobetBets()
			if err != nil {
				sendMessage(u.ChatId, err.Error())
				return
			}

			inlinekeyboard := make([][]tgbotapi.InlineKeyboardButton, 2*len(bets))
			for i, bet := range bets {
				parts := strings.Split(bet.Description, "→")
				gamename := parts[0]
				backbet := parts[1]
				if bet.Exact {
					backbet += " (exact)"
				}

				inlinekeyboard[i*2] = []tgbotapi.InlineKeyboardButton{
					tgbotapi.NewInlineKeyboardButtonURL(
						fmt.Sprintf("%s (%d sat)", gamename, bet.Amount),
						"https://www.google.com/search?q="+gamename,
					),
				}
				inlinekeyboard[i*2+1] = []tgbotapi.InlineKeyboardButton{
					tgbotapi.NewInlineKeyboardButtonData(
						fmt.Sprintf("%s (%d)", backbet, bet.Backers),
						fmt.Sprintf("app=microbet-%s-true", bet.Id),
					),
					tgbotapi.NewInlineKeyboardButtonData(
						fmt.Sprintf("NOT (%d)", bet.TotalUsers-bet.Backers),
						fmt.Sprintf("app=microbet-%s-false", bet.Id),
					),
				}
			}

			chattable := tgbotapi.NewMessage(u.ChatId, "<b>[Microbet]</b> Bet on one of these predictions:")
			chattable.ParseMode = "HTML"
			chattable.BaseChat.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{inlinekeyboard}
			bot.Send(chattable)
		} else if opts["bets"].(bool) {
			// list my bets
			bets, err := getMyMicrobetBets(u)
			if err != nil {
				sendMessage(u.ChatId, err.Error())
				return
			}

			u.notify(t.MICROBETLIST, t.T{"Bets": bets})
		} else if opts["balance"].(bool) {
			balance, err := getMicrobetBalance(u)
			if err != nil {
				u.notify(t.MICROBETBALANCEERROR, t.T{"Err": err.Error()})
				return
			}

			chattable := tgbotapi.NewMessage(
				u.ChatId,
				translateTemplate(t.MICROBETBALANCE, u.Locale, t.T{"Balance": balance}),
			)
			chattable.ParseMode = "HTML"
			chattable.BaseChat.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(translate(t.WITHDRAW, u.Locale), "app=microbet-withdraw"),
				),
			)
			bot.Send(chattable)
		} else if opts["withdraw"].(bool) {
			balance, err := getMicrobetBalance(u)
			if err != nil {
				u.notify(t.MICROBETBALANCEERROR, t.T{"Err": err.Error()})
				return
			}

			err = withdrawMicrobet(u, int(float64(balance)*0.99))
			if err != nil {
				u.notifyAsReply(t.ERROR, t.T{"Err": err.Error()}, messageId)
				return
			}
		} else {
			u.notify(t.MICROBETHELP, nil)
		}
	case opts["bitflash"].(bool):
		if opts["orders"].(bool) {
			var data struct {
				Orders []string `json:"orders"`
			}
			err := u.getAppData("bitflash", &data)
			if err != nil {
				u.notify(t.ERROR, t.T{"Err": err.Error()})
				return
			}

			orders := make([]BitflashOrder, len(data.Orders))
			for i, id := range data.Orders {
				order, err := getBitflashOrder(id)
				if err != nil {
					log.Warn().Err(err).Str("id", id).Msg("error getting bitflash order on list")
					continue
				}
				orders[i] = order
			}

			u.notify(t.BITFLASHLIST, t.T{"Orders": orders})
		} else if opts["status"].(bool) {

		} else if opts["rate"].(bool) {

		} else {
			// queue a transaction or show help if no arguments
			satoshis, err1 := opts.Int("<satoshis>")
			address, err2 := opts.String("<address>")

			if err1 != nil || err2 != nil {
				u.notify(t.BITFLASHHELP, nil)
				return
			}

			ordercreated, err := prepareBitflashTransaction(u, messageId, satoshis, address)
			if err != nil {
				u.notifyAsReply(t.ERROR, t.T{"Err": err.Error()}, messageId)
				return
			}

			inv, _ := ln.Call("decodepay", ordercreated.Bolt11)

			// confirm
			chattable := tgbotapi.NewMessage(u.ChatId,
				translateTemplate(t.BITFLASHCONFIRM, u.Locale, t.T{
					"BTCAmount": ordercreated.ReceiverAmount,
					"Address":   ordercreated.Receiver,
					"Sats":      inv.Get("msatoshi").Float() / 1000,
				},
				))
			chattable.ParseMode = "HTML"
			chattable.BaseChat.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(
						translate(t.CANCEL, u.Locale),
						fmt.Sprintf("cancel=%d", u.Id),
					),
					tgbotapi.NewInlineKeyboardButtonData(
						translate(t.CONFIRM, u.Locale),
						fmt.Sprintf("app=bitflash-%s", ordercreated.ChargeId),
					),
				),
			)
			bot.Send(chattable)
		}
	case opts["satellite"].(bool):
		if opts["transmissions"].(bool) {
			var satdata SatelliteData
			err := u.getAppData("satellite", &satdata)
			if err != nil {
				u.notify(t.SATELLITEFAILEDTOGET, t.T{"Err": err.Error()})
				return
			}

			orders := make([]SatelliteOrder, len(satdata.Orders))
			for i, tuple := range satdata.Orders {
				order, err := fetchSatelliteOrder(tuple[0], tuple[1])
				if err == nil {
					orders[i] = order
				}
			}

			u.notify(t.SATELLITELIST, t.T{"Orders": orders})
		} else if opts["queue"].(bool) {
			queue, err := getSatelliteQueue()
			if err != nil {
				u.notifyAsReply(t.SATELLITEQUEUEERROR, t.T{"Err": err.Error()}, messageId)
				return
			}

			u.notify(t.SATELLITEQUEUE, t.T{"Orders": queue})
		} else if opts["bump"].(bool) {
			err := bumpSatelliteOrder(u, messageId, opts["<transaction_id>"].(string), opts["<satoshis>"].(int))
			if err != nil {
				u.notifyAsReply(t.SATELLITEBUMPERROR, t.T{"Err": err.Error()}, messageId)
				return
			}
		} else if opts["delete"].(bool) {
			err := deleteSatelliteOrder(u, opts["<transaction_id>"].(string))
			if err != nil {
				u.notifyAsReply(t.SATELLITEDELETEERROR, t.T{"Err": err.Error()}, messageId)
				return
			}
			u.notifyAsReply(t.SATELLITEDELETED, nil, messageId)
			return
		} else {
			// either show help or create an order
			satoshis, err := opts.Int("<satoshis>")

			if err != nil {
				u.notify(t.SATELLITEHELP, nil)
				return
			}

			// create an order
			var message string
			if imessage, ok := opts["<message>"]; ok {
				message = strings.Join(imessage.([]string), " ")
			}

			err = createSatelliteOrder(u, messageId, satoshis, message)
			if err != nil {
				u.notifyAsReply(t.SATELLITETRANSMISSIONERROR, t.T{"Err": err.Error()}, messageId)
				return
			}
		}
	}
}

func handleExternalAppCallback(u User, messageId int, cb *tgbotapi.CallbackQuery) (answer string) {
	parts := strings.Split(cb.Data[4:], "-")
	switch parts[0] {
	case "microbet":
		if parts[1] == "withdraw" {
			balance, err := getMicrobetBalance(u)
			if err != nil {
				u.notify(t.MICROBETBALANCEERROR, t.T{"Err": err.Error()})
				return translate(t.FAILURE, u.Locale)
			}

			err = withdrawMicrobet(u, int(float64(balance)*0.99))
			if err != nil {
				u.notify(t.ERROR, t.T{"Err": err.Error()})
				return translate(t.FAILURE, u.Locale)
			}

			return translate(t.PROCESSING, u.Locale)
		} else {
			betId := parts[1]
			back := parts[2] == "true"
			bet, err := getMicrobetBet(betId)
			if err != nil {
				log.Warn().Err(err).Str("betId", betId).Msg("bet not available")
				return translate(t.ERROR, u.Locale)
			}

			// post a notification message to identify this bet attempt
			message := u.notify(t.MICROBETPLACING, t.T{"Bet": bet})

			err = placeMicrobetBet(u, message.MessageID, betId, back)
			if err != nil {
				u.notify(t.ERROR, t.T{"Err": err.Error()})
				return translate(t.FAILURE, u.Locale)
			}

			return translate(t.PROCESSING, u.Locale)
		}
	case "bitflash":
		chargeId := parts[1]

		// get data for this charge
		order, err := getBitflashOrder(chargeId)
		if err != nil {
			u.notify(t.ERROR, t.T{"Err": err.Error()})
			return translate(t.ERROR, u.Locale)
		}

		// pay it - just paying the invoice is enough
		err = payBitflashInvoice(u, order, messageId)
		if err != nil {
			u.notify(t.ERROR, t.T{"Err": err.Error()})
			return translate(t.FAILURE, u.Locale)
		}

		// store order id so we can show it later on /app bitflash orders
		saveBitflashOrder(u, order.Id)

		removeKeyboardButtons(cb)
		return translate(t.PROCESSING, u.Locale)
	}

	return
}
