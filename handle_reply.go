package main

import (
	"fmt"
	"strconv"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/go-telegram-bot-api/telegram-bot-api"
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
		case "lnurlpay":
			sats, err := strconv.ParseFloat(message.Text, 64)
			if err != nil {
				u.notify(t.ERROR, t.T{"Err": "Invalid satoshi amount."})
			}

			handleLNURLPayConfirmation(u,
				int64(sats*1000),
				data.Get("u").String(),
				data.Get("m").String(),
				message.MessageID,
			)
		default:
			log.Debug().Int("userId", u.Id).Int("message", inreplyto).Str("type", data.Get("type").String()).
				Msg("reply to bot message unhandled procedure")
		}
	}
}
