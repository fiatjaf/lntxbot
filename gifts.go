package main

import (
	"errors"
	"fmt"
	"time"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"gopkg.in/jmcvetta/napping.v3"
)

type GiftsOrder struct {
	OrderId          string `json:"orderId"`
	ChargeId         string `json:"chargeId"`
	Status           string `json:"status"`
	LightningInvoice struct {
		PayReq string `json:"payreq"`
	} `json:"lightning_invoice"`
}

type GiftsGift struct {
	OrderId        string `json:"orderId"`
	Spent          bool   `json:"spent"`
	WithdrawalInfo struct {
		CreatedAt struct {
			Seconds int64 `json:"_seconds"`
		} `json:"createdAt"`
		Reference string `json:"reference"`
	} `json:"withdrawalInfo"`
	Amount int `json:"amount"`
}

type GiftsData struct {
	Gifts []string `json:"gifts"`
}

func (g GiftsGift) WithdrawDate() string {
	return time.Unix(g.WithdrawalInfo.CreatedAt.Seconds, 0).Format("2 Jan 2006 at 3:04PM")
}

func (g GiftsGift) RedeemerURL() string {
	inv, _ := ln.Call("decodepay", g.WithdrawalInfo.Reference)
	return nodeLink(inv.Get("payee").String())
}

func createGift(user User, sats int, messageId int) error {
	var resp GiftsOrder
	_, err := napping.Post("https://api.lightning.gifts/create", struct {
		Amount int `json:"amount"`
	}{sats}, &resp, nil)
	if err != nil {
		return err
	}
	if resp.LightningInvoice.PayReq == "" {
		return errors.New("invalid response from lightning.gifts")
	}

	inv, err := ln.Call("decodepay", resp.LightningInvoice.PayReq)
	if err != nil {
		return errors.New("Failed to decode invoice.")
	}

	return user.actuallySendExternalPayment(
		messageId, resp.LightningInvoice.PayReq, inv, inv.Get("msatoshi").Int(),
		fmt.Sprintf("%s.gifts.%s.%d", s.ServiceId, resp.OrderId, user.Id), map[string]interface{}{},
		func(
			u User,
			messageId int,
			msatoshi float64,
			msatoshi_sent float64,
			preimage string,
			hash string,
		) {
			// on success
			paymentHasSucceeded(u, messageId, msatoshi, msatoshi_sent, preimage, hash)

			// wait for gift to be available
			for i := 0; i < 10; i++ {
				var status GiftsOrder
				napping.Get("https://api.lightning.gifts/status/"+resp.ChargeId, nil, &status, nil)
				if status.Status == "paid" {
					break
				}
				time.Sleep(time.Second * time.Duration(i))
			}

			// we already have the order id which is the gift url
			u.notifyAsReply(t.GIFTSCREATED, t.T{"OrderId": resp.OrderId}, messageId)

			// save gift info as user data
			var data GiftsData
			err := user.getAppData("gifts", &data)
			if err != nil {
				u.notify(t.GIFTSFAILEDSAVE, t.T{"Err": err.Error()})
				return
			}
			data.Gifts = append(data.Gifts, resp.OrderId)

			// limit stored gifts to 50
			if len(data.Gifts) > 50 {
				data.Gifts = data.Gifts[len(data.Gifts)-50:]
			}

			err = user.setAppData("gifts", data.Gifts)
			if err != nil {
				u.notify(t.GIFTSFAILEDSAVE, t.T{"Err": err.Error()})
				return
			}
		},
		func(
			u User,
			messageId int,
			hash string,
		) {
			// on failure
			paymentHasFailed(u, messageId, hash)
		},
	)
}

func getGift(orderId string) (gift GiftsGift, err error) {
	_, err = napping.Get("https://api.lightning.gifts/gift/"+orderId, nil, &gift, nil)
	return
}
