package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/kballard/go-shellquote"
	"github.com/lucsky/cuid"
)

func handleInlineQuery(ctx context.Context, q *tgbotapi.InlineQuery) {
	var (
		u       User
		err     error
		resp    tgbotapi.APIResponse
		argv    []string
		text    string
		command string
	)

	u, err = loadTelegramUser(int(q.From.ID))
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
		msats, err := parseAmountString(argv[1])
		if err != nil {
			goto answerEmpty
		}

		bolt11, _, err := u.makeInvoice(ctx, &MakeInvoiceArgs{
			Msatoshi: msats,
		})
		if err != nil {
			log.Warn().Err(err).Msg("error making invoice on inline query.")
			goto answerEmpty
		}

		result := tgbotapi.NewInlineQueryResultArticleHTML(
			"inv-"+argv[1],
			translateTemplate(ctx, t.INLINEINVOICERESULT, t.T{"Sats": argv[1]}),
			bolt11,
		)

		resp, err = bot.AnswerInlineQuery(tgbotapi.InlineConfig{
			InlineQueryID: q.ID,
			Results:       []interface{}{result},
			IsPersonal:    true,
		})

		go u.track("make invoice", map[string]interface{}{
			"sats":   msats / 1000,
			"inline": true,
		})
		goto responded
	case "give", "giveaway":
		if len(argv) != 2 && len(argv) != 3 {
			goto answerEmpty
		}
		if len(argv) == 3 && command == "giveaway" {
			goto answerEmpty
		}

		msats, err := parseAmountString(argv[1])
		if err != nil {
			break
		}
		if !u.checkBalanceFor(ctx, msats, command) {
			break
		}
		sats := int(msats / 1000)

		var recv string
		if len(argv) == 3 {
			recv = argv[2]
			recv = strings.TrimSpace(recv)
			recv = strings.ToLower(recv)
			recv = strings.TrimPrefix(recv, "@")
		}

		go u.track(command+" created", map[string]interface{}{
			"sats":     sats,
			"inline":   true,
			"receiver": recv,
		})

		result := tgbotapi.NewInlineQueryResultArticleHTML(
			fmt.Sprintf("gv-%d-%d-%s", u.Id, sats, recv),
			translateTemplate(ctx, t.INLINEGIVEAWAYRESULT, t.T{
				"Sats":     sats,
				"Receiver": recv,
			}),
			translateTemplate(ctx, t.GIVEAWAYMSG, t.T{
				"User":     u.AtName(ctx),
				"Sats":     sats,
				"Receiver": recv,
				"Away":     command == "giveaway",
			}),
		)
		result.ReplyMarkup = giveawayKeyboard(ctx, u.Id, sats, recv)

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

		if !u.checkBalanceFor(ctx, int64(sats*1000), "coinflip") {
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
			translateTemplate(ctx, t.INLINECOINFLIPRESULT, t.T{
				"Sats":       sats,
				"MaxPlayers": nparticipants,
			}),
			translateTemplate(ctx, t.COINFLIPAD, t.T{
				"Sats":       sats,
				"Prize":      sats * nparticipants,
				"SpotsLeft":  nparticipants - 1,
				"MaxPlayers": nparticipants,
			}),
		)

		result.ReplyMarkup = coinflipKeyboard(ctx, "", u.Id, nparticipants, sats)

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

		msats, err := parseAmountString(argv[1])
		if err != nil {
			break
		}
		sats := int(msats / 1000)

		if !u.checkBalanceFor(ctx, msats, "giveflip") {
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
			translateTemplate(ctx, t.INLINEGIVEFLIPRESULT, t.T{
				"Sats":       sats,
				"MaxPlayers": nparticipants,
			}),
			translateTemplate(ctx, t.GIVEFLIPAD, t.T{
				"Sats":       sats,
				"SpotsLeft":  nparticipants,
				"MaxPlayers": nparticipants,
			}),
		)

		giveflipid := cuid.Slug()
		result.ReplyMarkup = giveflipKeyboard(ctx, giveflipid, u.Id, nparticipants, sats)

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
			_, hiddenid, hiddenmessage, err := getHiddenMessage(ctx, hiddenkey)
			if err != nil {
				continue
			}

			result := tgbotapi.NewInlineQueryResultArticleHTML(
				fmt.Sprintf("reveal-%s", hiddenkey),
				translateTemplate(ctx, t.INLINEHIDDENRESULT, t.T{
					"HiddenId": hiddenid,
					"Message":  hiddenmessage,
				}),
				hiddenmessage.Preview,
			)

			result.ReplyMarkup = revealKeyboard(ctx, hiddenkey, hiddenmessage, 0)
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
	case "show":
		if argv[1] == "id" {
			resp, err = bot.AnswerInlineQuery(tgbotapi.InlineConfig{
				InlineQueryID: q.ID,
				Results: []interface{}{
					tgbotapi.NewInlineQueryResultArticle("tgid",
						fmt.Sprintf("Telegram ID: %d", u.TelegramId),
						fmt.Sprintf("Your Telegram ID: %d", u.TelegramId),
					),
					tgbotapi.NewInlineQueryResultArticle("lntxbotid",
						fmt.Sprintf("@%s ID: %d", s.ServiceId, u.Id),
						fmt.Sprintf("Your @%s ID: %d", s.ServiceId, u.Id),
					),
				},
				IsPersonal: true,
			})
		}
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
