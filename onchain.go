package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/die-net/lrucache"
	decodepay "github.com/fiatjaf/ln-decodepay"
	"github.com/fiatjaf/lntxbot/t"
	"github.com/gregjones/httpcache"
)

// powered by deezy.io

var depositDeezyHttpClient = &http.Client{
	Transport: httpcache.NewTransport(
		lrucache.New(5, 60*5),
	),
}

func handleDepositOnchain(ctx context.Context) {
	u := ctx.Value("initiator").(*User)
	code := createLNURLPayCode(u, "deezy")

	params, _ := json.Marshal(struct {
		Code string `json:"lnurl_or_lnaddress"`
	}{code})

	resp, err := depositDeezyHttpClient.Post("https://api.deezy.io/v1/source", "application/json", bytes.NewBuffer(params))
	if err != nil {
		send(ctx, u, t.ERROR, t.T{
			"Err": fmt.Sprintf("failed to call deezy.io API: %s", err.Error()),
		})
		return
	}
	if resp.StatusCode != 200 {
		b, _ := ioutil.ReadAll(resp.Body)
		text := string(b)
		send(ctx, u, t.ERROR, t.T{
			"Err": fmt.Sprintf("deezy.io API returned an error (%d): %s", resp.StatusCode, text),
		})
		return
	}

	var val struct {
		Address    string `json:"address"`
		Commitment string `json:"commitment"`
		Signature  string `json:"signature"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&val); err != nil {
		send(ctx, u, t.ERROR, t.T{
			"Err": fmt.Sprintf("deezy.io API returned a broken response"),
		})
		return
	}

	send(ctx, u, t.ONCHAINDEPOSIT, t.T{
		"ServiceId":  s.ServiceId,
		"Address":    val.Address,
		"Commitment": escapeHTML(val.Commitment),
		"Signature":  val.Signature,
	})
}

func handleSendToAddress(ctx context.Context, address string, msats int64) {
	u := ctx.Value("initiator").(*User)

	params, _ := json.Marshal(struct {
		AmountSats          int64  `json:"amount_sats"`
		OnChainAddress      string `json:"on_chain_address"`
		OnChainSatsPerVByte int    `json:"on_chain_sats_per_vbyte"`
	}{msats / 1000, address, 2})

	resp, err := http.Post("https://api.deezy.io/v1/swap", "application/json", bytes.NewBuffer(params))
	if err != nil {
		send(ctx, u, t.ERROR, t.T{
			"Err": fmt.Sprintf("failed to call deezy.io API: %s", err.Error()),
		})
		return
	}
	if resp.StatusCode != 200 {
		b, _ := ioutil.ReadAll(resp.Body)
		text := string(b)
		send(ctx, u, t.ERROR, t.T{
			"Err": fmt.Sprintf("deezy.io API returned an error (%d): %s", resp.StatusCode, text),
		})
		return
	}

	var val struct {
		Bolt11Invoice string `json:"bolt11_invoice"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&val); err != nil || val.Bolt11Invoice == "" {
		send(ctx, u, t.ERROR, t.T{
			"Err": fmt.Sprintf("deezy.io API returned a broken response"),
		})
		return
	}

	processingMessageId := send(ctx, u, val.Bolt11Invoice+"\n\n"+translate(ctx, t.PROCESSING))

	if inv, err := decodepay.Decodepay(val.Bolt11Invoice); err != nil {
		send(ctx, u, t.ERROR,
			t.T{"Err": fmt.Errorf("error parsing invoice: %w", err)},
			processingMessageId)
		return
	} else if inv.MSatoshi > (msats + 1000000) {
		send(ctx, u, t.ERROR,
			t.T{"Err": "The invoice we got from deezy.io is too expensive, so we're stopping here just in case, let us know if this is wrong. You can also pay the invoice manually."},
			processingMessageId)
		return
	}

	if hash, err := u.payInvoice(ctx, val.Bolt11Invoice, 0); err != nil {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()}, processingMessageId)
	} else {
		// wait until invoice is paid
		go func() {
			<-waitPaymentSuccess(hash)
			time.Sleep(5 * time.Second)
			if resp, err := http.Get("https://api.deezy.io/v1/swap/lookup?bolt11_invoice=" + val.Bolt11Invoice); err == nil {
				var val struct {
					Txid string `json:"on_chain_txid"`
					Hex  string `json:"tx_hex"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&val); err != nil {
					send(ctx, u, t.ERROR, t.T{
						"Err": fmt.Sprintf("deezy.io API returned a broken response from status check"),
					})
					return
				}

				send(ctx, u, processingMessageId, t.ONCHAINSTATUS, t.T{"Txid": val.Txid, "Hex": val.Hex})
			}
		}()
	}
}
