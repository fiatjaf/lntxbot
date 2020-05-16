package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/skip2/go-qrcode"
)

func handleExternalApp(u User, opts docopt.Opts, message *tgbotapi.Message) {
	messageId := message.MessageID

	switch {
	case opts["etleneum"].(bool), opts["etl"].(bool):
		if contract, err := opts.String("<contract>"); err == nil {
			method, _ := opts["<method>"].(string)

			// translate alias into contract id
			contract = aliasToEtleneumContractId(u, contract)

			if opts["state"].(bool) {
				// contract state
				jqfilter, _ := opts.String("<jqfilter>")
				state, err := getEtleneumContractState(contract, jqfilter)
				if err != nil {
					u.notify(t.ERROR, t.T{"App": "Etleneum", "Err": err.Error()})
					return
				}
				statestr, _ := json.MarshalIndent(state, "", "  ")
				u.notify(t.ETLENEUMCONTRACTSTATE, t.T{
					"Id":    contract,
					"State": string(statestr),
				})
				go u.track("etleneum state", map[string]interface{}{"contract": contract})
			} else if opts["subscribe"].(bool) {
				// subscribe to a contract
				err = subscribeEtleneum(u, contract, false)
				if err != nil {
					u.notify(t.ERROR, t.T{"App": "Etleneum", "Err": err.Error()})
					return
				}
				u.notify(t.ETLENEUMSUBSCRIBED, t.T{"Contract": contract, "Subscribed": true})
				go u.track("etleneum subscribe", map[string]interface{}{"contract": contract})
			} else if opts["unsubscribe"].(bool) {
				// unsubscribe from a contract
				err = unsubscribeEtleneum(u, contract, false)
				if err != nil {
					u.notify(t.ERROR, t.T{"App": "Etleneum", "Err": err.Error()})
					return
				}
				u.notify(t.ETLENEUMSUBSCRIBED, t.T{"Contract": contract, "Subscribed": false})
				go u.track("etleneum unsubscribe", map[string]interface{}{"contract": contract})
			} else if method == "" {
				// contract metadata
				ct, err := getEtleneumContract(contract)
				if err != nil {
					u.notify(t.ERROR, t.T{"App": "Etleneum", "Err": err.Error()})
					return
				}

				ct.Readme = strings.Split(strings.TrimSpace(ct.Readme), "\n")[0]

				u.notify(t.ETLENEUMCONTRACT, t.T{"Contract": ct})
				go u.track("etleneum metadata", map[string]interface{}{"contract": contract})
			} else {
				// make a call
				params := opts["<params>"].([]string)
				var sats *int // nil means not specified
				if satoshi, err := opts.Int("<satoshi>"); err == nil {
					sats = &satoshi
				} else {
					// surprise! supplying <satoshi> is actually optional.
					// if it's not an integer we'll interpret it as a kv param.
					if kv, err := opts.String("<satoshi>"); err == nil {
						params = append(params, kv)
					}

					// and set sats to 0
					satoshi := 0
					sats = &satoshi
				}

				// start listening to this contract for a couple of minutes minutes
				subscribeEtleneum(u, contract, true)
				go func() {
					time.Sleep(5 * time.Minute)
					unsubscribeEtleneum(u, contract, true)
				}()

				etlurl, err := buildEtleneumCallLNURL(&u, contract, method, params, sats)
				if err != nil {
					u.notify(t.ERROR, t.T{"App": "Etleneum", "Err": err.Error()})
					return
				}
				log.Debug().Str("url", etlurl).Msg("etleneum call lnurl")

				var msatsAcceptable *int64
				if sats != nil {
					msatsAcceptableV := int64(*sats) * 1000
					msatsAcceptable = &msatsAcceptableV
				}

				handleLNURL(u, etlurl, handleLNURLOpts{
					messageId:          message.MessageID,
					payWithoutPromptIf: msatsAcceptable,
				})

				go u.track("etleneum call", map[string]interface{}{
					"contract": contract,
					"method":   method,
					"sats":     sats,
				})
			}
		} else if opts["call"].(bool) {
			call, err := getEtleneumCall(opts["<id>"].(string))
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "Etleneum", "Err": err.Error()})
				return
			}
			u.notify(t.ETLENEUMCALL, t.T{"Call": call})
			go u.track("etleneum view call", nil)
		} else if opts["contracts"].(bool) || opts["apps"].(bool) {
			contracts, aliases, err := listEtleneumContracts(u)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "Etleneum", "Err": err.Error()})
				return
			}
			go u.track("etleneum contracts", nil)
			u.notify(t.ETLENEUMCONTRACTS, t.T{"Contracts": contracts, "Aliases": aliases})
		} else if opts["withdraw"].(bool) {
			_, _, _, withdraw, err := etleneumLogin(u)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "Etleneum", "Err": err.Error()})
				return
			}

			go u.track("etleneum withdraw", nil)
			handleLNURL(u, withdraw, handleLNURLOpts{messageId: message.MessageID})
		} else {
			account, _, balance, _, err := etleneumLogin(u)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "Etleneum", "Err": err.Error()})
				return
			}
			go u.track("etleneum account", map[string]interface{}{"sats": balance})
			u.notifyWithKeyboard(t.ETLENEUMACCOUNT, t.T{
				"Account": account,
				"Balance": balance,
			}, &tgbotapi.InlineKeyboardMarkup{
				[][]tgbotapi.InlineKeyboardButton{
					{
						tgbotapi.NewInlineKeyboardButtonData(translate(t.WITHDRAW, u.Locale), "x=etleneum-withdraw"),
					},
				},
			}, 0)
		}
	case opts["microbet"].(bool):
		if opts["bets"].(bool) || opts["list"].(bool) {
			// list my bets
			bets, err := getMyMicrobetBets(u)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "Microbet", "Err": err.Error()})
				return
			}

			// only show the last 30
			if len(bets) > 30 {
				bets = bets[:30]
			}

			go u.track("microbet list", nil)
			u.notify(t.MICROBETLIST, t.T{"Bets": bets})
		} else if opts["balance"].(bool) {
			balance, err := getMicrobetBalance(u)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "Microbet", "Err": err.Error()})
				return
			}

			go u.track("microbet balance", map[string]interface{}{"sats": balance})
			u.notifyWithKeyboard(t.APPBALANCE, t.T{"App": "Etleneum", "Balance": balance}, &tgbotapi.InlineKeyboardMarkup{
				[][]tgbotapi.InlineKeyboardButton{
					{
						tgbotapi.NewInlineKeyboardButtonData(translate(t.WITHDRAW, u.Locale), "x=microbet-withdraw"),
					},
				},
			}, 0)
		} else if opts["withdraw"].(bool) {
			balance, err := getMicrobetBalance(u)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "Microbet", "Err": err.Error()})
				return
			}

			go u.track("microbet withdraw", map[string]interface{}{"sats": balance})
			err = withdrawMicrobet(u, int(float64(balance)*0.99))
			if err != nil {
				u.notifyAsReply(t.ERROR, t.T{"Err": err.Error()}, messageId)
				return
			}
		} else {
			// list available bets as actionable buttons
			inlineKeyboard, err := microbetKeyboard()
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "Microbet", "Err": err.Error()})
				return
			}
			u.notifyWithKeyboard(t.MICROBETBETHEADER, nil,
				&tgbotapi.InlineKeyboardMarkup{inlineKeyboard}, 0)

			go u.track("microbet show-bets", nil)
		}
	case opts["satellite"].(bool):
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
			u.notifyAsReply(t.ERROR,
				t.T{"App": "satellite", "Err": err.Error()}, messageId)
			return
		}

		go u.track("satellite send", map[string]interface{}{"sats": satoshis})
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
		go u.track("fundbtc start", map[string]interface{}{"sats": sats})
	case opts["bitclouds"].(bool):
		switch {
		case opts["create"].(bool):
			inlinekeyboard, err := bitcloudsImagesKeyboard()
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "bitclouds", "Err": err.Error()})
				return
			}
			u.notifyWithKeyboard(t.BITCLOUDSCREATEHEADER, nil, &tgbotapi.InlineKeyboardMarkup{inlinekeyboard}, 0)

			go u.track("bitclouds create-init", nil)
		case opts["topup"].(bool):
			satoshis, err := opts.Int("<satoshis>")
			if err != nil {
				u.notify(t.INVALIDAMT, t.T{"Amount": opts["<satoshis>"]})
				return
			}

			host, err := opts.String("<host>")
			if err == nil {
				// host provided
				host = unescapeBitcloudsHost(host)
				topupBitcloud(u, host, satoshis)
				go u.track("bitclouds topup", map[string]interface{}{"host": host})
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
		case opts["adopt"].(bool), opts["abandon"].(bool):
			host := unescapeBitcloudsHost(opts["<host>"].(string))
			var data BitcloudsData
			err := u.getAppData("bitclouds", &data)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "bitclouds", "Err": err.Error()})
				return
			}
			if opts["adopt"].(bool) {
				data[host] = BitcloudInstanceData{Policy: "remind"}
				go u.track("bitclouds adopt", map[string]interface{}{"host": host})
			} else {
				delete(data, host)
				go u.track("bitclouds abandon", map[string]interface{}{"host": host})
			}
			err = u.setAppData("bitclouds", data)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "bitclouds", "Err": err.Error()})
				return
			}
			if opts["adopt"].(bool) {
				// on success, simulate status command
				opts["adopt"] = false
				opts["status"] = true
				handleExternalApp(u, opts, message)
			} else {
				u.notify(t.COMPLETED, nil)
			}
		default: // "status"
			go u.track("bitclouds status", nil)

			host, err := opts.String("<host>")
			if err == nil {
				// host provided
				host = unescapeBitcloudsHost(host)
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
	case opts["skype"].(bool):
		lntorublnurl, _ := url.Parse("https://vds.sw4me.com/lnpay")
		qs := lntorublnurl.Query()
		qs.Set("tag", "pay")
		qs.Set("acc", hex.EncodeToString([]byte(opts["<username>"].(string))))
		qs.Set("p", "skype")
		if usd, err := opts.String("<usd>"); err == nil {
			qs.Set("usd", usd)
		}
		lntorublnurl.RawQuery = qs.Encode()
		handleLNURL(u, lntorublnurl.String(),
			handleLNURLOpts{messageId: message.MessageID})
	case opts["rub"].(bool):
		lntorublnurl, _ := url.Parse("https://vds.sw4me.com/lnpay")
		qs := lntorublnurl.Query()
		qs.Set("tag", "pay")
		qs.Set("acc", hex.EncodeToString([]byte(opts["<account>"].(string))))
		qs.Set("p", opts["<service>"].(string))
		if rub, err := opts.String("<rub>"); err == nil {
			qs.Set("rub", rub)
		}
		lntorublnurl.RawQuery = qs.Encode()
		handleLNURL(u, lntorublnurl.String(),
			handleLNURLOpts{messageId: message.MessageID})
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

			go u.track("bitrefill set-country", map[string]interface{}{
				"country": countryCode,
			})
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

			go u.track("bitrefill query", map[string]interface{}{
				"query": query,
			})

			u.notifyWithKeyboard(t.BITREFILLINVENTORYHEADER, nil, &tgbotapi.InlineKeyboardMarkup{inlineKeyboard}, 0)
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

			go u.track("gifts create", map[string]interface{}{"sats": sats})

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

			go u.track("gifts list", nil)

			u.notify(t.GIFTSLIST, t.T{"Gifts": gifts})
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

			go u.track("sats4ads on", map[string]interface{}{"rate": rate})

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

			go u.track("sats4ads off", nil)

			u.notify(t.SATS4ADSTOGGLE, t.T{"On": false})
		case opts["rates"].(bool):
			rates, err := getSats4AdsRates()
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "sats4ads", "Err": err.Error()})
				return
			}

			go u.track("sats4ads rates", nil)

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

			go u.track("sats4ads broadcast", map[string]interface{}{"sats": satoshis})

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
		handleHelp(u, "apps")
	}
}

func handleExternalAppCallback(u User, messageId int, cb *tgbotapi.CallbackQuery) (answer string) {
	parts := strings.Split(cb.Data[2:], "-")
	switch parts[0] {
	case "s4a":
		defer removeKeyboardButtons(cb)
		if parts[1] == "v" {
			hashfirst10chars := parts[2]
			confirmAdViewed(u, hashfirst10chars)
			go u.track("sats4ads viewed", nil)
		}
	case "etleneum":
		if parts[1] == "withdraw" {
			defer removeKeyboardButtons(cb)
			_, _, _, withdraw, err := etleneumLogin(u)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "Etleneum", "Err": err.Error()})
				return
			}
			go u.track("etleneum withdraw", nil)
			handleLNURL(u, withdraw, handleLNURLOpts{messageId: messageId})
		}
	case "microbet":
		if parts[1] == "withdraw" {
			defer removeKeyboardButtons(cb)
			balance, err := getMicrobetBalance(u)
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "Microbet", "Err": err.Error()})
				return translate(t.FAILURE, u.Locale)
			}

			go u.track("microbet withdraw", map[string]interface{}{"sats": balance})
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

			go u.track("microbet place-bet", nil)

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
			packages := item.Packages
			if len(packages) <= idx {
				u.notify(t.ERROR, t.T{"App": "Bitrefill", "Err": "not found"})
				return
			}
			pack = packages[idx]
			appendTextToMessage(cb, fmt.Sprintf("%v %s", pack.Value, item.Currency))

			phone := parts[4]
			handleProcessBitrefillOrder(u, item, pack, &phone)
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

			go u.track("bitclouds create-finish", map[string]interface{}{"image": image})

			appendTextToMessage(cb, image)
		case "status":
			host := unescapeBitcloudsHost(parts[2])
			appendTextToMessage(cb, host)
			showBitcloudStatus(u, host)
		default: // sats to topup
			sats, err := strconv.Atoi(parts[1])
			host := unescapeBitcloudsHost(parts[2])
			if err != nil {
				u.notify(t.ERROR, t.T{"App": "bitclouds", "Err": err.Error()})
				return
			}
			appendTextToMessage(cb, host)
			topupBitcloud(u, host, sats)

			go u.track("bitclouds topup", map[string]interface{}{"host": host})
		}
	}

	return
}
