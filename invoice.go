package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/go-lnurl"
	decodepay "github.com/fiatjaf/ln-decodepay"
	"github.com/fiatjaf/lntxbot/t"
	"github.com/imroc/req"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/tidwall/gjson"
)

type Invoice struct {
	decodepay.Bolt11

	Preimage string `json:"preimage"`
}

type InvoiceData struct {
	UserId    int
	Origin    string // "telegram" or "discord"
	MessageId interface{}
	Preimage  string

	makeInvoiceArgs
}

type makeInvoiceArgs struct {
	Description            string
	DescriptionHash        string
	Msatoshi               int64
	Expiry                 *time.Duration
	Tag                    string
	Extra                  map[string]interface{}
	BlueWallet             bool
	IgnoreInvoiceSizeLimit bool
}

func decodeInvoice(bolt11 string) (Invoice, error) {
	inv, err := decodepay.Decodepay(bolt11)
	if err != nil {
		return Invoice{}, err
	}

	return Invoice{
		Bolt11:   inv,
		Preimage: "",
	}, nil
}

var waitingInvoices = cmap.New() // make(map[string][]chan Invoice)

func waitInvoice(hash string) (inv <-chan Invoice) {
	wait := make(chan Invoice)
	waitingInvoices.Upsert(hash, wait,
		func(exists bool, arr interface{}, v interface{}) interface{} {
			if exists {
				return append(arr.([]interface{}), v)
			} else {
				return []interface{}{v}
			}
		},
	)
	return wait
}

func resolveWaitingInvoice(hash string, inv Invoice) {
	if chans, ok := waitingInvoices.Get(hash); ok {
		for _, ch := range chans.([]interface{}) {
			select {
			case ch.(chan Invoice) <- inv:
			default:
			}
		}
		waitingInvoices.Remove(hash)
	}
}

// creating too many small invoices is forbidden
// because we're not a faucet milking machine

var INVOICESPAMLIMITS = map[string]int64{
	"ridiculously_small_invoices": 1000,
	"very_small_invoices":         5000,
	"small_invoices":              23000,
	"still_small_invoices":        100000,
}

type RateLimiterPolicy struct {
	Key             string `json:"key"`
	TimeUnit        string `json:"time_unit"`
	RequestsPerUnit int    `json:"requests_per_unit"`
}

func setupInvoiceRateLimiter() error {
	auth := req.Header{"X-API-Key": s.RateBucketKey}

	var resp *req.Resp
	var err error

	for key, invMsat := range INVOICESPAMLIMITS {
		maxEvents := int(math.Pow(float64(invMsat)/1000, 0.7))

		resp, err = req.Post("https://api.ratebucket.io/v1/policy/fixed_window", auth,
			req.BodyJSON(RateLimiterPolicy{key, "hour", maxEvents}))
		if err == nil && resp.Response().StatusCode >= 300 {
			err = errors.New(resp.String())
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func checkInvoiceRateLimit(key string, userId int) bool {
	log.Print(key, " ", userId)
	resp, err := req.Get(
		fmt.Sprintf("https://api.ratebucket.io/v1/increment/%s/%d", key, userId))
	if err == nil && resp.Response().StatusCode >= 300 {
		err = errors.New(resp.String())
	}
	if err != nil {
		log.Error().Err(err).Str("key", key).Int("user-id", userId).
			Msg("failed to check/increment rate limit")
		return true
	}

	log.Print(resp.String())
	if gjson.Parse(resp.String()).Get("requests_remaining").Int() < 0 {
		return false
	}

	return true
}

func handleInvoice(ctx context.Context, opts docopt.Opts, desc string) {
	u := ctx.Value("initiator").(User)

	if opts["lnurl"].(bool) {
		// print static lnurl-pay for this user
		lnurl, _ := lnurl.LNURLEncode(
			fmt.Sprintf("%s/lnurl/pay?userid=%d", s.ServiceURL, u.Id))
		send(ctx, qrURL(lnurl), lnurl)
		go u.track("print lnurl", nil)
	} else {
		msats, err := parseSatoshis(opts)
		if err != nil {
			if opts["any"].(bool) {
				msats = 0
			} else {
				handleHelp(ctx, "receive")
				return
			}
		}

		go u.track("make invoice", map[string]interface{}{"sats": msats / 1000})

		bolt11, _, err := u.makeInvoice(ctx, makeInvoiceArgs{
			Msatoshi:    msats,
			Description: desc,
		})
		if err != nil {
			log.Warn().Err(err).Msg("failed to generate invoice")
			send(ctx, u, t.FAILEDINVOICE, t.T{"Err": err.Error()})
			return
		}

		// send invoice with qr code
		send(ctx, qrURL(bolt11), bolt11)
	}
}

func saveInvoiceData(hash string, data InvoiceData) error {
	b, _ := json.Marshal(data)
	return rds.Set("invdata:"+hash, string(b), *data.Expiry).Err()
}

func loadInvoiceData(hash string) (data InvoiceData, err error) {
	b, err := rds.Get("invdata:" + hash).Result()
	if err != nil {
		return
	}
	err = json.Unmarshal([]byte(b), &data)
	return
}
