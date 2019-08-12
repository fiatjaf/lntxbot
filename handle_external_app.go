package main

import (
	"fmt"
	"strings"
	"time"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/docopt/docopt-go"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

func handleExternalApp(u User, opts docopt.Opts, messageId int) {
	switch {
	case opts["microbet"].(bool):
		if opts["bets"].(bool) {
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

			u.notifyWithKeyboard(t.MICROBETBALANCE, t.T{"Balance": balance}, &tgbotapi.InlineKeyboardMarkup{
				[][]tgbotapi.InlineKeyboardButton{
					{
						tgbotapi.NewInlineKeyboardButtonData(translate(t.WITHDRAW, u.Locale), "app=microbet-withdraw"),
					},
				},
			}, 0)
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

			u.notifyWithKeyboard(t.MICROBETBETHEADER, nil, &tgbotapi.InlineKeyboardMarkup{inlinekeyboard}, 0)
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
				handleHelp(u, "bitflash")
				return
			}

			ordercreated, err := prepareBitflashTransaction(u, messageId, satoshis, address)
			if err != nil {
				u.notifyAsReply(t.ERROR, t.T{"Err": err.Error()}, messageId)
				return
			}

			inv, _ := ln.Call("decodepay", ordercreated.Bolt11)

			// confirm
			u.notifyWithKeyboard(t.BITFLASHCONFIRM, t.T{
				"BTCAmount": ordercreated.ReceiverAmount,
				"Address":   ordercreated.Receiver,
				"Sats":      inv.Get("msatoshi").Float() / 1000,
			}, &tgbotapi.InlineKeyboardMarkup{
				[][]tgbotapi.InlineKeyboardButton{
					{
						tgbotapi.NewInlineKeyboardButtonData(
							translate(t.CANCEL, u.Locale),
							fmt.Sprintf("cancel=%d", u.Id),
						),
						tgbotapi.NewInlineKeyboardButtonData(
							translate(t.CONFIRM, u.Locale),
							fmt.Sprintf("app=bitflash-%s", ordercreated.ChargeId),
						),
					},
				},
			}, 0)
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
				handleHelp(u, "satellite")
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
	case opts["golightning"].(bool):
		sats, err := opts.Int("<satoshis>")
		if err != nil {
			handleHelp(u, "golightning")
			return
		}

		order, err := prepareGoLightningTransaction(u, messageId, sats)
		if err != nil {
			u.notify(t.GOLIGHTNINGFAIL, t.T{"Err": err.Error()})
			return
		}

		u.notify(t.GOLIGHTNINGFINISH, t.T{"Order": order})
	case opts["poker"].(bool):
		subscribePoker(u, time.Minute*5, false)

		if opts["deposit"].(bool) {
			satoshis, err := opts.Int("<satoshis>")
			if err != nil {
				u.notify(t.INVALIDAMT, t.T{"Amount": opts["<satoshis>"]})
				break
			}

			err = pokerDeposit(u, satoshis, messageId)
			if err != nil {
				u.notify(t.POKERDEPOSITFAIL, t.T{"Err": err.Error()})
				break
			}
			subscribePoker(u, time.Minute*15, false)
		} else if opts["balance"].(bool) {
			balance, err := getPokerBalance(u)
			if err != nil {
				u.notify(t.POKERBALANCEERROR, t.T{"Err": err.Error()})
				break
			}

			u.notifyWithKeyboard(t.POKERBALANCE, t.T{"Balance": balance}, &tgbotapi.InlineKeyboardMarkup{
				[][]tgbotapi.InlineKeyboardButton{
					{
						tgbotapi.NewInlineKeyboardButtonData(translate(t.WITHDRAW, u.Locale), "app=poker-withdraw"),
					},
				},
			}, 0)
		} else if opts["withdraw"].(bool) {
			balance, err := getPokerBalance(u)
			if err != nil {
				u.notify(t.POKERBALANCEERROR, t.T{"Err": err.Error()})
				break
			}

			err = withdrawPoker(u, balance, messageId)
			if err != nil {
				u.notifyAsReply(t.ERROR, t.T{"Err": err.Error()}, messageId)
				break
			}
		} else if opts["play"].(bool) {
			chattable := tgbotapi.GameConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID: u.ChatId,
				},
				GameShortName: "poker",
			}
			bot.Send(chattable)
			subscribePoker(u, time.Minute*15, false)
		} else if opts["url"].(bool) {
			u.notify(t.POKERSECRETURL, t.T{"URL": getPokerURL(u)})
			subscribePoker(u, time.Minute*15, false)
		} else if opts["available"].(bool) || opts["wait"].(bool) || opts["watch"].(bool) {
			if minutes, err := opts.Int("<minutes>"); err != nil {
				u.notify(t.ERROR, t.T{"Err": err})
			} else {
				u.notify(t.POKERSUBSCRIBED, t.T{"Minutes": minutes})
				subscribePoker(u, time.Minute*time.Duration(minutes), true)
			}
		} else {
			// default to "status"
			nplayers, ntables, err1 := getActivePokerTables()
			_, chips, err2 := getCurrentPokerPlayers()
			if err1 != nil || err2 != nil {
				u.notify(t.ERROR, t.T{"Err": "failed to query."})
				break
			}

			u.notify(t.POKERSTATUS, t.T{
				"Tables":  ntables,
				"Players": nplayers,
				"Chips":   chips,
			})

			subscribePoker(u, time.Minute*10, false)
		}
	case opts["gifts"].(bool):
		// create gift or fallback to list gifts
		sats, err := opts.Int("<satoshis>")
		if err == nil {
			// create
			err = createGift(u, sats, messageId)
			if err != nil {
				u.notify(t.GIFTSERROR, t.T{"Err": err.Error()})
			}
			return
		} else {
			// list
			var data GiftsData
			err = u.getAppData("gifts", &data)
			if err != nil {
				u.notify(t.GIFTSERROR, t.T{"Err": err.Error()})
				return
			}

			gifts := make([]GiftsGift, len(data.Gifts))
			for i, orderId := range data.Gifts {
				gift, _ := getGift(orderId)
				gifts[i] = gift
			}

			u.notify(t.GIFTSLIST, t.T{"Gifts": gifts})
		}
	case opts["paywall"].(bool):
		switch {
		case opts["balance"].(bool):
			balance, err := getPaywallBalance(u)
			if err != nil {
				u.notify(t.PAYWALLERROR, t.T{"Err": err.Error()})
				return
			}

			u.notifyWithKeyboard(t.PAYWALLBALANCE, t.T{"Balance": balance}, &tgbotapi.InlineKeyboardMarkup{
				[][]tgbotapi.InlineKeyboardButton{
					{
						tgbotapi.NewInlineKeyboardButtonData(translate(t.WITHDRAW, u.Locale), "app=paywall-withdraw"),
					},
				},
			}, 0)
		case opts["withdraw"].(bool):
			err := withdrawPaywall(u)
			if err != nil {
				u.notify(t.PAYWALLERROR, t.T{"Err": err.Error()})
				return
			}
		default:
			// create paywall link or fallback to list paywalls
			if url, ok := opts["<url>"].(string); ok {
				// create
				sats, err := opts.Int("<satoshis>")
				if err != nil {
					u.notify(t.INVALIDAMOUNT, t.T{"Amount": opts["<satoshis>"]})
					return
				}

				var memo string
				if imemo, ok := opts["<memo>"]; ok {
					memo = strings.Join(imemo.([]string), " ")
				}

				link, err := createPaywallLink(u, sats, url, memo)
				if err != nil {
					u.notify(t.PAYWALLERROR, t.T{"Err": err.Error()})
					return
				}

				u.notify(t.PAYWALLCREATED, t.T{"Link": link})
			} else {
				// list
				links, err := listPaywallLinks(u)
				if err != nil {
					u.notify(t.PAYWALLERROR, t.T{"Err": err.Error()})
					return
				}

				u.notify(t.PAYWALLLISTLINKS, t.T{"Links": links})
			}
		}
	default:
		handleHelp(u, "app")
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

			removeKeyboardButtons(cb)
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
	case "poker":
		if parts[1] == "withdraw" {
			balance, err := getPokerBalance(u)
			if err != nil {
				u.notify(t.POKERBALANCEERROR, t.T{"Err": err.Error()})
				return translate(t.FAILURE, u.Locale)
			}

			if err != nil {
				u.notify(t.POKERBALANCEERROR, t.T{"Err": err.Error()})
				return translate(t.FAILURE, u.Locale)
			}

			err = withdrawPoker(u, balance, messageId)
			if err != nil {
				u.notify(t.ERROR, t.T{"Err": err.Error()})
				return translate(t.FAILURE, u.Locale)
			}

			removeKeyboardButtons(cb)
			return translate(t.PROCESSING, u.Locale)
		}
	case "paywall":
		if parts[1] == "withdraw" {
			err := withdrawPaywall(u)
			if err != nil {
				u.notify(t.PAYWALLERROR, t.T{"Err": err.Error()})
				return translate(t.FAILURE, u.Locale)
			}
		}
	}

	return
}
