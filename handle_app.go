package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/docopt/docopt-go"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/skip2/go-qrcode"
)

func handleExternalApp(u User, opts docopt.Opts, message *tgbotapi.Message) {
	messageId := message.MessageID

	switch {
	case opts["microbet"].(bool):
		if opts["bets"].(bool) || opts["list"].(bool) {
			// list my bets
			bets, err := getMyMicrobetBets(u)
			if err != nil {
				u.notify(t.ERROR, t.T{"Err": err.Error()})
				return
			}

			// only show the last 30
			if len(bets) > 30 {
				bets = bets[:30]
			}

			u.notify(t.MICROBETLIST, t.T{"Bets": bets})
		} else if opts["balance"].(bool) {
			balance, err := getMicrobetBalance(u)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "microbet", "Err": err.Error()})
				return
			}

			u.notifyWithKeyboard(t.MICROBETBALANCE, t.T{"Balance": balance}, &tgbotapi.InlineKeyboardMarkup{
				[][]tgbotapi.InlineKeyboardButton{
					{
						tgbotapi.NewInlineKeyboardButtonData(translate(t.WITHDRAW, u.Locale), "x=microbet-withdraw"),
					},
				},
			}, 0)
		} else if opts["withdraw"].(bool) {
			balance, err := getMicrobetBalance(u)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "microbet", "Err": err.Error()})
				return
			}

			err = withdrawMicrobet(u, int(float64(balance)*0.99))
			if err != nil {
				u.notifyAsReply(t.ERROR, t.T{"Err": err.Error()}, messageId)
				return
			}
		} else {
			// list available bets as actionable buttons
			inlineKeyboard, err := microbetKeyboard()
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "microbet", "Err": err.Error()})
				return
			}
			u.notifyWithKeyboard(t.MICROBETBETHEADER, nil,
				&tgbotapi.InlineKeyboardMarkup{inlineKeyboard}, 0)
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
							fmt.Sprintf("x=bitflash-%s", ordercreated.ChargeId),
						),
					},
				},
			}, 0)
		}
	case opts["satellite"].(bool):
		if opts["transmissions"].(bool) {
			// show past transmissions
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
		} else {
			// create an order
			satoshis, err := opts.Int("<satoshis>")
			if err != nil {
				handleHelp(u, "satellite")
				return
			}

			message := getVariadicFieldOrReplyToContent(opts, message, "<message>")
			if message == "" {
				handleHelp(u, "satellite")
				return
			}

			err = createSatelliteOrder(u, messageId, satoshis, message)
			if err != nil {
				u.notifyAsReply(t.ERROR, t.T{"App": "satellite", "Err": err.Error()}, messageId)
				return
			}
		}
	case opts["fundbtc"].(bool):
		sats, err := opts.Int("<satoshis>")
		if err != nil {
			handleHelp(u, "fundbtc")
			return
		}

		order, err := prepareGoLightningTransaction(u, messageId, sats-99)
		if err != nil {
			u.notify(t.ERROR, t.T{"App": "fundbtc", "Err": err.Error()})
			return
		}

		qrpath := qrImagePath(order.Address)
		err = qrcode.WriteFile("bitcoin:"+order.Address+"?amount="+order.Price, qrcode.Medium, 256, qrpath)
		if err == nil {
			sendMessageWithPicture(message.Chat.ID, qrpath,
				translateTemplate(t.FUNDBTCFINISH, u.Locale, t.T{"Order": order}))
		} else {
			u.notify(t.FUNDBTCFINISH, t.T{"Order": order})
		}
	case opts["bitclouds"].(bool):
		switch {
		case opts["create"].(bool):
			inlinekeyboard, err := bitcloudsImagesKeyboard()
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "bitclouds", "Err": err.Error()})
				return
			}
			u.notifyWithKeyboard(t.BITCLOUDSCREATEHEADER, nil, &tgbotapi.InlineKeyboardMarkup{inlinekeyboard}, 0)
		case opts["topup"].(bool):
			satoshis, err := opts.Int("<satoshis>")
			if err != nil {
				u.notify(t.INVALIDAMT, t.T{"Amount": opts["<satoshis>"]})
				return
			}

			host, err := opts.String("<host>")
			if err == nil {
				// host provided
				topupBitcloud(u, host, satoshis)
			} else {
				// host not provided, display options
				noHosts, singleHost,
					inlineKeyboard, err := bitcloudsHostsKeyboard(u, strconv.Itoa(satoshis))
				if err != nil {
					u.notify(t.ERROR, t.T{"App": "bitclouds", "Err": err.Error()})
					return
				}
				if noHosts {
					u.notify(t.BITCLOUDSNOHOSTS, nil)
					return
				}
				if singleHost != "" {
					topupBitcloud(u, singleHost, satoshis)
					return
				}

				u.notifyWithKeyboard(t.BITCLOUDSHOSTSHEADER, nil, &tgbotapi.InlineKeyboardMarkup{inlineKeyboard}, 0)
			}
		default: // "status"
			host, err := opts.String("<host>")
			if err == nil {
				// host provided
				showBitcloudStatus(u, host)
			} else {
				// host not provided, display options
				noHosts, singleHost,
					inlineKeyboard, err := bitcloudsHostsKeyboard(u, "status")
				if err != nil {
					u.notify(t.ERROR, t.T{"App": "bitclouds", "Err": err.Error()})
					return
				}
				if noHosts {
					u.notify(t.BITCLOUDSNOHOSTS, nil)
					return
				}
				if singleHost != "" {
					showBitcloudStatus(u, singleHost)
					return
				}

				u.notifyWithKeyboard(t.BITCLOUDSHOSTSHEADER, nil, &tgbotapi.InlineKeyboardMarkup{inlineKeyboard}, 0)
			}
		}
	case opts["qiwi"].(bool), opts["yandex"].(bool):
		exchangeType := "qiwi"
		if opts["yandex"].(bool) {
			exchangeType = "yandex"
		}

		switch {
		case opts["list"].(bool):
			// list past orders
			var data LNToRubData
			err := u.getAppData("lntorub", &data)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": exchangeType, "Err": err.Error()})
				return
			}

			orders, _ := data.Orders[exchangeType]
			u.notify(t.LNTORUBORDERLIST, t.T{"Type": exchangeType, "Orders": orders})
		case opts["default"].(bool):
			// show or set current default
			if target, err := opts.String("<target>"); err == nil {
				// set target
				err := setDefaultLNToRubTarget(u, exchangeType, target)
				if err != nil {
					u.notify(t.ERROR, t.T{"App": exchangeType, "Err": err.Error()})
					return
				}
				u.notify(t.LNTORUBDEFAULTTARGET, t.T{"Type": exchangeType, "Target": target})
			} else {
				// no target given, show current
				target := getDefaultLNToRubTarget(u, exchangeType)
				u.notify(t.LNTORUBDEFAULTTARGET, t.T{"Type": exchangeType, "Target": target})
			}
		default:
			// ask to send transfer
			amount, err := opts.Float64("<amount>")
			if err != nil {
				u.notify(t.ERROR, t.T{"App": exchangeType, "Err": "Invalid amount."})
				return
			}

			unit := "rub"
			if opts["sat"].(bool) {
				unit = "sat"
			}

			target, err := opts.String("<target>")
			if err != nil {
				// no target, let's check if there's some saved
				target = getDefaultLNToRubTarget(u, exchangeType)
				if target == "" {
					u.notify(t.LNTORUBMISSINGTARGET, t.T{"Type": exchangeType})
					return
				}
			}

			// init an order and get the exchange rate
			order, err := LNToRubExchangeInit(u, amount, exchangeType, unit, target, messageId)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": exchangeType, "Err": err.Error()})
				return
			}

			// serialize this intermediary structure to json an save it on redis
			jorder, err := json.Marshal(order)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": exchangeType, "Err": err.Error()})
				return
			}
			err = rds.Set("lntorub:"+order.Hash, jorder, time.Hour).Err()
			if err != nil {
				u.notify(t.ERROR, t.T{"App": exchangeType, "Err": err.Error()})
				return
			}

			u.notifyWithKeyboard(t.LNTORUBCONFIRMATION, t.T{
				"Sat":    order.Sat,
				"Rub":    order.Rub,
				"Type":   order.Type,
				"Target": order.Target,
			}, &tgbotapi.InlineKeyboardMarkup{
				[][]tgbotapi.InlineKeyboardButton{
					{
						tgbotapi.NewInlineKeyboardButtonData(
							translate(t.CANCEL, u.Locale),
							fmt.Sprintf("cancel=%d", u.Id)),
						tgbotapi.NewInlineKeyboardButtonData(
							translate(t.YES, u.Locale),
							fmt.Sprintf("x=lntorub-%s", order.Hash)),
					},
				},
			}, 0)

			// cancel this order after 2 minutes
			go func() {
				time.Sleep(2 * time.Minute)
				LNToRubExchangeCancel(order.Hash)
			}()
		}
	case opts["bitrefill"].(bool):
		switch {
		case opts["country"].(bool):
			countryCode, _ := opts.String("<country_code>")
			countryCode = strings.ToUpper(countryCode)

			if isValidBitrefillCountry(countryCode) {
				err := setBitrefillCountry(u, countryCode)
				if err != nil {
					u.notify(t.ERROR, t.T{"App": "Bitrefill", "Err": err.Error()})
				}
				u.notify(t.BITREFILLCOUNTRYSET, t.T{"CountryCode": countryCode})
			} else {
				u.notify(t.BITREFILLINVALIDCOUNTRY, t.T{"CountryCode": countryCode, "Available": BITREFILLCOUNTRIES})
			}
		default:
			query, err := opts.String("<query>")
			if err != nil {
				handleHelp(u, "bitrefill")
				return
			}

			phone, _ := opts.String("<phone_number>")
			countryCode, _ := getBitrefillCountry(u)

			items := queryBitrefillInventory(query, phone, countryCode)
			nitems := len(items)

			if nitems == 1 {
				handleBitrefillItem(u, items[0], phone)
				return
			}

			if nitems == 0 {
				u.notify(t.BITREFILLNOPROVIDERS, nil)
				return
			}

			inlineKeyboard := make([][]tgbotapi.InlineKeyboardButton, nitems/2+nitems%2)
			for i, item := range items {
				if i%2 == 0 {
					inlineKeyboard[i/2] = make([]tgbotapi.InlineKeyboardButton, 0, 2)
				}

				inlineKeyboard[i/2] = append(inlineKeyboard[i/2], tgbotapi.NewInlineKeyboardButtonData(
					item.Name,
					fmt.Sprintf("x=bitrefill-it-%s-%s", strings.Replace(item.Slug, "-", "~", -1), phone),
				))
			}

			u.notifyWithKeyboard(t.BITREFILLINVENTORYHEADER, nil, &tgbotapi.InlineKeyboardMarkup{inlineKeyboard}, 0)
		}
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
				u.notify(t.ERROR, t.T{"App": "poker", "Err": err.Error()})
				break
			}

			u.notifyWithKeyboard(t.POKERBALANCE, t.T{"Balance": balance}, &tgbotapi.InlineKeyboardMarkup{
				[][]tgbotapi.InlineKeyboardButton{
					{
						tgbotapi.NewInlineKeyboardButtonData(translate(t.WITHDRAW, u.Locale), "x=poker-withdraw"),
					},
				},
			}, 0)
		} else if opts["withdraw"].(bool) {
			balance, err := getPokerBalance(u)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "poker", "Err": err.Error()})
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
				u.notify(t.ERROR, t.T{"App": "gifts", "Err": err.Error()})
			}
			return
		} else {
			// list
			var data GiftsData
			err = u.getAppData("gifts", &data)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "gifts", "Err": err.Error()})
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
				u.notify(t.ERROR, t.T{"App": "paywall", "Err": err.Error()})
				return
			}

			u.notifyWithKeyboard(t.PAYWALLBALANCE, t.T{"Balance": balance}, &tgbotapi.InlineKeyboardMarkup{
				[][]tgbotapi.InlineKeyboardButton{
					{
						tgbotapi.NewInlineKeyboardButtonData(translate(t.WITHDRAW, u.Locale), "x=paywall-withdraw"),
					},
				},
			}, 0)
		case opts["withdraw"].(bool):
			err := withdrawPaywall(u)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "paywall", "Err": err.Error()})
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

				memo := getVariadicFieldOrReplyToContent(opts, nil, "<memo>")
				if memo == "" {
					handleHelp(u, "paywall")
					return
				}

				link, err := createPaywallLink(u, sats, url, memo)
				if err != nil {
					u.notify(t.ERROR, t.T{"App": "paywall", "Err": err.Error()})
					return
				}

				u.notify(t.PAYWALLCREATED, t.T{"Link": link})
				sendMessage(u.ChatId, fmt.Sprintf(`<a href="https://paywall.link/to/%s">https://paywall.link/to/%s</a>`, link.ShortURL, link.ShortURL))
			} else {
				// list
				links, err := listPaywallLinks(u)
				if err != nil {
					u.notify(t.ERROR, t.T{"App": "paywall", "Err": err.Error()})
					return
				}

				u.notify(t.PAYWALLLISTLINKS, t.T{"Links": links})
			}
		}
	case opts["sats4ads"].(bool):
		switch {
		case opts["on"].(bool):
			rate, err := opts.Int("<msat_per_character>")
			if err != nil {
				rate = 1
			}
			if rate > 1000 {
				u.notify(t.ERROR, t.T{"App": "sats4ads", "Err": "max = 1000 msatoshi"})
				return
			}

			err = turnSats4AdsOn(u, rate)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "sats4ads", "Err": err.Error()})
				return
			}
			u.notify(t.SATS4ADSTOGGLE, t.T{"On": true, "Sats": float64(rate) / 1000})
		case opts["off"].(bool):
			err := turnSats4AdsOff(u)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "sats4ads", "Err": err.Error()})
				return
			}

			u.notify(t.SATS4ADSTOGGLE, t.T{"On": false})
		case opts["rates"].(bool):
			rates, err := getSats4AdsRates(u)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "sats4ads", "Err": err.Error()})
				return
			}
			u.notify(t.SATS4ADSPRICETABLE, t.T{"Rates": rates})
		case opts["broadcast"].(bool):
			// check user banned
			var data Sats4AdsData
			err := u.getAppData("sats4ads", &data)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "sats4ads", "Err": err.Error()})
				return
			}
			if data.Banned {
				u.notify(t.ERROR, t.T{"App": "sats4ads", "Err": "user banned"})
				return
			}

			satoshis, err := opts.Int("<spend_satoshis>")
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "sats4ads", "Err": err.Error()})
				return
			}

			// we'll use either a message passed as an argument or the contents of the message being replied to
			contentMessage := message.ReplyToMessage
			if imessage, ok := opts["<message>"]; ok {
				text := strings.Join(imessage.([]string), " ")
				if text != "" {
					contentMessage = &tgbotapi.Message{
						MessageID: message.MessageID,
						Text:      text,
					}
				}
			}

			if contentMessage == nil {
				handleHelp(u, "sats4ads")
				return
			}

			// optional args
			maxrate, _ := opts.Int("--max-rate")
			offset, _ := opts.Int("--skip")

			go func() {
				nmessagesSent, totalCost, errMsg, err := broadcastSats4Ads(u, satoshis, contentMessage, maxrate, offset)
				if err != nil {
					log.Warn().Err(err).Str("user", u.Username).Msg("sats4ads broadcast fail")
					u.notify(t.ERROR, t.T{"App": "sats4ads", "Err": errMsg})
					return
				}

				u.notifyAsReply(t.SATS4ADSBROADCAST, t.T{"NSent": nmessagesSent, "Sats": totalCost}, messageId)
			}()
		}
	default:
		handleHelp(u, "app")
	}
}

func handleExternalAppCallback(u User, messageId int, cb *tgbotapi.CallbackQuery) (answer string) {
	parts := strings.Split(cb.Data[2:], "-")
	switch parts[0] {
	case "microbet":
		if parts[1] == "withdraw" {
			defer removeKeyboardButtons(cb)
			balance, err := getMicrobetBalance(u)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "microbet", "Err": err.Error()})
				return translate(t.FAILURE, u.Locale)
			}

			err = withdrawMicrobet(u, int(float64(balance)*0.99))
			if err != nil {
				u.notify(t.ERROR, t.T{"Err": err.Error()})
				return translate(t.FAILURE, u.Locale)
			}

			return translate(t.PROCESSING, u.Locale)
		} else {
			// bet on something
			betId := parts[1]
			back := parts[2] == "true"
			bet, err := getMicrobetBet(betId)
			if err != nil {
				log.Warn().Err(err).Str("betId", betId).Msg("bet not available")
				return translate(t.ERROR, u.Locale)
			}

			// post a notification message to identify this bet attempt
			message := u.notify(t.MICROBETPLACING, t.T{"Bet": bet, "Back": back})

			err = placeMicrobetBet(u, message.MessageID, betId, back)
			if err != nil {
				u.notify(t.ERROR, t.T{"Err": err.Error()})
				return translate(t.FAILURE, u.Locale)
			}

			return translate(t.PROCESSING, u.Locale)
		}
	case "bitrefill":
		switch parts[1] {
		case "it":
			removeKeyboardButtons(cb)

			item, ok := bitrefillInventory[strings.Replace(parts[2], "~", "-", -1)]
			if !ok {
				u.notify(t.ERROR, t.T{"App": "Bitrefill", "Err": "not found"})
				return
			}

			phone := parts[3]

			appendTextToMessage(cb, item.Name)
			handleBitrefillItem(u, item, phone)
		case "pl":
			removeKeyboardButtons(cb)

			// get item and package info
			item, ok := bitrefillInventory[strings.Replace(parts[2], "~", "-", -1)]
			if !ok {
				u.notify(t.ERROR, t.T{"App": "Bitrefill", "Err": "not found"})
				return
			}

			var pack BitrefillPackage
			idx, _ := strconv.Atoi(parts[3])
			packages := getBitRefillPackagesForItem(item)
			if len(packages) <= idx {
				u.notify(t.ERROR, t.T{"App": "Bitrefill", "Err": "not found"})
				return
			}
			pack = packages[idx]
			appendTextToMessage(cb, fmt.Sprintf("%v %s", pack.Value, item.Currency))

			phone := parts[4]

			// create order
			orderId, invoice, err := placeBitrefillOrder(u, item, pack, &phone)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "Bitrefill", "Err": err.Error()})
				return
			}

			// parse invoice
			inv, err := ln.Call("decodepay", invoice)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "Bitrefill", "Err": err.Error()})
				return
			}

			u.notifyWithKeyboard(t.BITREFILLCONFIRMATION, t.T{
				"Item":    item,
				"Package": pack,
				"Sats":    inv.Get("msatoshi").Float() / 1000,
			}, &tgbotapi.InlineKeyboardMarkup{
				[][]tgbotapi.InlineKeyboardButton{
					{
						tgbotapi.NewInlineKeyboardButtonData(
							translate(t.CANCEL, u.Locale),
							fmt.Sprintf("cancel=%d", u.Id)),
						tgbotapi.NewInlineKeyboardButtonData(
							translate(t.YES, u.Locale),
							fmt.Sprintf("x=bitrefill-pch-%s", orderId)),
					},
				},
			}, 0)
		case "pch":
			defer removeKeyboardButtons(cb)
			orderId := parts[2]
			err := purchaseBitrefillOrder(u, orderId)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "bitrefill", "Err": err.Error()})
				return
			}
		}
	case "bitclouds":
		defer removeKeyboardButtons(cb)
		switch parts[1] {
		case "create":
			image := parts[2]
			err := createBitcloudImage(u, image)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "bitclouds", "Err": err.Error()})
				return
			}
			appendTextToMessage(cb, image)
		case "status":
			host := parts[2]
			appendTextToMessage(cb, host)
			showBitcloudStatus(u, host)
		default: // sats to topup
			host := parts[2]
			sats, err := strconv.Atoi(parts[1])
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "bitclouds", "Err": err.Error()})
				return
			}
			appendTextToMessage(cb, host)
			topupBitcloud(u, host, sats)
		}
	case "lntorub":
		defer removeKeyboardButtons(cb)
		orderId := parts[1]

		// get order data from redis
		var order LNToRubOrder
		j, err := rds.Get("lntorub:" + orderId).Bytes()
		if err != nil {
			LNToRubExchangeCancel(orderId)
			u.notify(t.LNTORUBCANCELED, t.T{"Type": order.Type, "OrderId": orderId})
			return translate(t.ERROR, u.Locale)
		}
		err = json.Unmarshal(j, &order)
		if err != nil {
			LNToRubExchangeCancel(orderId)
			u.notify(t.ERROR, nil)
			return translate(t.ERROR, u.Locale)
		}

		err = LNToRubExchangeFinish(u, order)
		if err != nil {
			u.notify(t.ERROR, t.T{"App": order.Type, "Err": err.Error()})
			return translate(t.ERROR, u.Locale)
		}

		// query the status until it returns a success or error
		go func() {
			for i := 0; i < 10; i++ {
				time.Sleep(time.Second * 5 * time.Duration(i))
				status, err := LNToRubQueryStatus(orderId)
				if err != nil {
					break
				}

				switch status {
				case LNIN:
					continue
				case OKAY:
					u.notifyAsReply(t.LNTORUBFULFILLED,
						t.T{"Type": order.Type, "OrderId": orderId}, order.MessageId)
					return
				case CANC:
					u.notifyAsReply(t.LNTORUBCANCELED,
						t.T{"Type": order.Type, "OrderId": orderId}, order.MessageId)
					return
				case QER1, QER2:
					u.notifyAsReply(t.LNTORUBFIATERROR,
						t.T{"Type": order.Type, "OrderId": orderId}, order.MessageId)
					return
				default:
					continue
				}
			}
		}()
	case "bitflash":
		defer removeKeyboardButtons(cb)
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

		return translate(t.PROCESSING, u.Locale)
	case "poker":
		defer removeKeyboardButtons(cb)
		if parts[1] == "withdraw" {
			balance, err := getPokerBalance(u)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "poker", "Err": err.Error()})
				return translate(t.FAILURE, u.Locale)
			}

			if err != nil {
				u.notify(t.ERROR, t.T{"App": "poker", "Err": err.Error()})
				return translate(t.FAILURE, u.Locale)
			}

			err = withdrawPoker(u, balance, messageId)
			if err != nil {
				u.notify(t.ERROR, t.T{"Err": err.Error()})
				return translate(t.FAILURE, u.Locale)
			}

			return translate(t.PROCESSING, u.Locale)
		}
	case "paywall":
		defer removeKeyboardButtons(cb)
		if parts[1] == "withdraw" {
			err := withdrawPaywall(u)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "paywall", "Err": err.Error()})
				return translate(t.FAILURE, u.Locale)
			}
		}
	}

	return
}
