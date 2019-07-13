package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"gopkg.in/jmcvetta/napping.v3"
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

func (order SatelliteOrder) String() string {
	parsedtime, _ := time.Parse("2006-01-02T15:04:05Z", order.CreatedAt)
	return "📡" +
		fmt.Sprintf(" <code>%s</code> <i>%s</i> <code>%db</code> <code>%.23fsat/b</code> <i>%s</i>",
			order.UUID, order.Status, order.MessageSize, order.BidPerByte/1000,
			parsedtime.Format("2 Jan 15:04"))
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

func createSatelliteOrder(user User, messageId int, satoshis int, message string) (err error) {
	resp, err := http.PostForm("https://api.blockstream.space/order", url.Values{
		"bid":     {strconv.Itoa(satoshis * 1000)},
		"message": {message},
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

		log.Warn().Interface("saterr", saterr).Str("user", user.Username).
			Int("satoshis", satoshis).Int("messagesize", len(message)).
			Msg("satellite returned error")
		err = errors.New(saterr.Message)
		return
	}

	var orderreq SatelliteOrderRequest
	err = json.NewDecoder(resp.Body).Decode(&orderreq)
	if err != nil {
		return
	}

	return paySatelliteOrder(user, messageId, orderreq)
}

func bumpSatelliteOrder(user User, messageId int, uuid string, satoshis int) (err error) {
	token, ok := getSatelliteOrderToken(user, uuid)
	if !ok {
		return errors.New("Couldn't find order " + uuid + ".")
	}

	resp, err := http.PostForm("https://api.blockstream.space/order/"+uuid+"/bump", url.Values{
		"auth_token":   {token},
		"bid_increase": {strconv.Itoa(satoshis * 1000)},
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

		log.Warn().Interface("saterr", saterr).Str("uuid", uuid).Str("token", token).Str("user", user.Username).
			Msg("satellite returned error")
		err = errors.New(saterr.Message)
		return
	}

	var orderreq SatelliteOrderRequest
	err = json.NewDecoder(resp.Body).Decode(&orderreq)
	if err != nil {
		return
	}

	return paySatelliteOrder(user, messageId, orderreq)
}

func paySatelliteOrder(user User, messageId int, orderreq SatelliteOrderRequest) error {
	inv, err := ln.Call("decodepay", orderreq.LightningInvoice.PayReq)
	if err != nil {
		return errors.New("Failed to decode invoice.")
	}
	return user.actuallySendExternalPayment(
		messageId, orderreq.LightningInvoice.PayReq, inv, inv.Get("msatoshi").Int(),
		fmt.Sprintf("%s.satellite.%s.%d", s.ServiceId, orderreq.UUID, user.Id), map[string]interface{}{},
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

			// save order uuid and token
			var satdata SatelliteData
			err := u.getAppData("satellite", &satdata)
			if err != nil {
				log.Warn().Err(err).Str("user", u.Username).
					Msg("failed to get satellite data")
				u.notify(t.SATELLITEFAILEDTOSTORE, t.T{"Err": err.Error()})
			} else {
				satdata.Orders = append(satdata.Orders, []string{orderreq.UUID, orderreq.AuthToken})
				err = u.setAppData("satellite", satdata)
				if err != nil {
					log.Warn().Err(err).Str("user", u.Username).Interface("satdata", satdata).
						Msg("failed to set satellite data")
					u.notify(t.SATELLITEFAILEDTOSTORE, t.T{"Err": err.Error()})
				}
			}

			// done
			u.notifyAsReply(t.SATELLITEPAID, t.T{"UUID": orderreq.UUID}, messageId)
		},
		func(
			u User,
			messageId int,
			hash string,
		) {
			// on failure
			paymentHasFailed(u, messageId, hash)

			u.notifyAsReply(t.SATELLITEFAILEDTOPAY, nil, messageId)
		},
	)
}

func getSatelliteOrder(user User, uuid string) (order SatelliteOrder, err error) {
	token, ok := getSatelliteOrderToken(user, uuid)
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

func deleteSatelliteOrder(user User, uuid string) (err error) {
	token, ok := getSatelliteOrderToken(user, uuid)
	if !ok {
		err = errors.New("Couldn't find order " + uuid + ".")
		return
	}

	body := url.Values{"auth_token": {token}}
	req, _ := http.NewRequest("DELETE", "https://api.blockstream.space/order/"+uuid, strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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

		log.Warn().Interface("saterr", saterr).Str("uuid", uuid).Str("token", token).Str("user", user.Username).
			Msg("satellite returned error")
		err = errors.New(saterr.Message)
		return
	}

	// delete order uuid and token
	var satdata SatelliteData
	err = user.getAppData("satellite", &satdata)
	if err != nil {
		log.Warn().Err(err).Str("user", user.Username).
			Msg("failed to get satellite data")
		user.notify(t.SATELLITEFAILEDTODELETE, t.T{"Err": err.Error()})
	} else {
		// remove the order we've just deleted -- by cloning the entire array except it.
		neworders := make([][]string, 0, len(satdata.Orders))
		for _, tuple := range satdata.Orders {
			if tuple[0] != uuid {
				neworders = append(neworders, tuple)
			}
		}

		err = user.setAppData("satellite", satdata)
		if err != nil {
			log.Warn().Err(err).Str("user", user.Username).Interface("satdata", satdata).
				Msg("failed to set satellite data")
			user.notify(t.SATELLITEFAILEDTODELETE, t.T{"Err": err.Error()})
		}
	}

	return nil
}

func getSatelliteQueue() (queue []SatelliteOrder, err error) {
	var saterr SatelliteError
	resp, err := napping.Get("https://api.blockstream.space/orders/queued",
		&url.Values{"limit": {"25"}}, &queue, &saterr)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		log.Warn().Interface("saterr", saterr).Msg("satellite returned error")
		err = errors.New(saterr.Message)
		return
	}

	return
}

func getSatelliteOrderToken(user User, uuid string) (token string, ok bool) {
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
