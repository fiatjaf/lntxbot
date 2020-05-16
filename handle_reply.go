package main

import (
	"fmt"
	"strconv"

	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/tidwall/gjson"
)

func handleReply(u User, message *tgbotapi.Message, inreplyto int) {
	key := fmt.Sprintf("reply:%d:%d", u.Id, inreplyto)
	if val, err := rds.Get(key).Result(); err != nil {
		log.Debug().Int("userId", u.Id).Int("message", inreplyto).
			Msg("reply to bot message doesn't have a stored procedure")
	} else {
		data := gjson.Parse(val)
		switch data.Get("type").String() {
		case "pay":
			sats, err := strconv.ParseFloat(message.Text, 64)
			if err != nil {
				u.notify(t.ERROR, t.T{"Err": "Invalid satoshi amount."})
			}
			handlePayVariableAmount(u, int64(sats*1000), data, message.MessageID)
		case "lnurlpay":
			sats, err := strconv.ParseFloat(message.Text, 64)
			if err != nil {
				u.notify(t.ERROR, t.T{"Err": "Invalid satoshi amount."})
			}
			handleLNURLPayConfirmation(u, int64(sats*1000), data, message.MessageID)
		case "bitrefill":
			value, err := strconv.ParseFloat(message.Text, 64)
			if err != nil {
				u.notify(t.ERROR, t.T{"Err": "Invalid satoshi amount."})
			}

			// get item and package info
			item, ok := bitrefillInventory[data.Get("item").String()]
			if !ok {
				u.notify(t.ERROR, t.T{"App": "Bitrefill", "Err": "not found"})
				return
			}

			phone := data.Get("phone").String()
			handleProcessBitrefillOrder(u, item, BitrefillPackage{Value: value}, &phone)
		default:
			log.Debug().Int("userId", u.Id).Int("message", inreplyto).Str("type", data.Get("type").String()).
				Msg("reply to bot message unhandled procedure")
		}
	}
}
