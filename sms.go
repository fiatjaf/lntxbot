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

	country := "Russia"
	if str, ok := opts["<country>"].(string); ok {
		country = str
	}

	service := "other"
	if str, ok := opts["<service>"].(string); ok {
		service = str
	}

	params, _ := json.Marshal(struct {
		Country string `json:"country"`
		Service string `json:"service"`
	}{country, service})

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
			"Err": fmt.Sprintf("sms4sats.com API returned a broken response while creating order"),
		})
		return
	}

	processingMessageId := send(ctx, u, val.Payreq+"\n\n"+translate(ctx, t.PROCESSING))

	if _, err := u.payInvoice(ctx, val.Payreq, 0); err != nil {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()}, processingMessageId)
	} else {
		go func() {
			// sms4sats uses hold invoices
			// <-waitPaymentSuccess(hash)
			time.Sleep(12 * time.Second)
			if resp, err := http.Get("https://api2.sms4sats.com/orderstatus?orderId=" + val.OrderId); err == nil {
				var val2 struct {
					Number int    `json:"number"`
					Status string `json:"status"`
					Paid   bool   `json:"paid"`
					// Error  string `json:"error"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&val2); err != nil {
					send(ctx, u, t.ERROR, t.T{
						"Err": fmt.Sprintf("sms4sats.com API returned a broken response from status check"),
					})
					return
				}

				if val2.Number == 0 {
					send(ctx, u, t.ERROR, t.T{"Err": "Failed to get a number, try another country/service e.g. /sms russia twitter"}, processingMessageId)
					return
				}

				numberMessageId := send(ctx, u, t.SMSRECEIVE, t.T{
					"number":  val2.Number,
					"country": country,
					"service": service,
					"orderId": val.OrderId,
				})

				// continue checking orderstatus to see if code is received, then send to user
				// check orderstatus every 5 seconds for 20 minutes and then give up
				count := 0
				for count < 1200 {
					if resp, err := http.Get("https://api2.sms4sats.com/orderstatus?orderId=" + val.OrderId); err == nil {
						var val3 struct {
							Code string `json:"code"`
						}
						if err := json.NewDecoder(resp.Body).Decode(&val3); err != nil {
							return
						}

						if val3.Code != "" {
							count = 1200
							send(ctx, u, numberMessageId, t.SMSSTATUS, t.T{"code": val3.Code})
							break
						}
					}

					time.Sleep(5 * time.Second)
					count += 5
				}
			}
		}()
	}
}
