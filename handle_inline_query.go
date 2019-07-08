package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/kballard/go-shellquote"
	"github.com/lucsky/cuid"
)

func handleInlineQuery(q *tgbotapi.InlineQuery) {
	var (
		u    User
		err  error
		resp tgbotapi.APIResponse
		argv []string
		text string
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
	if err != nil || len(argv) < 1 || (len(argv) < 2 && argv[0] != "reveal") {
		goto answerEmpty
	}

	switch argv[0] {
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

		result := tgbotapi.NewInlineQueryResultPhoto("inv-"+argv[1], qrurl)
		result.Title = argv[1] + " sat"
		result.Description = "Payment request for " + argv[1] + " sat"
		result.ThumbURL = qrurl
		result.Caption = bolt11

		resp, err = bot.AnswerInlineQuery(tgbotapi.InlineConfig{
			InlineQueryID: q.ID,
			Results:       []interface{}{result},
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

		result := tgbotapi.NewInlineQueryResultArticle(
			fmt.Sprintf("give-%d-%d", u.Id, sats),
			fmt.Sprintf("Giving %d away", sats),
			fmt.Sprintf("%s is giving %d sat away!", u.AtName(), sats),
		)

		keyboard := giveawayKeyboard(u.Id, sats)
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

		result := tgbotapi.NewInlineQueryResultArticle(
			fmt.Sprintf("flip-%d-%d-%d", u.Id, sats, nparticipants),
			fmt.Sprintf("Lottery with entry fee of %d sat for %d participants", sats, nparticipants),
			fmt.Sprintf("Pay %d and get a chance to win %d! %d out of %d spots left!",
				sats, sats*nparticipants, nparticipants-1, nparticipants),
		)

		coinflipid := cuid.Slug()
		rds.SAdd("coinflip:"+coinflipid, u.Id)
		rds.Expire("coinflip:"+coinflipid, s.GiveAwayTimeout)
		keyboard := coinflipKeyboard(coinflipid, nparticipants, sats)
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

		result := tgbotapi.NewInlineQueryResultArticle(
			fmt.Sprintf("gifl-%d-%d-%d", u.Id, sats, nparticipants),
			fmt.Sprintf("Give out %d sat for one out of %d participants", sats, nparticipants),
			fmt.Sprintf("Join and get a chance to win %d! %d out of %d spots left!",
				sats, nparticipants, nparticipants),
		)

		giveflipid := cuid.Slug()
		keyboard := giveflipKeyboard(giveflipid, u.Id, nparticipants, sats)
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
			_, hiddenid, content, preview, satoshis, err := getHiddenMessage(hiddenkey)
			if err != nil {
				continue
			}

			result := tgbotapi.NewInlineQueryResultArticle(
				fmt.Sprintf("reveal-%s", hiddenkey),
				fmt.Sprintf("Hidden message %s: %s", hiddenid, content),
				preview,
			)

			keyboard := revealKeyboard(hiddenkey, satoshis)
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
