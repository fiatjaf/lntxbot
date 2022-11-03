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
	"github.com/docopt/docopt-go"
	decodepay "github.com/fiatjaf/ln-decodepay"
	"github.com/fiatjaf/lntxbot/t"
	"github.com/gregjones/httpcache"
)

// powered by sms4sats.com

var sms4satsHttpClient = &http.Client{
	Transport: httpcache.NewTransport(
		lrucache.New(5, 60*5),
	),
}

func handleReceiveSMS(ctx context.Context, opts docopt.Opts) {
	u := ctx.Value("initiator").(*User)

	// for {
	// 	time.Sleep(5 * time.Second)
	// 	fmt.Println("orderstatus check loop: ")

	// 	// select {
	// 	// case err := <-clichePing():
	// 	// 	if err != nil {
	// 	// 		log.Error().Err(err).Msg("cliche ping returned error")
	// 	// 		break
	// 	// 	} else {
	// 	// 		log.Debug().Msg("cliche is fine")
	// 	// 		continue
	// 	// 	}
	// 	// case <-time.After(3 * time.Minute):
	// 	// 	log.Error().Msg("cliche is not responding after 3 minutes")
	// 	// 	break
	// 	// }
	// }
	// return

	country := "Russia"
	if opts["country"].(bool) {
		country = opts["country"].(string)
	}

	service := "other"
	if opts["service"].(bool) {
		service = opts["other"].(string)
	}

	fmt.Println("1sms params: ", country, service)

	// var order struct {
	// 	country string `json:"country"`
	// 	service string `json:"service"`
	// }

	// map[string]interface{}{"sats": sats}
	// params, _ := json.Marshal(Order {country, service})

	params, _ := json.Marshal(struct {
		Country string `json:"country"`
		Service string `json:"service"`
	}{country, service})

	fmt.Println("sms params order, params ", params)

	resp, err := sms4satsHttpClient.Post("https://api2.sms4sats.com/createorder", "application/json", bytes.NewBuffer(params))
	if err != nil {
		send(ctx, u, t.ERROR, t.T{
			"Err": fmt.Sprintf("failed to call sms4sats.com API: %s", err.Error()),
		})
		return
	}

	if resp.StatusCode != 200 {
		b, _ := ioutil.ReadAll(resp.Body)
		text := string(b)
		send(ctx, u, t.ERROR, t.T{
			"Err": fmt.Sprintf("sms4sats.com API returned an error (%d): %s", resp.StatusCode, text),
		})
		return
	}

	var val struct {
		Status  string `json:"status"`
		OrderId string `json:"orderId"`
		Payreq  string `json:"payreq"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&val); err != nil {
		send(ctx, u, t.ERROR, t.T{
			"Err": fmt.Sprintf("sms4sats.com API returned a broken response"),
		})
		return
	}

	fmt.Println("sms resp.Body val: ", val.Status, val.OrderId, val.Payreq)

	processingMessageId := send(ctx, u, val.Payreq+"\n\n"+translate(ctx, t.PROCESSING))

	if hash, err := u.payInvoice(ctx, val.Payreq, 0); err != nil {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()}, processingMessageId)
	} else {
		// wait until invoice is paid
		go func() {
			<-waitPaymentSuccess(hash)
			time.Sleep(5 * time.Second)
			if resp, err := http.Get("https://api2.sms4sats.com/orderstatus?orderId=" + val.OrderId); err == nil {
				var val2 struct {
					Number int `json:"number"`
					// Hex  	string `json:"tx_hex"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&val2); err != nil {
					send(ctx, u, t.ERROR, t.T{
						"Err": fmt.Sprintf("sms4sats.com API returned a broken response from status check"),
					})
					return
				}

				// numberMessageId :=
				send(ctx, u, t.SMSRECEIVE, t.T{
					"number":  val2.Number,
					"country": country,
					"service": service,
					"orderId": val.OrderId,
				})

				// // TODO: continue checking orderstatus to see if code is received, then send to user
				// // send(ctx, u, processingMessageId, t.SMSSTATUS, t.T{"code": val.code})
				// // check orderstatus every 5 seconds for 20 minutes and then give up
				// count := 0
				// for count < 1200 {
				// 	fmt.Println("orderstatus check loop: ", count)

				// 	if resp, err := http.Get("https://api2.sms4sats.com/orderstatus?orderId=" + val.OrderId); err == nil {
				// 		var val2 struct {
				// 			Code int `json:"code"`
				// 			// Hex  	string `json:"tx_hex"`
				// 		}
				// 		if err := json.NewDecoder(resp.Body).Decode(&val2); err != nil {
				// 			send(ctx, u, t.ERROR, t.T{
				// 				"Err": fmt.Sprintf("sms4sats.com API returned a broken response from status check"),
				// 			})
				// 			return
				// 		}

				// 		count = 1200
				// 		send(ctx, u, numberMessageId, t.SMSSTATUS, t.T{"code": val.Code})
				// 		break

				// 	time.Sleep(5 * time.Second)
				// 	count += 5
				// }
			}
		}()
	}
}

func handleSendSMS(ctx context.Context, address string, msats int64) {
	u := ctx.Value("initiator").(*User)

	params, _ := json.Marshal(struct {
		AmountSats          int64  `json:"amount_sats"`
		OnChainAddress      string `json:"on_chain_address"`
		OnChainSatsPerVByte int    `json:"on_chain_sats_per_vbyte"`
	}{msats / 1000, address, 2})

	resp, err := http.Post("https://api2.sms4sats.com/v1/swap", "application/json", bytes.NewBuffer(params))
	if err != nil {
		send(ctx, u, t.ERROR, t.T{
			"Err": fmt.Sprintf("failed to call sms4sats.com API: %s", err.Error()),
		})
		return
	}
	if resp.StatusCode != 200 {
		b, _ := ioutil.ReadAll(resp.Body)
		text := string(b)
		send(ctx, u, t.ERROR, t.T{
			"Err": fmt.Sprintf("sms4sats.com API returned an error (%d): %s", resp.StatusCode, text),
		})
		return
	}

	var val struct {
		Bolt11Invoice string `json:"bolt11_invoice"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&val); err != nil || val.Bolt11Invoice == "" {
		send(ctx, u, t.ERROR, t.T{
			"Err": fmt.Sprintf("sms4sats.com API returned a broken response"),
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
			t.T{"Err": "The invoice we got from sms4sats.com is too expensive, so we're stopping here just in case, let us know if this is wrong. You can also pay the invoice manually."},
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
			if resp, err := http.Get("https://api.sms4sats.com/v1/swap/lookup?bolt11_invoice=" + val.Bolt11Invoice); err == nil {
				var val struct {
					Txid string `json:"on_chain_txid"`
					Hex  string `json:"tx_hex"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&val); err != nil {
					send(ctx, u, t.ERROR, t.T{
						"Err": fmt.Sprintf("sms4sats.com API returned a broken response from status check"),
					})
					return
				}

				send(ctx, u, processingMessageId, t.ONCHAINSTATUS, t.T{"Txid": val.Txid, "Hex": val.Hex})
			}
		}()
	}
}
