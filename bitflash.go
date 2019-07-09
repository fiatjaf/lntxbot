package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type BitflashResponse struct {
	OrderId           string `json:"order_id"`
	ChargeId          string `json:"charge_id"`
	Bolt11            string `json:"bolt11"`
	Fee               int    `json:"fee_satoshi"`
	NetworkFee        int    `json:"network_fee_sat"`
	TransactionAmount string `json:"tobe_paid_satoshi"`
	Receiver          string `json:"receiver"`
	ReceiverAmount    string `json:"receiver_amount"`
}

type BitflashOrder struct {
	Id          string `json:"id"`
	PayReq      string `json:"payreq"`
	Description string `json:"description"`
	CreatedAt   int64  `json:"created_at"`
	PaidAt      int64  `json:"paid_at"`
}

func prepareBitflashTransaction(user User, messageId int, satoshi int, address string) (bfresp BitflashResponse, err error) {
	if satoshi < 1000 {
		err = errors.New("Amount too small.")
		return
	}

	btc := fmt.Sprintf("%.8f", float64(satoshi)/100000000)
	resp, err := http.PostForm("https://api.bitflash.club/new", url.Values{"amount": {btc}, "address": {address}})
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		err = errors.New("Bitflash call failed.")
		return
	}

	err = json.NewDecoder(resp.Body).Decode(&bfresp)
	if err != nil {
		return
	}

	return
}

func getBitflashOrder(chargeId string) (order BitflashOrder, err error) {
	log.Print(chargeId)
	resp, err := http.PostForm("https://api.bitflash.club/invoiceinfo", url.Values{"id": {chargeId}})
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		err = errors.New("Bitflash call failed.")
		return
	}

	err = json.NewDecoder(resp.Body).Decode(&order)
	if err != nil {
		body, errx := ioutil.ReadAll(resp.Body)
		log.Print(string(body), errx)
		return
	}

	return
}

func payBitflashInvoice(user User, order BitflashOrder, messageId int) (err error) {
	inv, err := ln.Call("decodepay", order.PayReq)
	if err != nil {
		err = errors.New("Failed to decode invoice.")
		return
	}
	err = user.actuallySendExternalPayment(
		messageId, order.PayReq, inv, inv.Get("msatoshi").Int(),
		fmt.Sprintf("%s.bitflash.%s.%d", s.ServiceId, order.Id, user.Id), map[string]interface{}{},
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

			sendMessageAsReply(u.ChatId, "Transaction queued!", messageId)
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
	if err != nil {
		return
	}

	return
}

func saveBitflashOrder(user User, orderId string) {
	var data struct {
		Orders []string `json:"orders"`
	}
	err = user.getAppData("bitflash", &data)
	if err == nil {
		data.Orders = append(data.Orders, orderId)
		err = user.setAppData("bitflash", data)
		if err != nil {
			sendMessage(user.ChatId, "Failed to save Bitflash order. Please report: "+err.Error())
		}
	}
}
