package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/fiatjaf/lightningd-gjson-rpc/plugin"
	decodepay "github.com/fiatjaf/ln-decodepay"
)

var continueHTLC = map[string]interface{}{"result": "continue"}
var failHTLC = map[string]interface{}{"result": "fail", "failure_code": 16392}
var failUnknown = map[string]interface{}{"result": "fail", "failure_code": 16399}

func htlc_accepted(p *plugin.Plugin, params plugin.Params) (resp interface{}) {
	ctx := context.WithValue(context.Background(), "origin", "background")

	amount := params.Get("htlc.amount").String()
	scid := params.Get("onion.short_channel_id").String()
	if scid == "0x0x0" {
		// payment coming to this node, accept it
		return continueHTLC
	}

	hash := params.Get("htlc.payment_hash").String()
	p.Logf("got HTLC. amount=%s short_channel_id=%s hash=%s", amount, scid, hash)

	for rds == nil || pg == nil {
		p.Log("htlc_accepted: waiting until redis and postgres are available.")
		time.Sleep(1 * time.Second)
	}

	msatoshi, err := strconv.ParseInt(amount[:len(amount)-4], 10, 64)
	if err != nil {
		// I don't know what is happening
		p.Logf("error parsing onion.forward_amount: %s - continue", err.Error())
		return continueHTLC
	}

	bscid, err := decodeShortChannelId(scid)
	if err != nil {
		p.Logf("onion.short_channel_id is not in the usual format - continue")
		return continueHTLC
	}

	shadowData, ok := extractDataFromShadowChannelId(bscid)
	if !ok {
		// it's not an invoice for an etleneum call or contract
		p.Logf("failed to get data from onion.short_channel_id - continue")
		return continueHTLC
	}

	// don't accept payments smaller than the requested amount
	if shadowData.Msatoshi > msatoshi || msatoshi > shadowData.Msatoshi*2 {
		return failHTLC
	}

	// ensure our preimage is correct
	preimage, _ := hex.DecodeString(shadowData.Preimage)
	derivedHash := sha256.Sum256(preimage)
	derivedHashHex := hex.EncodeToString(derivedHash[:])
	if derivedHashHex != hash {
		p.Logf("we have a preimage %s, but its hash %s didn't match the expected hash %s - continue", shadowData.Preimage, derivedHashHex, hash)
		return failUnknown
	}

	// here we know it's a payment for an lntxbot user
	go deleteDataAssociatedWithShadowChannelId(bscid)
	go onInvoicePaid(ctx, hash, shadowData)

	invoice := Invoice{
		Bolt11: decodepay.Bolt11{
			MSatoshi:        shadowData.Msatoshi,
			Description:     shadowData.Description,
			DescriptionHash: shadowData.DescriptionHash,
			PaymentHash:     hash,
		},
		Preimage: shadowData.Preimage,
	}
	go resolveWaitingInvoice(hash, invoice)

	p.Logf("invoice received. we have a preimage: %s - resolve", shadowData.Preimage)
	return map[string]interface{}{
		"result":      "resolve",
		"payment_key": shadowData.Preimage,
	}
}
