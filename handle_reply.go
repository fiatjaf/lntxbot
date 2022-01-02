package main

import (
	"context"
	"fmt"

	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/tidwall/gjson"
)

func handleReply(ctx context.Context) {
	u := ctx.Value("initiator").(User)
	message := ctx.Value("message").(*tgbotapi.Message)
	inreplyto := message.ReplyToMessage.MessageID

	key := fmt.Sprintf("reply:%d:%d", u.Id, inreplyto)
	if val, err := rds.Get(key).Result(); err != nil {
		log.Debug().Int("userId", u.Id).Int("message", inreplyto).
			Msg("reply to bot message doesn't have a stored procedure")
	} else {
		switch gjson.Parse(val).Get("type").String() {
		case "pay":
			msats, err := parseAmountString(message.Text)
			if err != nil {
				send(ctx, u, t.ERROR, t.T{"Err": "Invalid satoshi amount."})
			}
			handlePayVariableAmount(ctx, msats, val)
		case "lnurlpay-amount":
			msats, err := parseAmountString(message.Text)
			if err != nil {
				send(ctx, u, t.ERROR, t.T{"Err": "Invalid satoshi amount."})
			}
			handleLNURLPayAmount(ctx, msats, val)
		case "lnurlpay-comment":
			handleLNURLPayComment(ctx, message.Text, val)
		default:
			log.Debug().Int("userId", u.Id).Int("message", inreplyto).
				Str("type", gjson.Parse(val).Get("type").String()).
				Msg("reply to bot message unhandled procedure")
		}
	}
}
