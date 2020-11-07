package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/aead/chacha20"
	"github.com/btcsuite/btcd/btcec"
	"github.com/fiatjaf/lightningd-gjson-rpc/plugin"
	decodepay "github.com/fiatjaf/ln-decodepay"
	sphinx "github.com/lightningnetwork/lightning-onion"
	"github.com/lightningnetwork/lnd/lnwire"
)

var continueHTLC = map[string]interface{}{"result": "continue"}
var failHTLC = map[string]interface{}{"result": "fail", "failure_code": 16392}

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

	// here we know it's a payment for an lntxbot user
	receiver, err := loadUser(shadowData.UserId)
	if err != nil {
		log.Warn().Err(err).Interface("shadow-data", shadowData).
			Msg("failed to load user on htlc_accepted")
		return continueHTLC
	}

	// ensure our preimage is correct
	preimage, _ := hex.DecodeString(shadowData.Preimage)
	derivedHash := sha256.Sum256(preimage)
	derivedHashHex := hex.EncodeToString(derivedHash[:])
	if derivedHashHex != hash {
		p.Logf("we have a preimage %s, but its hash %s didn't match the expected hash %s - return incorrect_or_unknown_payment_details",
			shadowData.Preimage, derivedHashHex, hash)

		// get keys stuff so we can return a wrapped onion to pre-pay probes
		nextOnion, err := hex.DecodeString(params.Get("onion.next_onion").String())
		if err != nil {
			p.Logf("lightningd has sent us an invalid onion.next_onion: %s",
				err.Error())
			return failHTLC
		}

		var nextOnionPacket sphinx.OnionPacket
		err = nextOnionPacket.Decode(bytes.NewBuffer(nextOnion))
		if err != nil {
			p.Logf("couldn't parse onion.next_onion: %s", err.Error())
			return failHTLC
		}

		lastHopKey := receiver.invoicePrivateKey()

		// bolt04 shared key stuff: ecdh() then sha256()
		s := &btcec.PublicKey{}
		s.X, s.Y = btcec.S256().ScalarMult(
			nextOnionPacket.EphemeralKey.X,
			nextOnionPacket.EphemeralKey.Y,
			lastHopKey.D.Bytes(),
		)
		lastHopSharedSecret := sha256.Sum256(s.SerializeCompressed())

		// produce the error as if we were the last hop
		failure := lnwire.NewFailIncorrectDetails(lnwire.MilliSatoshi(msatoshi), 0)
		var payload bytes.Buffer
		if err := lnwire.EncodeFailure(&payload, failure, 0); err != nil {
			panic(err)
		}
		data := payload.Bytes()

		// hmac the payload
		umKey := generateKey("um", lastHopSharedSecret[:])
		mac := hmac.New(sha256.New, umKey[:])
		mac.Write(data)
		h := mac.Sum(nil)
		failureOnion := append(h, data...)

		// obfuscate/wrap the message as if we were the last hop
		ammagKey := generateKey("ammag", lastHopSharedSecret[:])
		placeholder := make([]byte, len(failureOnion))
		xor(
			placeholder,
			failureOnion,
			generateCipherStream(ammagKey, uint(len(failureOnion))),
		)
		failureOnion = placeholder

		// return the onion as failure_onion and lightningd will wrap it
		return map[string]interface{}{
			"result":        "fail",
			"failure_onion": hex.EncodeToString(failureOnion),
		}
	}

	// here we know this payment has succeeded
	go deleteDataAssociatedWithShadowChannelId(bscid)
	go receiver.onReceivedInvoicePayment(ctx, hash, shadowData)

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

func generateCipherStream(key [32]byte, numBytes uint) []byte {
	var (
		nonce [8]byte
	)
	cipher, err := chacha20.NewCipher(nonce[:], key[:])
	if err != nil {
		panic(err)
	}
	output := make([]byte, numBytes)
	cipher.XORKeyStream(output, output)

	return output
}

func xor(dst, a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		dst[i] = a[i] ^ b[i]
	}
	return n
}

func generateKey(keyType string, sharedKey []byte) [32]byte {
	mac := hmac.New(sha256.New, []byte(keyType))
	mac.Write(sharedKey)
	h := mac.Sum(nil)

	var key [32]byte
	copy(key[:], h[:32])

	return key
}
