package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
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

	if len(argv) < 2 {
		goto answerEmpty
	}

	switch command {
	case "invoice", "receive", "fund":
		sats, err := strconv.Atoi(argv[1])
		if err != nil {
			goto answerEmpty
		}

		bolt11, _, qrpath, err := u.makeInvoice(makeInvoiceArgs{
			Msatoshi:  int64(sats) * 1000,
			MessageId: q.ID,
		})
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

		go u.track("make invoice", map[string]interface{}{
			"sats":   sats,
			"inline": true,
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
		if !u.checkBalanceFor(sats, "giveaway", nil) {
			break
		}

		go u.track("giveaway created", map[string]interface{}{
			"sats":   sats,
			"inline": true,
		})

		result := tgbotapi.NewInlineQueryResultArticleHTML(
			fmt.Sprintf("give-%d-%d", u.Id, sats),
			translateTemplate(t.INLINEGIVEAWAYRESULT, u.Locale, t.T{"Sats": sats}),
			translateTemplate(t.GIVEAWAYMSG, u.Locale, t.T{"User": u.AtName(), "Sats": sats}),
		)
		result.ReplyMarkup = giveawayKeyboard(u.Id, sats, u.Locale)

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

		if !canCreateCoinflip(u.Id) {
			u.notify(t.RATELIMIT, nil)
			return
		}
		if !canJoinCoinflip(u.Id) {
			u.notify(t.OVERQUOTA, t.T{"App": "coinflip"})
			return
		}
		if !u.checkBalanceFor(sats, "coinflip", nil) {
			break
		}

		nparticipants := 2
		if len(argv) > 2 {
			if n, err := strconv.Atoi(argv[2]); err == nil {
				nparticipants = n
			}
		}

		go u.track("coinflip created", map[string]interface{}{
			"sats":   sats,
			"n":      nparticipants,
			"inline": true,
		})

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

		result.ReplyMarkup = coinflipKeyboard("", u.Id, nparticipants, sats, u.Locale)

		resp, err = bot.AnswerInlineQuery(tgbotapi.InlineConfig{
			InlineQueryID: q.ID,
			Results:       []interface{}{result},
			IsPersonal:    true,
		})

		go func() {
			// after a while save this to limit coinflip creation per user
			time.Sleep(time.Second * 120)
			rds.Set(fmt.Sprintf("recentcoinflip:%d", u.Id), "t", time.Minute*30)
		}()
	case "giveflip":
		if len(argv) < 3 {
			goto answerEmpty
		}

		var sats int
		if sats, err = strconv.Atoi(argv[1]); err != nil {
			break
		}

		if !canCreateGiveflip(u.Id) {
			u.notify(t.RATELIMIT, nil)
			return
		}
		if !canJoinGiveflip(u.Id) {
			u.notify(t.OVERQUOTA, t.T{"App": "giveflip"})
			return
		}
		if !u.checkBalanceFor(sats, "giveflip", nil) {
			break
		}

		var nparticipants int
		if n, err := strconv.Atoi(argv[2]); err == nil {
			nparticipants = n
		}

		go u.track("giveflip created", map[string]interface{}{
			"sats":   sats,
			"inline": true,
		})

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
		result.ReplyMarkup = giveflipKeyboard(giveflipid,
			u.Id, nparticipants, sats, u.Locale)

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

		hiddenkeys := rds.Keys(fmt.Sprintf("hidden:%d:%s", u.Id, hiddenid)).Val()

		results := make([]interface{}, len(hiddenkeys))
		for i, hiddenkey := range hiddenkeys {
			_, hiddenid, hiddenmessage, err := getHiddenMessage(hiddenkey, u.Locale)
			if err != nil {
				continue
			}

			result := tgbotapi.NewInlineQueryResultArticleHTML(
				fmt.Sprintf("reveal-%s", hiddenkey),
				translateTemplate(t.INLINEHIDDENRESULT, u.Locale, t.T{
					"HiddenId": hiddenid,
					"Message":  hiddenmessage,
				}),
				hiddenmessage.Preview,
			)

			result.ReplyMarkup = revealKeyboard(hiddenkey, hiddenmessage, 0, u.Locale)
			results[i] = result
		}

		if len(results) > 0 {
			go u.track("reveal query", map[string]interface{}{
				"inline": true,
			})
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
