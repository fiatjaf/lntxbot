package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
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

var waitingInvoices = cmap.New() // make(map[string][]chan gjson.Result)

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

// we make a short channel id that contains an id to an object on redis with all things.
// besides storing these important values, this secret will also be used to build the
// preimage.
type ShadowChannelData struct {
	UserId          int
	Origin          string // "telegram" or "discord"
	MessageId       interface{}
	Tag             string
	Msatoshi        int64
	Description     string
	DescriptionHash string
	Preimage        string
	Extra           map[string]interface{}
}

func makeShadowChannelId(data ShadowChannelData) uint64 {
	secret := make([]byte, 8)
	rand.Read(secret)

	key := hex.EncodeToString(secret)
	j, _ := json.Marshal(data)
	rds.Set(key, string(j), time.Hour*24*7)

	return binary.BigEndian.Uint64(secret)
}

func extractDataFromShadowChannelId(channelId uint64) (data ShadowChannelData, ok bool) {
	bin := make([]byte, 8)
	binary.BigEndian.PutUint64(bin, channelId)

	key := hex.EncodeToString(bin)
	j, err := rds.Get(key).Result()
	if err != nil {
		return
	}

	err = json.Unmarshal([]byte(j), &data)
	if err != nil {
		return
	}

	return data, true
}

func deleteDataAssociatedWithShadowChannelId(channelId uint64) error {
	bin := make([]byte, 8)
	binary.BigEndian.PutUint64(bin, channelId)

	key := hex.EncodeToString(bin)
	return rds.Del(key).Err()
}

func decodeShortChannelId(scid string) (uint64, error) {
	spl := strings.Split(scid, "x")

	x, err := strconv.ParseUint(spl[0], 10, 64)
	if err != nil {
		return 0, err
	}
	y, err := strconv.ParseUint(spl[1], 10, 64)
	if err != nil {
		return 0, err
	}
	z, err := strconv.ParseUint(spl[2], 10, 64)
	if err != nil {
		return 0, err
	}

	return ((x & 0xFFFFFF) << 40) | ((y & 0xFFFFFF) << 16) | (z & 0xFFFF), nil
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

// what happens when a payment is received
func onInvoicePaid(ctx context.Context, hash string, data ShadowChannelData) {
	receiver, err := loadUser(data.UserId)
	if err != nil {
		log.Warn().Err(err).
			Interface("shadow-data", data).
			Msg("failed to load on onInvoicePaid")
		return
	}

	receiver.track("got payment", map[string]interface{}{
		"sats": float64(data.Msatoshi) / 1000,
	})

	// is there a comment associated with this?
	go func() {
		time.Sleep(3 * time.Second)
		if comment, ok := data.Extra["comment"]; ok && comment != "" {
			send(ctx, receiver, t.LNURLPAYCOMMENT, t.T{
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
		send(ctx, receiver, t.FAILEDTOSAVERECEIVED, t.T{"Hash": hash}, data.MessageId)
		if dmi, ok := data.MessageId.(DiscordMessageID); ok {
			discord.MessageReactionAdd(dmi.Channel(), dmi.Message(), "✅")
		}
		return
	}

	send(ctx, receiver, t.PAYMENTRECEIVED, t.T{
		"Sats": data.Msatoshi / 1000,
		"Hash": hash[:5],
	})

	if dmi, ok := data.MessageId.(DiscordMessageID); ok {
		discord.MessageReactionAdd(dmi.Channel(), dmi.Message(), "⚠️")
	}
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
		sats, err := parseSatoshis(opts)
		if err != nil {
			if opts["any"].(bool) {
				sats = 0
			} else {
				handleHelp(ctx, "receive")
				return
			}
		}

		go u.track("make invoice", map[string]interface{}{"sats": sats})

		bolt11, _, err := u.makeInvoice(ctx, makeInvoiceArgs{
			Msatoshi: int64(sats) * 1000,
			Desc:     desc,
		})
		if err != nil {
			log.Warn().Err(err).Msg("failed to generate invoice")
			send(ctx, u, t.FAILEDINVOICE, t.T{"Err": messageFromError(err)})
			return
		}

		// send invoice with qr code
		send(ctx, qrURL(bolt11), bolt11)
	}
}
