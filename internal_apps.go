package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api"
)

// hide and reveal
func getHiddenMessage(redisKey string) (sourceuser int, id, content, preview string, satoshis int, err error) {
	content, err = rds.Get(redisKey).Result()
	if err != nil {
		return
	}

	keyparts := strings.Split(redisKey, ":")
	satoshis, err = strconv.Atoi(keyparts[3])
	if err != nil {
		return
	}

	sourceuser, err = strconv.Atoi(keyparts[1])
	if err != nil {
		return
	}

	id = keyparts[2]

	preview = fmt.Sprintf("A message is hidden here. %d sat needed to unlock.", satoshis)
	contentparts := strings.SplitN(content, "~", 2)
	if len(contentparts) == 2 {
		preview = contentparts[0]
		content = contentparts[1]
	}

	return
}

func revealKeyboard(fullRedisKey string, sats int) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("Pay %d sat to reveal the full message", sats),
				fmt.Sprintf("reveal=%s", fullRedisKey),
			),
		),
	)
}
