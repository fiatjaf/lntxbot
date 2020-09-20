package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/fiatjaf/lntxbot/t"
	"gopkg.in/jmcvetta/napping.v3"
)

type GiftsOrder struct {
	OrderId          string `json:"orderId"`
	ChargeId         string `json:"chargeId"`
	Status           string `json:"status"`
	LightningInvoice struct {
		PayReq string `json:"payreq"`
	} `json:"lightningInvoice"`
}

type GiftsError struct {
	Message string `json:"message"`
}

type GiftsGift struct {
	OrderId        string `json:"orderId"`
	Spent          bool   `json:"spent"`
	WithdrawalInfo struct {
		CreatedAt struct {
			Seconds int64 `json:"_seconds"`
		} `json:"createdAt"`

		// these two fields are the same, the API was broken and old stuff is different from new stuff
		Reference         string `json:"reference"`
		WithdrawalInvoice string `json:"withdrawalInvoice"`
	} `json:"withdrawalInfo"`
	Amount int `json:"amount"`
}

type GiftSpentEvent struct {
	OrderId string `json:"id"`
	Amount  int    `json:"amount"`
	Spent   bool   `json:"spent"`
}

type GiftsData struct {
	Gifts []string `json:"gifts"`
}

func (g GiftsGift) WithdrawDate() string {
	return time.Unix(g.WithdrawalInfo.CreatedAt.Seconds, 0).Format("2 Jan 2006 at 3:04PM")
}

func (g GiftsGift) RedeemerURL() string {
	invoice := g.WithdrawalInfo.WithdrawalInvoice
	if invoice == "" {
		invoice = g.WithdrawalInfo.Reference
	}
	inv, _ := ln.Call("decodepay", invoice)
	return nodeLink(inv.Get("payee").String())
}

func createGift(user User, sats int, messageId int) error {
	var order GiftsOrder
	var gerr GiftsError
	resp, err := napping.Post("https://api.lightning.gifts/create", struct {
		Amount int    `json:"amount"`
		Notify string `json:"notify"`
	}{sats, fmt.Sprintf("%s/app/gifts/webhook?user=%d", s.ServiceURL, user.Id)}, &order, &gerr)
	if err != nil {
		return err
	}
	if resp.Status() >= 300 {
		if gerr.Message == "GIFT_AMOUNT_UNDER_100" {
			return errors.New("Gift should be at least 100 sat!")
		} else {
			return errors.New("lightning.gifts error: " + gerr.Message)
		}
	}

	inv, err := decodeInvoice(order.LightningInvoice.PayReq)
	if err != nil {
		return errors.New("Failed to decode invoice.")
	}
	return user.actuallySendExternalPayment(
		messageId, order.LightningInvoice.PayReq, inv, inv.MSatoshi,
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
			paymentHasSucceeded(u, messageId, msatoshi, msatoshi_sent, preimage, "gifts", hash)

			// wait for gift to be available
			for i := 0; i < 10; i++ {
				var status GiftsOrder
				napping.Get("https://api.lightning.gifts/status/"+order.ChargeId, nil, &status, nil)
				if status.Status == "paid" {
					break
				}
				time.Sleep(time.Second * time.Duration(i))
			}

			// we already have the order id which is the gift url
			u.notifyAsReply(t.GIFTSCREATED, t.T{"OrderId": order.OrderId}, messageId)

			// save gift info as user data
			var data GiftsData
			err := user.getAppData("gifts", &data)
			if err != nil {
				u.notify(t.GIFTSFAILEDSAVE, t.T{"Err": err.Error()})
				return
			}
			data.Gifts = append(data.Gifts, order.OrderId)

			// limit stored gifts to 50
			if len(data.Gifts) > 50 {
				data.Gifts = data.Gifts[len(data.Gifts)-50:]
			}

			err = user.setAppData("gifts", data)
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

func serveGiftsWebhook() {
	router.Path("/app/gifts/webhook").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// parse the incoming data
		var event GiftSpentEvent
		err := json.NewDecoder(r.Body).Decode(&event)
		if err != nil {
			log.Warn().Err(err).Msg("error decoding gifts webhook")
			return
		}

		// fetch gift
		gift, err := getGift(event.OrderId)
		if err != nil {
			log.Warn().Err(err).Interface("ev", event).Msg("error fetching gift on webhook")
			return
		}

		// fetch user
		userId, err := strconv.Atoi(r.URL.Query().Get("user"))
		if err != nil {
			log.Warn().Err(err).Interface("ev", event).Msg("invalid user on gifts webhook")
		}
		user, err := loadUser(userId)
		if err != nil {
			log.Warn().Err(err).Interface("ev", event).Msg("error loading gifts giver after webhook")
		}

		user.notify(t.GIFTSSPENTEVENT, t.T{
			"Id":          gift.OrderId,
			"Description": gift.WithdrawalInfo.Reference,
			"Amount":      gift.Amount,
		})
	})
}

func loadUserFromGiftId(giftId string) (u User, err error) {
	err = pg.Get(&u, `
SELECT `+USERFIELDS+` 
FROM telegram.account
WHERE appdata->'gifts' ?| array[$1]
    `, giftId)
	return
}
