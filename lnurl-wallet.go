package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/fiatjaf/go-lnurl"
	decodepay "github.com/fiatjaf/ln-decodepay"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/tidwall/gjson"
	"gopkg.in/jmcvetta/napping.v3"
)

func serveLNURLBalanceNotify() {
	ctx := context.WithValue(context.Background(), "origin", "external")

	router.Path("/lnurl/withdraw/notify").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)

		qs := r.URL.Query()
		service := qs.Get("service")
		user, _ := strconv.Atoi(qs.Get("user"))

		log.Debug().Str("url", r.URL.String()).Str("service", service).Int("user", user).
			Msg("lnurl-withdraw balance notify")

		u, err := loadUser(user)
		if err != nil {
			return
		}

		balanceCheck, err := u.loadBalanceCheckURL(service)
		if err != nil {
			log.Warn().Err(err).Msg("loading balanceCheck url")
			return
		}

		if balanceCheck != "" {
			handleLNURL(context.WithValue(ctx, "initiator", u),
				balanceCheck, handleLNURLOpts{balanceCheckService: &service})
		}
	})
}

type handleLNURLOpts struct {
	loginSilently       bool
	payWithoutPromptIf  *int64
	balanceCheckService *string
}

func handleLNURL(ctx context.Context, lnurltext string, opts handleLNURLOpts) {
	u := ctx.Value("initiator").(User)

	_, iparams, err := lnurl.HandleLNURL(lnurltext)
	if err != nil {
		if lnurlerr, ok := err.(lnurl.LNURLErrorResponse); ok {
			send(ctx, u, t.LNURLERROR, t.T{
				"Host":   lnurlerr.URL.Host,
				"Reason": lnurlerr.Reason,
			})
		} else {
			send(ctx, u, t.ERROR, t.T{
				"Err": fmt.Sprintf("failed to fetch lnurl params: %s", err.Error()),
			})

			// cancel automatic balance checks
			if opts.balanceCheckService != nil {
				err := u.saveBalanceCheckURL(*opts.balanceCheckService, "")
				if err == nil {
					send(ctx, u, t.LNURLBALANCECHECKCANCELED, t.T{
						"Service": *opts.balanceCheckService,
					})
				}
			}
		}
		return
	}

	switch params := iparams.(type) {
	case lnurl.LNURLAuthParams:
		handleLNURLAuth(ctx, u, opts, params)
	case lnurl.LNURLWithdrawResponse:
		handleLNURLWithdraw(ctx, u, opts, params)
	case lnurl.LNURLPayResponse1:
		handleLNURLPay(ctx, u, opts, params)
	default:
		send(ctx, u, t.LNURLUNSUPPORTED, ctx.Value("message"))
	}

	return
}

func handleLNURLAuth(
	ctx context.Context,
	u User,
	opts handleLNURLOpts,
	params lnurl.LNURLAuthParams,
) {
	// lnurl-auth: create a key based on the user id and sign with it
	seedhash := sha256.Sum256([]byte(fmt.Sprintf("lnurlkeyseed:%s:%d:%s", params.Host, u.Id, s.TelegramBotToken)))
	sk, pk := btcec.PrivKeyFromBytes(btcec.S256(), seedhash[:])
	k1, err := hex.DecodeString(params.K1)
	if err != nil {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
		return
	}
	sig, err := sk.Sign(k1)
	if err != nil {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
		return
	}

	signature := hex.EncodeToString(sig.Serialize())
	pubkey := hex.EncodeToString(pk.SerializeCompressed())

	var sentsigres lnurl.LNURLResponse
	_, err = napping.Get(params.Callback, &url.Values{
		"sig": {signature},
		"key": {pubkey},
	}, &sentsigres, &sentsigres)
	if err != nil {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
		return
	}
	if sentsigres.Status == "ERROR" {
		send(ctx, u, t.LNURLERROR, t.T{
			"Host":   params.Host,
			"Reason": sentsigres.Reason,
		})
		return
	}

	if !opts.loginSilently {
		send(ctx, u, t.LNURLAUTHSUCCESS, t.T{
			"Host":      params.Host,
			"PublicKey": pubkey,
		})

		go u.track("lnurl-auth", map[string]interface{}{"domain": params.Host})
	}
}

func handleLNURLWithdraw(
	ctx context.Context,
	u User,
	opts handleLNURLOpts,
	params lnurl.LNURLWithdrawResponse,
) {
	// if possible, save this
	u.saveBalanceCheckURL(params.CallbackURL.Host, params.BalanceCheck)

	// modify description
	desc := params.DefaultDescription
	if opts.balanceCheckService != nil {
		desc += " (automatic)"
		log.Info().Stringer("user", &u).Str("service", params.CallbackURL.Host).
			Msg("performing automatic balanceCheck")
	}

	// lnurl-withdraw: make an invoice with the highest possible value and send
	bolt11, _, err := u.makeInvoice(ctx, makeInvoiceArgs{
		IgnoreInvoiceSizeLimit: false,
		Msatoshi:               params.MaxWithdrawable,
		Desc:                   desc,
	})
	if err != nil {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
		return
	}
	log.Debug().Str("bolt11", bolt11).Str("k1", params.K1).Msg("sending invoice to lnurl callback")
	var sentinvres lnurl.LNURLResponse
	_, err = napping.Get(params.Callback, &url.Values{
		"k1": {params.K1},
		"pr": {bolt11},
		"balanceNotify": {fmt.Sprintf("%s/lnurl/withdraw/notify?service=%s&user=%d",
			s.ServiceURL, params.CallbackURL.Host, u.Id)},
	}, &sentinvres, &sentinvres)
	if err != nil {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
		return
	}
	if sentinvres.Status == "ERROR" {
		send(ctx, u, t.LNURLERROR, t.T{
			"Host":   params.CallbackURL.Host,
			"Reason": sentinvres.Reason,
		})
		return
	}
	go u.track("lnurl-withdraw", map[string]interface{}{"sats": params.MaxWithdrawable})
}

func handleLNURLPay(
	ctx context.Context,
	u User,
	opts handleLNURLOpts,
	params lnurl.LNURLPayResponse1,
) {
	// display metadata and ask for amount
	var fixedAmount int64 = 0
	if params.MaxSendable == params.MinSendable {
		fixedAmount = params.MaxSendable
	}

	go u.track("lnurl-pay", map[string]interface{}{
		"domain": params.CallbackURL.Host,
		"fixed":  float64(fixedAmount) / 1000,
		"max":    float64(params.MaxSendable) / 1000,
		"min":    float64(params.MinSendable) / 1000,
	})

	if fixedAmount > 0 &&
		opts.payWithoutPromptIf != nil &&
		fixedAmount < *opts.payWithoutPromptIf+3000 {
		// we have the amount already

		if params.CommentAllowed > 0 {
			// need a comment
			lnurlpayAskForComment(ctx, u, params.Callback,
				params.EncodedMetadata, fixedAmount)
		} else {
			// we have everything, proceed to pay
			lnurlpayFinish(
				ctx,
				u,
				fixedAmount,
				"",
				params.Callback,
				params.EncodedMetadata,
			)
		}
	} else {
		// must ask for amount or confirmation
		var actionPrompt interface{}
		if fixedAmount > 0 {
			keyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(
						translate(ctx, t.CANCEL),
						fmt.Sprintf("cancel=%d", u.Id)),
					tgbotapi.NewInlineKeyboardButtonData(
						translateTemplate(ctx, t.PAYAMOUNT,
							t.T{"Sats": float64(fixedAmount) / 1000}),
						fmt.Sprintf("lnurlpay=%d", fixedAmount)),
				),
			)
			actionPrompt = &keyboard
		} else {
			actionPrompt = &tgbotapi.ForceReply{ForceReply: true}
		}

		var imageURL interface{}
		if params.Metadata.ImageExtension() != "" {
			imageURL = tempAssetURL("."+params.Metadata.ImageExtension(),
				params.Metadata.ImageBytes())
		}

		sent := send(ctx, u, t.LNURLPAYPROMPT, t.T{
			"Domain":      params.CallbackURL.Host,
			"FixedAmount": float64(fixedAmount) / 1000,
			"Max":         float64(params.MaxSendable) / 1000,
			"Min":         float64(params.MinSendable) / 1000,
			"Text":        params.Metadata.Description(),
		}, ctx.Value("message"), actionPrompt, imageURL)
		if sent == nil {
			return
		}

		sentId, _ := sent.(int)
		data, _ := json.Marshal(struct {
			Type         string `json:"type"`
			Metadata     string `json:"metadata"`
			URL          string `json:"url"`
			NeedsComment bool   `json:"needs_comment"`
		}{"lnurlpay-amount", params.EncodedMetadata, params.Callback,
			params.CommentAllowed > 0})
		rds.Set(fmt.Sprintf("reply:%d:%d", u.Id, sentId), data, time.Hour*1)
	}
}

func handleLNURLPayAmount(
	ctx context.Context,
	msats int64,
	data gjson.Result,
) {
	u := ctx.Value("initiator").(User)

	// get data from redis object
	callback := data.Get("url").String()
	metadata := data.Get("metadata").String()
	needsComment := data.Get("needs_comment").Bool()

	if needsComment {
		// ask for comment
		lnurlpayAskForComment(ctx, u, callback, metadata, msats)
	} else {
		// proceed to fetch invoice and pay
		lnurlpayFinish(ctx, u, msats, "", callback, metadata)
	}
}

func handleLNURLPayComment(ctx context.Context, comment string, data gjson.Result) {
	u := ctx.Value("initiator").(User)

	// get data from redis object
	callback := data.Get("url").String()
	metadata := data.Get("metadata").String()
	msats := data.Get("msatoshi").Int()

	// proceed to fetch invoice and pay
	lnurlpayFinish(ctx, u, msats, comment, callback, metadata)
}

func lnurlpayAskForComment(
	ctx context.Context,
	u User,
	callback,
	metadata string,
	msats int64,
) {
	callbackURL, _ := url.Parse(callback)
	sent := send(ctx, u, ctx.Value("message"), &tgbotapi.ForceReply{ForceReply: true},
		t.LNURLPAYPROMPTCOMMENT, t.T{"Domain": callbackURL.Host})
	if sent == nil {
		return
	}
	sentId, _ := sent.(int)

	data, _ := json.Marshal(struct {
		Type     string `json:"type"`
		Metadata string `json:"metadata"`
		MSatoshi int64  `json:"msatoshi"`
		URL      string `json:"url"`
	}{"lnurlpay-comment", metadata, msats, callback})
	rds.Set(fmt.Sprintf("reply:%d:%d", u.Id, sentId), data, time.Hour*1)
}

func lnurlpayFinish(
	ctx context.Context,
	u User,
	msats int64,
	comment string,
	callback string,
	metadata string,
) {
	params := &url.Values{
		"amount": {fmt.Sprintf("%d", msats)},
	}
	if comment != "" {
		params.Set("comment", comment)
	}

	// call callback with params and get invoice
	var res lnurl.LNURLPayResponse2
	_, err := napping.Get(callback, params, &res, &res)
	if err != nil {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
		return
	}
	if res.Status == "ERROR" {
		callbackURL, _ := url.Parse(callback)
		if callbackURL == nil {
			callbackURL = &url.URL{Host: "<unknown>"}
		}

		send(ctx, u, t.LNURLERROR, t.T{
			"Host":   callbackURL.Host,
			"Reason": res.Reason,
		})
		return
	}

	log.Debug().Interface("res", res).Msg("got lnurl-pay values")

	// check invoice amount
	inv, err := decodepay.Decodepay(res.PR)
	if err != nil {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
		return
	}

	if inv.DescriptionHash != hashString(metadata) {
		send(ctx, u, t.ERROR, t.T{"Err": "Got invoice with wrong description_hash"})
		return
	}

	if int64(inv.MSatoshi) != msats {
		send(ctx, u, t.ERROR, t.T{"Err": "Got invoice with wrong amount."})
		return
	}

	processingMessageId := send(ctx, u, res.PR+"\n\n"+translate(ctx, t.PROCESSING))

	// pay it
	hash, err := u.payInvoice(ctx, res.PR, 0)
	if err == nil {
		deleteMessage(&tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: u.TelegramChatId},
			MessageID: processingMessageId.(int),
		})

		// wait until lnurl-pay is paid successfully.
		go func() {
			preimage := <-waitPaymentSuccess(hash)
			bpreimage, _ := hex.DecodeString(preimage)
			callbackURL, _ := url.Parse(callback)

			// send raw metadata, for later checking with the description_hash
			zippedmeta, err := zipdata(hashString(metadata)+".json", []byte(metadata))
			if err != nil {
				log.Warn().Err(err).Msg("failed to zip metadata")
				send(ctx, u, t.ERROR, t.T{
					"Err": "Failed to send lnurl-pay metadata. Please report."})
				return
			}

			send(ctx, u, t.LNURLPAYMETADATA, t.T{
				"Domain":         callbackURL.Host,
				"Hash":           inv.PaymentHash,
				"HashFirstChars": inv.PaymentHash[:5],
			}, tempAssetURL(".zip", zippedmeta))

			// notify user with success action end applicable
			if res.SuccessAction != nil {
				var text string
				var decerr error
				var value string

				switch res.SuccessAction.Tag {
				case "message":
					text = res.SuccessAction.Message
				case "url":
					text = res.SuccessAction.Description
				case "aes":
					text = res.SuccessAction.Description
					value, decerr = res.SuccessAction.Decipher(bpreimage)
				}

				// give it a time so it's the last message to be sent
				time.Sleep(2 * time.Second)

				send(ctx, u, t.LNURLPAYSUCCESS, t.T{
					"Domain":        callbackURL.Host,
					"Text":          text,
					"Value":         value,
					"URL":           res.SuccessAction.URL,
					"DecipherError": decerr,
				}, ctx.Value("message"))
			}
		}()
	} else {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()}, processingMessageId.(int))
	}
}

func lnurlBalanceCheckRoutine() {
	ctx := context.WithValue(context.Background(), "origin", "background")

	for {
		// every day, check everybody's balance on other services
		time.Sleep(time.Hour * 24)

		var checks []struct {
			UserID  int    `db:"account"`
			URL     string `db:"url"`
			Service string `db:"service"`
		}
		err = pg.Get(&checks, `SELECT account, service, url FROM balance_check`)
		if err == sql.ErrNoRows {
			err = nil
		}
		if err != nil {
			log.Error().Err(err).Msg("failed to fetch balance_checks on routine")
		}

		for _, check := range checks {
			u, err := loadUser(check.UserID)
			if err != nil {
				log.Error().Err(err).Int("user", check.UserID).Str("url", check.URL).
					Msg("failed to load user on balance_checks routine")
				continue
			}

			handleLNURL(context.WithValue(ctx, "initiator", u),
				check.URL, handleLNURLOpts{balanceCheckService: &check.Service})
		}
	}
}
