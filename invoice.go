package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/go-lnurl"
	decodepay "github.com/fiatjaf/ln-decodepay"
	"github.com/fiatjaf/lntxbot/t"
	cmap "github.com/orcaman/concurrent-map"
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

type InvoiceSpamLimit struct {
	EqualOrSmallerThan int64
	Key                string
	PerDay             int
}

var INVOICESPAMLIMITS = []InvoiceSpamLimit{
	{1000, "<=1", 1},
	{10000, "<=10", 3},
	{100000, "<=100", 10},
}

func onInvoicePaid(ctx context.Context, hash string, data ShadowChannelData) {
	log.Print("invoice paid ", data)
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
		switch data.Origin {
		case "telegram":
			mid, _ := data.MessageId.(int)
			send(ctx, receiver, t.FAILEDTOSAVERECEIVED, t.T{"Hash": hash}, mid)
		case "discord":
			send(ctx, receiver, t.FAILEDTOSAVERECEIVED, t.T{"Hash": hash})
			if dmi, ok := data.MessageId.(DiscordMessageID); ok {
				discord.MessageReactionAdd(dmi.Channel(), dmi.Message(), "✅")
			}
		}
		return
	}

	switch data.Origin {
	case "telegram":
		mid, _ := data.MessageId.(int)
		send(ctx, receiver, t.PAYMENTRECEIVED, t.T{
			"Sats": data.Msatoshi / 1000,
			"Hash": hash[:5],
		}, mid)
	case "discord":
		send(ctx, receiver, t.PAYMENTRECEIVED, t.T{
			"Sats": data.Msatoshi / 1000,
			"Hash": hash[:5],
		})

		if dmi, ok := data.MessageId.(DiscordMessageID); ok {
			discord.MessageReactionAdd(dmi.Channel(), dmi.Message(), "⚠️")
		}
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
