package main

import (
	"fmt"
	"strconv"
	"strings"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/lucsky/cuid"
)

// hide and reveal
func getHiddenMessage(redisKey, locale string) (sourceuser int, id, content, preview string, satoshis int, err error) {
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

	preview = translateTemplate(t.HIDDENDEFAULTPREVIEW, locale, t.T{"Sats": satoshis})
	contentparts := strings.SplitN(content, "~", 2)
	if len(contentparts) == 2 {
		preview = contentparts[0]
		content = contentparts[1]
	}

	return
}

func revealKeyboard(fullRedisKey string, sats int, locale string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf(translateTemplate(t.HIDDENREVEALBUTTON, locale, t.T{"Sats": sats})),
				fmt.Sprintf("reveal=%s", fullRedisKey),
			),
		),
	)
}

// giveaway
func giveawayKeyboard(giverId, sats int, locale string) tgbotapi.InlineKeyboardMarkup {
	giveawayid := cuid.Slug()
	buttonData := fmt.Sprintf("give=%d-%d-%s", giverId, sats, giveawayid)

	rds.Set("giveaway:"+giveawayid, buttonData, s.GiveAwayTimeout)

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				translate(t.CANCEL, locale),
				fmt.Sprintf("cancel=%d", giverId),
			),
			tgbotapi.NewInlineKeyboardButtonData(
				translate(t.GIVEAWAYCLAIM, locale),
				buttonData,
			),
		),
	)
}

// giveflip
func giveflipKeyboard(giveflipid string, giverId, nparticipants, sats int, locale string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				translate(t.CANCEL, locale),
				fmt.Sprintf("cancel=%d", giverId),
			),
			tgbotapi.NewInlineKeyboardButtonData(
				translate(t.GIVEFLIPJOIN, locale),
				fmt.Sprintf("gifl=%d-%d-%d-%s", giverId, nparticipants, sats, giveflipid),
			),
		),
	)
}

// coinflip
func coinflipKeyboard(coinflipid string, nparticipants, sats int, locale string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				translate(t.COINFLIPJOIN, locale),
				fmt.Sprintf("flip=%d-%d-%s", nparticipants, sats, coinflipid),
			),
		),
	)
}

// fundraise
func fundraiseKeyboard(
	fundraiseid string,
	receiverId int,
	nparticipants int,
	sats int,
	locale string,
) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				translate(t.FUNDRAISEJOIN, locale),
				fmt.Sprintf("raise=%d-%d-%d-%s", receiverId, nparticipants, sats, fundraiseid),
			),
		),
	)
}
