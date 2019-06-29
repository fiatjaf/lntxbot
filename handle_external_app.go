package main

import (
	"fmt"
	"strings"

	"github.com/docopt/docopt-go"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

func handleExternalApp(u User, opts docopt.Opts) {
	switch {
	case opts["microbet"].(bool):
		if opts["bets"].(bool) {
			// list my bets
			bets, err := getMyMicrobetBets(u)
			if err != nil {
				u.notify(err.Error())
				return
			}

			message := make([]string, 0, len(bets))
			for _, bet := range bets {
				result := "open"
				if bet.Canceled {
					result = "canceled"
				} else if bet.Closed {
					if bet.WonAmount != 0 {
						result = fmt.Sprintf("won %d", bet.WonAmount)
					} else {
						result = "lost"
					}
				}

				if bet.UserBack > 0 {
					message = append(message, fmt.Sprintf("<code>%s</code> <code>%d</code> 🔵 %d/%d × %d ~ <i>%s</i>",
						bet.Description,
						bet.Amount,
						bet.UserBack,
						bet.Backers,
						bet.TotalUsers-bet.Backers,
						result,
					))
				}

				if bet.UserLay > 0 {
					message = append(message, fmt.Sprintf("<code>%s</code> <code>%d</code> 🔴 %d/%d × %d ~ <i>%s</i>",
						bet.Description,
						bet.Amount,
						bet.UserLay,
						bet.TotalUsers-bet.Backers,
						bet.Backers,
						result,
					))
				}
			}

			u.notify("<b>[Microbet]</b> Your bets\n" + strings.Join(message, "\n"))
		} else if opts["balance"].(bool) {

		} else if opts["withdraw"].(bool) {

		} else {
			// list available bets as actionable buttons
			bets, err := getMicrobetBets()
			if err != nil {
				u.notify(err.Error())
				return
			}

			inlinekeyboard := make([][]tgbotapi.InlineKeyboardButton, 2*len(bets))
			for i, bet := range bets {
				parts := strings.Split(bet.Description, "→")
				gamename := parts[0]
				backbet := parts[1]
				if bet.Exact {
					backbet += " (exact)"
				}

				inlinekeyboard[i*2] = []tgbotapi.InlineKeyboardButton{
					tgbotapi.NewInlineKeyboardButtonURL(
						fmt.Sprintf("%s (%d sat)", gamename, bet.Amount),
						"https://www.google.com/search?q="+gamename,
					),
				}
				inlinekeyboard[i*2+1] = []tgbotapi.InlineKeyboardButton{
					tgbotapi.NewInlineKeyboardButtonData(
						fmt.Sprintf("%s (%d)", backbet, bet.Backers),
						fmt.Sprintf("app=microbet-%s-true", bet.Id),
					),
					tgbotapi.NewInlineKeyboardButtonData(
						fmt.Sprintf("NOT (%d)", bet.TotalUsers-bet.Backers),
						fmt.Sprintf("app=microbet-%s-false", bet.Id),
					),
				}
			}

			chattable := tgbotapi.NewMessage(u.ChatId, "<b>[Microbet]</b> Bet on one of these predictions:")
			chattable.ParseMode = "HTML"
			chattable.BaseChat.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{inlinekeyboard}
			bot.Send(chattable)
		}
	}
}

func handleExternalAppCallback(u User, messageId int, data string) (answer string) {
	parts := strings.Split(data[4:], "-")
	switch parts[0] {
	case "microbet":
		betId := parts[1]
		back := parts[2] == "true"
		bet, err := getMicrobetBet(betId)
		if err != nil {
			return "Bet not available."
		}

		// post a notification message to identify this bet attempt
		gamename := strings.Split(bet.Description, "→")[0]
		message := u.notify(fmt.Sprintf("Placing bet on <b>%s</b>.", strings.TrimSpace(gamename)))

		err = placeMicrobetBet(u, message.MessageID, betId, back)
		if err != nil {
			u.notify(err.Error())
		}

		return "Placing bet."
	}

	return
}
