package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/fiatjaf/lntxbot/t"
	"github.com/tidwall/gjson"
)

func handleReply(ctx context.Context) {
	u := ctx.Value("initiator").(User)
	message := ctx.Value("message").(*tgbot.Message)
	inreplyto := message.ReplyToMessage.MessageID

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
				send(ctx, u, t.ERROR, t.T{"Err": "Invalid satoshi amount."})
			}
			handlePayVariableAmount(u, int64(sats*1000), data, message.MessageID)
		case "lnurlpay-amount":
			sats, err := strconv.ParseFloat(message.Text, 64)
			if err != nil {
				send(ctx, u, t.ERROR, t.T{"Err": "Invalid satoshi amount."})
			}
			handleLNURLPayAmount(u, int64(sats*1000), data, message.MessageID)
		case "lnurlpay-comment":
			handleLNURLPayComment(u, message.Text, data, message.MessageID)
		case "bitrefill":
			value, err := strconv.ParseFloat(message.Text, 64)
			if err != nil {
				send(ctx, u, t.ERROR, t.T{"Err": "Invalid satoshi amount."})
			}

			// get item and package info
			item, ok := bitrefillInventory[data.Get("item").String()]
			if !ok {
				send(ctx, u, t.ERROR, t.T{"App": "Bitrefill", "Err": "not found"})
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
