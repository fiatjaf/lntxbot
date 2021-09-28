package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/eclair-go"
	"github.com/fiatjaf/go-lnurl"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/imroc/req"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/tidwall/gjson"
	"gopkg.in/antage/eventsource.v1"
)

type InvoiceData struct {
	UserId    int
	Origin    string // "telegram" or "discord"
	MessageId interface{}
	Preimage  string

	*MakeInvoiceArgs
}

func (inv InvoiceData) Hash() string {
	preimage, err := hex.DecodeString(inv.Preimage)
	if err != nil {
		log.Error().Err(err).Interface("data", inv).
			Msg("failed to decode preimage on InvoiceData")
		return ""
	}
	hash := sha256.Sum256(preimage)
	return hex.EncodeToString(hash[:])
}

type MakeInvoiceArgs struct {
	Description            string
	DescriptionHash        string
	Msatoshi               int64
	Expiry                 *time.Duration
	Tag                    string
	Extra                  InvoiceExtra
	BlueWallet             bool
	IgnoreInvoiceSizeLimit bool
}

type InvoiceExtra struct {
	// lnurlpay comment
	Comment string

	Message *tgbotapi.Message
}

var waitingInvoices = cmap.New() // make(map[string][]chan Invoice)

func waitInvoice(hash string) (inv <-chan InvoiceData) {
	wait := make(chan InvoiceData)
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

func resolveWaitingInvoice(hash string, inv InvoiceData) {
	if chans, ok := waitingInvoices.Get(hash); ok {
		for _, ch := range chans.([]interface{}) {
			select {
			case ch.(chan InvoiceData) <- inv:
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

func checkInvoiceRateLimit(key string, userId int) bool {
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

		if desc == "" {
			desc = "to @lntxbot"
		}

		bolt11, _, err := u.makeInvoice(ctx, &MakeInvoiceArgs{
			Msatoshi:    msats,
			Description: u.Username + ":  " + desc,
			Extra:       InvoiceExtra{Message: ctx.Value("message").(*tgbotapi.Message)},
		})
		if err != nil {
			log.Warn().Err(err).Msg("failed to generate invoice")
			send(ctx, u, t.FAILEDINVOICE, t.T{"Err": err.Error()})
			return
		}

		// send invoice with qr code
		send(ctx, qrURL(bolt11), "<pre>"+bolt11+"</pre>")
	}
}

// what happens when a payment is received
var userPaymentStream = cmap.New() // make(map[int]eventsource.EventSource)

func paymentReceived(
	ctx context.Context,
	hash string,
	amount int64,
) (user User, err error) {
	data, err := loadInvoiceData(hash)
	if err != nil {
		log.Debug().Err(err).Interface("hash", hash).
			Msg("no invoice stored for this hash, not a bot invoice?")
		return
	}

	user, err = loadUser(data.UserId)
	if err != nil {
		log.Error().Err(err).Int("user-id", data.UserId).Interface("data", data).
			Msg("couldn't load user on paymentReceived")
		return
	}

	_, err = pg.Exec(`
INSERT INTO lightning.transaction
  (to_id, amount, description, payment_hash, preimage, tag)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (payment_hash) DO UPDATE SET to_id = $1
    `, user.Id, amount, data.Description, hash,
		data.Preimage, sql.NullString{String: data.Tag, Valid: data.Tag != ""})
	if err != nil {
		log.Error().Err(err).
			Stringer("user", &user).Str("hash", hash).
			Msg("failed to save payment received.")
		send(ctx, user, t.FAILEDTOSAVERECEIVED, t.T{"Hash": hash}, data.MessageId)
		if dmi, ok := data.MessageId.(DiscordMessageID); ok {
			discord.MessageReactionAdd(dmi.Channel(), dmi.Message(), "✅")
		}
		return
	}

	go resolveWaitingInvoice(hash, data)

	user.track("got payment", map[string]interface{}{
		"sats": amount / 1000,
	})

	// send to user stream if the user is listening
	if ies, ok := userPaymentStream.Get(strconv.Itoa(user.Id)); ok {
		go ies.(eventsource.EventSource).SendEventMessage(
			`{"payment_hash": "`+hash+`", "msatoshi": `+
				strconv.FormatInt(data.Msatoshi, 10)+`}`,
			"payment-received", "")
	}

	tmplParams := t.T{
		"Sats": data.Msatoshi / 1000,
		"Hash": hash[:5],
	}

	if comment := data.Extra.Comment; comment != "" {
		name := extractNameFromDesc(comment)
		if name != "" {
			tmplParams["SenderName"] = name
			tmplParams["Comment"] = comment[len(name)+3:]
		} else {
			tmplParams["Comment"] = comment
		}
	}

	send(ctx, user, t.PAYMENTRECEIVED, tmplParams)

	if dmi, ok := data.MessageId.(DiscordMessageID); ok {
		discord.MessageReactionAdd(dmi.Channel(), dmi.Message(), "⚠️")
	}

	return
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

func checkAllIncomingPayments(ctx context.Context) {
	from := time.Now().AddDate(0, 0, -7)

	var lastInvoiceTime time.Time
	err := pg.Get(&lastInvoiceTime, `
    SELECT max(time) FROM lightning.transaction WHERE from_id IS NULL
  `)
	if err != nil {
		log.Error().Err(err).Msg("failed to get last invoice time from db")
	}

	if lastInvoiceTime.Before(from) {
		from = lastInvoiceTime
	}

	res, err := ln.Call("audit", eclair.Params{"from": from.Unix()})
	if err != nil {
		log.Error().Err(err).Msg("failed to call 'audit'")
		return
	}

	log.Debug().Time("from", from).Int64("n", res.Get("received.#").Int()).
		Msg("checking incoming payments")
	for _, recv := range res.Get("received").Array() {
		hash := recv.Get("paymentHash").String()
		var exists bool
		if err := pg.Get(&exists, `
            SELECT true FROM lightning.transaction
            WHERE payment_hash = $1
        `, hash); err != nil && err != sql.ErrNoRows {
			log.Error().Err(err).Str("hash", hash).Msg("checking existence of invoice hash")
			continue
		}

		if !exists {
			var amount int64 = 0
			for _, part := range recv.Get("parts").Array() {
				amount += part.Get("amount").Int()
			}
			go paymentReceived(ctx, hash, amount)
		}
	}
}
