package main

import (
	"strconv"
	"time"

	"github.com/fiatjaf/lightningd-gjson-rpc/plugin"
	decodepay "github.com/fiatjaf/ln-decodepay"
	"github.com/fiatjaf/lntxbot/t"
)

var continueHTLC = map[string]interface{}{"result": "continue"}
var failHTLC = map[string]interface{}{"result": "fail", "failure_code": 16392}

func htlc_accepted(p *plugin.Plugin, params plugin.Params) (resp interface{}) {
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

	// here we know it's a payment for an lntxbot user
	go handleInvoicePaid(hash, shadowData)
	go resolveWaitingInvoice(hash, Invoice{
		Bolt11: decodepay.Bolt11{
			MSatoshi:        shadowData.Msatoshi,
			Description:     shadowData.Description,
			DescriptionHash: shadowData.DescriptionHash,
			PaymentHash:     hash,
		},
		Preimage: shadowData.Preimage,
	})

	p.Logf("invoice received. we have a preimage: %s - resolve", shadowData.Preimage)
	return map[string]interface{}{
		"result":      "resolve",
		"payment_key": shadowData.Preimage,
	}
}

func handleInvoicePaid(hash string, data ShadowChannelData) {
	receiver, err := loadUser(data.UserId, 0)
	if err != nil {
		log.Warn().Err(err).
			Interface("shadow-data", data).
			Msg("failed to load on handleInvoicePaid")
		return
	}

	receiver.track("got payment", map[string]interface{}{
		"sats": float64(data.Msatoshi) / 1000,
	})

	// is there a comment associated with this?
	go func() {
		time.Sleep(3 * time.Second)
		if comment, ok := data.Extra["comment"]; ok && comment != "" {
			receiver.notify(t.LNURLPAYCOMMENT, t.T{
				"Text":           comment,
				"HashFirstChars": hash[:5],
			})
		}
	}()

	// proceed to compute an incoming payment for this user
	err = receiver.paymentReceived(
		int(data.Msatoshi),
		data.Description,
		hash,
		data.Preimage,
		data.Tag,
	)
	if err != nil {
		receiver.notifyAsReply(t.FAILEDTOSAVERECEIVED, t.T{"Hash": hash}, data.MessageId)
		return
	}

	receiver.notifyAsReply(t.PAYMENTRECEIVED, t.T{
		"Sats": data.Msatoshi / 1000,
		"Hash": hash[:5],
	}, data.MessageId)
}
