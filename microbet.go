package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"gopkg.in/jmcvetta/napping.v3"
)

type MicrobetBet struct {
	Id           string `json:"id"`
	Description  string `json:"description"`
	Amount       int    `json:"amount"`
	Backers      int    `json:"backers"`
	GameDatetime string `json:"game_datetime"`
	TotalUsers   int    `json:"total_users"`
	Exact        bool   `json:"exact"`
	Sport        string `json:"sport"`
}

type MyMicrobetBet struct {
	MicrobetBet
	UserBack  int     `json:"userBack"`
	UserLay   int     `json:"userLay"`
	Canceled  bool    `json:"canceled"`
	Closed    bool    `json:"closed"`
	WonAmount float64 `json:"wonAmount"`
}

func (mb MyMicrobetBet) Layers() int      { return mb.TotalUsers - mb.Backers }
func (mb MyMicrobetBet) BackIcon() string { return "🔵" }
func (mb MyMicrobetBet) LayIcon() string  { return "🔴" }
func (mb MyMicrobetBet) AmountWon() string {
	return fmt.Sprintf("%.0f", mb.WonAmount-float64(mb.Amount))
}
func (mb MyMicrobetBet) AmountLost() string {
	return fmt.Sprintf("%.0f", float64(mb.Amount)-mb.WonAmount)
}

func getMicrobetBets() (bets []MicrobetBet, err error) {
	var betdata struct {
		Data []MicrobetBet `json:"data"`
	}
	resp, err := napping.Post("https://microbet.fun/api/v1/get_bets", nil, &betdata, nil)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = errors.New("microbet.fun returned an invalid response.")
		return
	}

	bets = betdata.Data
	return
}

func getMicrobetBet(betId string) (_ MicrobetBet, err error) {
	bets, err := getMicrobetBets()
	if err != nil {
		return
	}

	for _, bet := range bets {
		if bet.Id == betId {
			return bet, nil
		}
	}

	err = errors.New("Bet not found.")
	return
}

func placeMicrobetBet(user User, messageId int, betId string, back bool) (err error) {
	session := &napping.Session{
		Client: &http.Client{
			Jar: &microbetCookiejar{user},
		},
	}

	var payreq struct {
		RHash          string `json:"r_hash"`
		PaymentRequest string `json:"payment_request"`
	}
	resp, err := session.Post("https://microbet.fun/api/v1/create_addin_invoice", struct {
		BetId string `json:"betId"`
		Back  bool   `json:"back"`
	}{betId, back}, &payreq, nil)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = errors.New("microbet.fun returned an invalid response.")
		return
	}

	inv, err := ln.Call("decodepay", payreq.PaymentRequest)
	if err != nil {
		return errors.New("Failed to decode invoice.")
	}
	err = user.actuallySendExternalPayment(
		messageId, payreq.PaymentRequest, inv, inv.Get("msatoshi").Int(),
		fmt.Sprintf("%s.microbet.%s.%d", s.ServiceId, betId, user.Id), map[string]interface{}{},
		func(
			u User,
			messageId int,
			msatoshi float64,
			msatoshi_sent float64,
			preimage string,
			tag string,
			hash string,
		) {
			// on success
			paymentHasSucceeded(u, messageId, msatoshi, msatoshi_sent, preimage, "microbet", hash)

			// acknowledge bet on microbet.fun
			var paidreq struct {
				Settled bool `json:"settled"`
			}
			resp, err = session.Post("https://microbet.fun/api/v1/wait_addin_invoice", struct {
				RHash string `json:"r_hash"`
				BetId string `json:"bet_id"`
				Back  bool   `json:"back"`
			}{payreq.RHash, betId, back}, &paidreq, nil)
			if err != nil {
				u.notifyAsReply(t.ERROR, t.T{"App": "Microbet", "Err": err.Error()}, messageId)
			}
			if resp.Status() >= 300 {
				u.notifyAsReply(t.ERROR, t.T{"App": "Microbet", "Err": "microbet.fun returned an invalid response, please report."}, messageId)
				return
			}
			if !paidreq.Settled {
				u.notifyAsReply(t.MICROBETPAIDBUTNOTCONFIRMED, nil, messageId)
				return
			}

			u.notifyAsReply(t.MICROBETPLACED, nil, messageId)
		},
		paymentHasFailed,
	)
	if err != nil {
		return
	}

	return
}

func getMyMicrobetBets(user User) (mybets []MyMicrobetBet, err error) {
	session := &napping.Session{
		Client: &http.Client{
			Jar: &microbetCookiejar{user},
		},
	}

	var mybetdata struct {
		Data []MyMicrobetBet `json:"data"`
	}
	resp, err := session.Post("https://microbet.fun/api/v1/my_bets", struct {
		Page     int `json:"page"`
		PageSize int `json:"pageSize"`
	}{1, 500}, &mybetdata, nil)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = errors.New("microbet.fun returned an invalid response.")
		return
	}

	mybets = mybetdata.Data
	return
}

func getMicrobetBalance(user User) (_ int64, err error) {
	session := &napping.Session{
		Client: &http.Client{
			Jar: &microbetCookiejar{user},
		},
	}

	var balance struct {
		Success bool  `json:"success"`
		Balance int64 `json:"balance"`
	}
	resp, err := session.Get("https://microbet.fun/api/v1/get_balance", nil, &balance, nil)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = errors.New("microbet.fun returned an invalid response.")
		return
	}

	if !balance.Success {
		err = errors.New("microbet.fun balance request failed.")
		return
	}

	return balance.Balance, nil
}

func withdrawMicrobet(user User, sats int) (err error) {
	session := &napping.Session{
		Client: &http.Client{
			Jar: &microbetCookiejar{user},
		},
	}

	bolt11, _, _, err := user.makeInvoice(sats, "withdraw from microbet.fun", "", nil, nil, "", "microbet", true)
	if err != nil {
		return
	}

	var success struct {
		PaymentStatus string  `json:"payment_status"`
		Balance       float64 `json:"balance"`
	}
	resp, err := session.Post("https://microbet.fun/api/v1/withdraw", struct {
		PaymentRequest string `json:"payment_request"`
	}{bolt11}, &success, nil)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = errors.New("microbet.fun returned an invalid response.")
		return
	}
	if success.PaymentStatus != "success" {
		err = errors.New("microbet.fun withdraw failed.")
		return
	}

	return
}

type microbetCookiejar struct {
	user User
}

func (j *microbetCookiejar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	data := MicrobetData{
		Cookies: cookies,
	}

	err := j.user.setAppData("microbet", data)
	if err != nil {
		log.Error().Err(err).Str("user", j.user.Username).Msg("error saving microbet cookies")
	}
}

func (j *microbetCookiejar) Cookies(u *url.URL) []*http.Cookie {
	var data MicrobetData
	err := j.user.getAppData("microbet", &data)
	if err != nil {
		log.Error().Err(err).Str("user", j.user.Username).Msg("error loading microbet cookies")
		return nil
	}

	return data.Cookies
}

type MicrobetData struct {
	Cookies []*http.Cookie `json:"cookies"`
}

func microbetKeyboard() (inlinekeyboard [][]tgbotapi.InlineKeyboardButton, err error) {
	bets, err := getMicrobetBets()
	if err != nil {
		return
	}

	inlinekeyboard = make([][]tgbotapi.InlineKeyboardButton, 2*len(bets))
	for i, bet := range bets {
		parts := strings.Split(bet.Description, "→")
		gamename := parts[0]
		backbet := parts[1]
		if bet.Exact {
			backbet += " (exact)"
		}

		inlinekeyboard[i*2] = []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonURL(
				fmt.Sprintf("(%d) %s", bet.Amount, gamename),
				"https://www.google.com/search?q="+gamename,
			),
		}
		inlinekeyboard[i*2+1] = []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%s (%d)", backbet, bet.Backers),
				fmt.Sprintf("x=microbet-%s-true", bet.Id),
			),
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("NOT (%d)", bet.TotalUsers-bet.Backers),
				fmt.Sprintf("x=microbet-%s-false", bet.Id),
			),
		}
	}

	return
}
