package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/kballard/go-shellquote"
	"github.com/lucsky/cuid"
)

func handleInlineQuery(q *tgbotapi.InlineQuery) {
	var (
		u       User
		err     error
		resp    tgbotapi.APIResponse
		argv    []string
		text    string
		command string
	)

	u, err = loadUser(0, int(q.From.ID))
	if err != nil {
		log.Debug().Err(err).
			Str("username", q.From.UserName).
			Int("id", q.From.ID).
			Msg("unregistered user trying to use inline query")

		goto answerEmpty
	}

	text = strings.TrimSpace(q.Query)
	argv, err = shellquote.Split(text)
	if err != nil || len(argv) < 1 {
		goto answerEmpty
	}

	command = argv[0]
	if strings.HasPrefix(command, "/") {
		command = command[1:]
	}

	switch command {
	case "invoice", "receive", "fund":
		sats, err := strconv.Atoi(argv[1])
		if err != nil {
			goto answerEmpty
		}

		bolt11, _, qrpath, err := u.makeInvoice(sats, "inline-"+q.ID, "", nil, q.ID, "", false)
		if err != nil {
			log.Warn().Err(err).Msg("error making invoice on inline query.")
			goto answerEmpty
		}

		qrurl := s.ServiceURL + "/qr/" + qrpath

		resultphoto := tgbotapi.NewInlineQueryResultPhoto("inv-"+argv[1]+"-photo", qrurl)
		resultphoto.Title = argv[1] + " sat"
		resultphoto.Description = translateTemplate(t.INLINEINVOICERESULT, u.Locale, t.T{"Sats": argv[1]})
		resultphoto.ThumbURL = qrurl
		resultphoto.Caption = bolt11

		resultnophoto := tgbotapi.NewInlineQueryResultArticleHTML(
			"inv-"+argv[1]+"-nophoto",
			translateTemplate(t.INLINEINVOICERESULT, u.Locale, t.T{"Sats": argv[1]}),
			bolt11,
		)

		resp, err = bot.AnswerInlineQuery(tgbotapi.InlineConfig{
			InlineQueryID: q.ID,
			Results:       []interface{}{resultnophoto, resultphoto},
			IsPersonal:    true,
		})

		go func(qrpath string) {
			time.Sleep(30 * time.Second)
			os.Remove(qrpath)
		}(qrpath)
		goto responded
	case "giveaway":
		if len(argv) != 2 {
			goto answerEmpty
		}

		var sats int
		if sats, err = strconv.Atoi(argv[1]); err != nil {
			break
		}
		if !u.checkBalanceFor(sats, "giveaway") {
			break
		}

		result := tgbotapi.NewInlineQueryResultArticleHTML(
			fmt.Sprintf("give-%d-%d", u.Id, sats),
			translateTemplate(t.INLINEGIVEAWAYRESULT, u.Locale, t.T{"Sats": sats}),
			translateTemplate(t.GIVEAWAYMSG, u.Locale, t.T{"User": u.AtName(), "Sats": sats}),
		)

		keyboard := giveawayKeyboard(u.Id, sats, u.Locale)
		result.ReplyMarkup = &keyboard

		resp, err = bot.AnswerInlineQuery(tgbotapi.InlineConfig{
			InlineQueryID: q.ID,
			Results:       []interface{}{result},
			IsPersonal:    true,
		})
	case "coinflip", "lottery":
		if len(argv) < 2 {
			goto answerEmpty
		}

		var sats int
		if sats, err = strconv.Atoi(argv[1]); err != nil {
			break
		}
		if !u.checkBalanceFor(sats, "coinflip") {
			break
		}

		nparticipants := 2
		if len(argv) > 2 {
			if n, err := strconv.Atoi(argv[2]); err == nil {
				nparticipants = n
			}
		}

		result := tgbotapi.NewInlineQueryResultArticleHTML(
			fmt.Sprintf("flip-%d-%d-%d", u.Id, sats, nparticipants),
			translateTemplate(t.INLINECOINFLIPRESULT, u.Locale, t.T{
				"Sats":       sats,
				"MaxPlayers": nparticipants,
			}),
			translateTemplate(t.COINFLIPAD, u.Locale, t.T{
				"Sats":       sats,
				"Prize":      sats * nparticipants,
				"SpotsLeft":  nparticipants - 1,
				"MaxPlayers": nparticipants,
			}),
		)

		coinflipid := cuid.Slug()
		rds.SAdd("coinflip:"+coinflipid, u.Id)
		rds.Expire("coinflip:"+coinflipid, s.GiveAwayTimeout)
		keyboard := coinflipKeyboard(coinflipid, nparticipants, sats, u.Locale)
		result.ReplyMarkup = &keyboard

		resp, err = bot.AnswerInlineQuery(tgbotapi.InlineConfig{
			InlineQueryID: q.ID,
			Results:       []interface{}{result},
			IsPersonal:    true,
		})
	case "giveflip":
		if len(argv) < 3 {
			goto answerEmpty
		}

		var sats int
		if sats, err = strconv.Atoi(argv[1]); err != nil {
			break
		}
		if !u.checkBalanceFor(sats, "giveflip") {
			break
		}

		var nparticipants int
		if n, err := strconv.Atoi(argv[2]); err == nil {
			nparticipants = n
		}

		result := tgbotapi.NewInlineQueryResultArticleHTML(
			fmt.Sprintf("gifl-%d-%d-%d", u.Id, sats, nparticipants),
			translateTemplate(t.INLINEGIVEFLIPRESULT, u.Locale, t.T{
				"Sats":       sats,
				"MaxPlayers": nparticipants,
			}),
			translateTemplate(t.GIVEFLIPAD, u.Locale, t.T{
				"Sats":       sats,
				"SpotsLeft":  nparticipants,
				"MaxPlayers": nparticipants,
			}),
		)

		giveflipid := cuid.Slug()
		keyboard := giveflipKeyboard(giveflipid, u.Id, nparticipants, sats, u.Locale)
		result.ReplyMarkup = &keyboard

		resp, err = bot.AnswerInlineQuery(tgbotapi.InlineConfig{
			InlineQueryID: q.ID,
			Results:       []interface{}{result},
			IsPersonal:    true,
		})
	case "reveal":
		hiddenid := "*"
		if len(argv) == 2 {
			hiddenid = argv[1]
		}

		hiddenkeys := rds.Keys(fmt.Sprintf("hidden:%d:%s:*", u.Id, hiddenid)).Val()
		results := make([]interface{}, len(hiddenkeys))
		for i, hiddenkey := range hiddenkeys {
			_, hiddenid, content, preview, satoshis, err := getHiddenMessage(hiddenkey, u.Locale)
			if err != nil {
				continue
			}

			result := tgbotapi.NewInlineQueryResultArticleHTML(
				fmt.Sprintf("reveal-%s", hiddenkey),
				translateTemplate(t.INLINEHIDDENRESULT, u.Locale, t.T{"HiddenId": hiddenid, "Content": content}),
				preview,
			)

			keyboard := revealKeyboard(hiddenkey, satoshis, u.Locale)
			result.ReplyMarkup = &keyboard
			results[i] = result
		}

		resp, err = bot.AnswerInlineQuery(tgbotapi.InlineConfig{
			InlineQueryID: q.ID,
			Results:       results,
			IsPersonal:    true,
		})
	default:
		goto answerEmpty
	}

responded:
	if err != nil || !resp.Ok {
		log.Warn().Err(err).
			Str("resp", resp.Description).
			Msg("error answering inline query")
	}
	return

answerEmpty:
	bot.AnswerInlineQuery(tgbotapi.InlineConfig{
		InlineQueryID: q.ID,
		Results:       []interface{}{},
	})
}
