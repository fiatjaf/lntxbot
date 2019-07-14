package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kr/pretty"
	"github.com/lucsky/cuid"
	"gopkg.in/jmcvetta/napping.v3"
)

type PokerFirestoreDocument struct {
	Name   string                         `json:"name"`
	Fields map[string]PokerFirestoreValue `json:"fields"`
}

type PokerFirestoreDocumentList struct {
	Documents []PokerFirestoreDocument `json:"documents"`
}

type PokerFirestoreValue struct {
	String  string `json:"stringValue,omitempty"`
	Integer string `json:"integerValue,omitempty"`
}

func getPokerId(user User) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s.poker.%d", s.BotToken, user.Id)))
	secret := hex.EncodeToString(sum[:])
	return s.ServiceId + ":" + secret[:14]
}

func pokerDeposit(user User, sats int, messageId int) (err error) {
	// create invoice
	var invcreate PokerFirestoreDocument
	resp, err := napping.Post("https://firestore.googleapis.com/v1/projects/ln-pkr/databases/(default)/documents/invoices", PokerFirestoreDocument{
		Fields: map[string]PokerFirestoreValue{
			"amount":    {String: strconv.Itoa(sats)},
			"accountId": {String: getPokerId(user)},
			"state":     {String: "requested"},
		},
	}, &invcreate, nil)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = errors.New("error calling lightning-poker.com firestore backend")
		return
	}

	invid := strings.Split(invcreate.Name, "/")[6]

	// get bolt11 from invoice id (after waiting a while for the invoice to get created)
	time.Sleep(2 * time.Second)
	var invdata PokerFirestoreDocument
	resp, err = napping.Get("https://firestore.googleapis.com/v1/projects/ln-pkr/databases/(default)/documents/invoices/"+invid, nil, &invdata, nil)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = errors.New("error calling lightning-poker.com firestore backend")
		return
	}

	bolt11 := invdata.Fields["payment_request"].String
	pretty.Log(invdata)

	// actually pay
	inv, err := ln.Call("decodepay", bolt11)
	if err != nil {
		return errors.New("Failed to decode invoice.")
	}
	err = user.actuallySendExternalPayment(
		messageId, bolt11, inv, inv.Get("msatoshi").Int(),
		fmt.Sprintf("%s.poker.%s.%d", s.ServiceId, cuid.Slug(), user.Id), map[string]interface{}{},
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

func getPokerBalance(user User) (int, error) {
	var balance PokerFirestoreDocument
	resp, err := napping.Get("https://firestore.googleapis.com/v1/projects/ln-pkr/databases/(default)/documents/accounts/"+getPokerId(user), nil, &balance, nil)
	if err != nil {
		return 0, err
	}
	if resp.Status() >= 300 {
		err = errors.New("error calling lightning-poker.com firestore backend")
		return 0, err
	}

	val, _ := strconv.Atoi(balance.Fields["balance"].Integer)
	return val, nil
}

func withdrawPoker(user User, sats int, messageId int) (err error) {
	bolt11, _, _, err := user.makeInvoice(sats, "withdraw from lightning-poker.com", "", nil, messageId, "", true)
	if err != nil {
		return
	}

	resp, err := napping.Post("https://firestore.googleapis.com/v1/projects/ln-pkr/databases/(default)/documents/payments", PokerFirestoreDocument{
		Fields: map[string]PokerFirestoreValue{
			"payment_request": {String: bolt11},
			"accountId":       {String: getPokerId(user)},
			"state":           {String: "requested"},
		},
	}, nil, nil)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = errors.New("error calling lightning-poker.com firestore backend")
		return
	}

	return
}

func getActivePokerTables() (int, error) {
	var tables PokerFirestoreDocumentList
	resp, err := napping.Get("https://firestore.googleapis.com/v1/projects/ln-pkr/databases/(default)/documents/tables", nil, &tables, nil)
	if err != nil {
		return 0, err
	}
	if resp.Status() >= 300 {
		err = errors.New("error calling lightning-poker.com firestore backend")
		return 0, err
	}

	var activeTables int
	for _, table := range tables.Documents {
		if table.Fields["playing"].Integer != "0" {
			activeTables += 1
		}
	}

	return activeTables, nil
}

func getCurrentPokerStakes() (int, error) {
	var players PokerFirestoreDocumentList
	resp, err := napping.Get("https://firestore.googleapis.com/v1/projects/ln-pkr/databases/(default)/documents/players", nil, &players, nil)
	if err != nil {
		return 0, err
	}
	if resp.Status() >= 300 {
		err = errors.New("error calling lightning-poker.com firestore backend")
		return 0, err
	}

	var totalChips int
	for _, player := range players.Documents {
		chips, _ := strconv.Atoi(player.Fields["chips"].Integer)
		totalChips += chips
	}

	return totalChips, nil
}
