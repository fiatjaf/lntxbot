package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/fiatjaf/lntxbot/t"
)

type SatelliteOrderRequest struct {
	AuthToken        string `json:"auth_token"`
	UUID             string `json:"uuid"`
	LightningInvoice struct {
		PayReq   string `json:"payreq"`
		Metadata struct {
			Sha256 string `json:"sha256_message_digest"`
		} `json:"metadata"`
	} `json:"lightning_invoice"`
}

type SatelliteOrder struct {
	Bid                   int64   `json:"bid"`
	MessageSize           int     `json:"message_size"`
	BidPerByte            float64 `json:"bid_per_byte"`
	Digest                string  `json:"message_digest"`
	Status                string  `json:"status"`
	UUID                  string  `json:"uuid"`
	CreatedAt             string  `json:"created_at"`
	StartedTransmissionAt string  `json:"started_transmission_at"`
	EndedTransmissionAt   string  `json:"ended_transmission_at"`
	TxSeqNum              int64   `json:"tx_seq_num"`
	UnpaidBid             int64   `json:"unpaid_bid"`
}

func (order SatelliteOrder) Time() string {
	parsedtime, _ := time.Parse("2006-01-02T15:04:05Z", order.CreatedAt)
	return parsedtime.Format("2 Jan 15:04")
}

type SatelliteError struct {
	Message string `json:"message"`
	Errors  []struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
		Code   int    `json:"code"`
	} `json:"errors"`
}

type SatelliteData struct {
	Orders [][]string `json:"orders"`
}

func createSatelliteOrder(ctx context.Context, satoshis int, text string) (err error) {
	resp, err := http.PostForm("https://api.blockstream.space/order", url.Values{
		"bid":     {strconv.Itoa(satoshis * 1000)},
		"message": {text},
	})
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var saterr SatelliteError
		err = json.NewDecoder(resp.Body).Decode(&saterr)
		if err != nil {
			return
		}

		log.Warn().Interface("saterr", saterr).
			Int("satoshis", satoshis).Int("size", len(text)).
			Msg("satellite returned error")
		err = errors.New(saterr.Message)
		return
	}

	var orderreq SatelliteOrderRequest
	err = json.NewDecoder(resp.Body).Decode(&orderreq)
	if err != nil {
		return
	}

	return paySatelliteOrder(ctx, orderreq)
}

func paySatelliteOrder(ctx context.Context, orderreq SatelliteOrderRequest) error {
	inv, err := decodeInvoice(orderreq.LightningInvoice.PayReq)
	if err != nil {
		return errors.New("Failed to decode invoice.")
	}

	user := ctx.Value("initiator").(User)
	return user.actuallySendExternalPayment(
		ctx, orderreq.LightningInvoice.PayReq, inv, inv.MSatoshi,
		func(
			ctx context.Context,
			u User,
			msatoshi float64,
			msatoshi_sent float64,
			preimage string,
			tag string,
			hash string,
		) {
			// on success
			paymentHasSucceeded(ctx, u, msatoshi, msatoshi_sent, preimage, "satellite", hash)

			// done
			send(ctx, u, t.SATELLITEPAID, t.T{"UUID": orderreq.UUID})
		},
		paymentHasFailed,
	)
}

func getSatelliteOrder(ctx context.Context, uuid string) (order SatelliteOrder, err error) {
	token, ok := getSatelliteOrderToken(ctx, uuid)
	if !ok {
		err = errors.New("Couldn't find order " + uuid + ".")
		return
	}

	return fetchSatelliteOrder(uuid, token)
}

func fetchSatelliteOrder(uuid, token string) (order SatelliteOrder, err error) {
	req, _ := http.NewRequest("GET", "https://api.blockstream.space/order/"+uuid, nil)
	req.Header.Add("X-Auth-Token", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var saterr SatelliteError
		err = json.NewDecoder(resp.Body).Decode(&saterr)
		if err != nil {
			return
		}

		log.Warn().Interface("saterr", saterr).Str("uuid", uuid).Str("token", token).
			Msg("satellite returned error")
		err = errors.New(saterr.Message)
		return
	}

	json.NewDecoder(resp.Body).Decode(&order)
	return
}

func getSatelliteOrderToken(ctx context.Context, uuid string) (token string, ok bool) {
	user := ctx.Value("initiator").(User)

	var satdata SatelliteData
	err = user.getAppData("satellite", &satdata)
	if err != nil {
		log.Error().Err(err).Str("user", user.Username).Str("uuid", uuid).
			Msg("failed to load satellite data when searching for token")
		return
	}

	for _, tuple := range satdata.Orders {
		if tuple[0] == uuid {
			return tuple[1], true
		}
	}

	return
}
