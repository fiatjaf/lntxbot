package main

import (
	"archive/zip"
	"bytes"
	"context"
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
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
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
	loginSilently          bool
	payWithoutPromptIf     *int64
	balanceCheckService    *string
	payAmountWithoutPrompt *int64
	forceSendComment       string
	anonymous              bool
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
	case lnurl.LNURLPayParams:
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
	key, sig, err := u.SignKeyAuth(params.Host, params.K1)
	if err != nil {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
		return
	}

	var sentsigres lnurl.LNURLResponse
	_, err = napping.Get(params.Callback, &url.Values{
		"key": {key},
		"sig": {sig},
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
			"PublicKey": key,
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

	// stop here if zero
	if params.MaxWithdrawable == 0 {
		return
	}

	// modify description
	desc := params.DefaultDescription
	if opts.balanceCheckService != nil {
		desc += " (automatic)"
		log.Info().Stringer("user", &u).Str("service", params.CallbackURL.Host).
			Msg("performing automatic balanceCheck")
	}

	// lnurl-withdraw: make an invoice with the highest possible value and send
	bolt11, _, err := u.makeInvoice(ctx, &MakeInvoiceArgs{
		IgnoreInvoiceSizeLimit: false,
		Msatoshi:               params.MaxWithdrawable,
		Description:            desc,
	})
	if err != nil {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
		return
	}
	log.Debug().Str("bolt11", bolt11).Str("k1", params.K1).
		Msg("sending invoice to lnurl callback")
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

type RedisPayParams struct {
	Type     string               `json:"type"`
	Params   lnurl.LNURLPayParams `json:"params"`
	MSatoshi int64                `json:"msatoshi"`
}

func handleLNURLPay(
	ctx context.Context,
	u User,
	opts handleLNURLOpts,
	params lnurl.LNURLPayParams,
) {
	receiverName := params.CallbackURL().Host
	if params.Metadata.LightningAddress != "" {
		receiverName = params.Metadata.LightningAddress
	}

	if opts.payAmountWithoutPrompt != nil {
		// we will try to pay this amount and we don't care about anything else

		// except we check for amount between limits
		if *opts.payAmountWithoutPrompt < params.MinSendable || *opts.payAmountWithoutPrompt > params.MaxSendable {
			send(ctx, u, t.LNURLPAYAMOUNTSNOTICE, t.T{
				"Domain": receiverName,
				"Min":    float64(params.MinSendable) / 1000,
				"Max":    float64(params.MaxSendable) / 1000,
				"Exact":  params.MinSendable == params.MaxSendable,
				"NoMax":  params.MaxSendable > 1000000000,
			})
			return
		}

		// ok, now proceed to pay
		lnurlpayFinish(
			ctx,
			u,
			params,
			*opts.payAmountWithoutPrompt,
			opts.forceSendComment,
			opts.anonymous,
		)
		return
	}

	// display metadata and ask for amount
	var fixedAmount int64 = 0
	if params.MaxSendable == params.MinSendable {
		fixedAmount = params.MaxSendable
	}

	go u.track("lnurl-pay", map[string]interface{}{
		"domain": params.CallbackURL().Host,
		"fixed":  float64(fixedAmount) / 1000,
		"max":    float64(params.MaxSendable) / 1000,
		"min":    float64(params.MinSendable) / 1000,
	})

	if fixedAmount > 0 &&
		opts.payWithoutPromptIf != nil &&
		fixedAmount < *opts.payWithoutPromptIf+3000 {
		// we have everything, proceed to pay
		lnurlpayFinish(
			ctx,
			u,
			params,
			fixedAmount,
			"",
			opts.anonymous,
		)
		return
	}

	// must ask for amount, comment or confirmation
	var actionPrompt interface{}
	if fixedAmount == 0 {
		// need the amount
		actionPrompt = &tgbotapi.ForceReply{ForceReply: true}
	} else if params.CommentAllowed == 0 {
		// need a confirmation
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
	}

	var imageURL interface{}
	if params.Metadata.Image.Ext != "" {
		imageURL = tempAssetURL("."+params.Metadata.Image.Ext,
			params.Metadata.Image.Bytes)
	}

	sent := send(ctx, u, t.LNURLPAYPROMPT, t.T{
		"Domain":      receiverName,
		"FixedAmount": float64(fixedAmount) / 1000,
		"Max":         float64(params.MaxSendable) / 1000,
		"Min":         float64(params.MinSendable) / 1000,
		"Text":        params.Metadata.Description,
		"Long":        params.Metadata.LongDescription,
	}, ctx.Value("message"), actionPrompt, imageURL)
	if sent == nil {
		return
	}

	sentId, _ := sent.(int)
	data, _ := json.Marshal(RedisPayParams{Type: "lnurlpay-amount", Params: params})
	rds.Set(fmt.Sprintf("reply:%d:%d", u.Id, sentId), data, time.Hour*1)

	if fixedAmount > 0 && params.CommentAllowed > 0 {
		// need a comment
		lnurlpayAskForComment(ctx, u, params, fixedAmount)
	}
}

func handleLNURLPayAmount(ctx context.Context, msats int64, raw string) {
	u := ctx.Value("initiator").(User)

	// get data from redis object
	var data RedisPayParams
	json.Unmarshal([]byte(raw), &data)

	if data.Params.CommentAllowed > 0 {
		// ask for comment
		lnurlpayAskForComment(ctx, u, data.Params, msats)
	} else {
		// proceed to fetch invoice and pay
		lnurlpayFinish(ctx, u, data.Params, msats, "", false)
	}
}

func handleLNURLPayComment(ctx context.Context, comment string, raw string) {
	u := ctx.Value("initiator").(User)

	// get data from redis object
	var data RedisPayParams
	json.Unmarshal([]byte(raw), &data)

	// proceed to fetch invoice and pay
	lnurlpayFinish(ctx, u, data.Params, data.MSatoshi, comment, false)
}

func lnurlpayAskForComment(
	ctx context.Context,
	u User,
	params lnurl.LNURLPayParams,
	msats int64,
) {
	sent := send(ctx, u, ctx.Value("message"), &tgbotapi.ForceReply{ForceReply: true},
		t.LNURLPAYPROMPTCOMMENT, t.T{"Domain": params.CallbackURL().Host})
	if sent == nil {
		return
	}
	sentId, _ := sent.(int)

	data, _ := json.Marshal(RedisPayParams{
		Type:     "lnurlpay-comment",
		Params:   params,
		MSatoshi: msats,
	})
	rds.Set(fmt.Sprintf("reply:%d:%d", u.Id, sentId), data, time.Hour*1)
}

func lnurlpayFinish(
	ctx context.Context,
	u User,
	params lnurl.LNURLPayParams,
	msats int64,
	comment string,
	anonymous bool,
) {
	var payerdata *lnurl.PayerDataValues
	var proofOfPayerKey *btcec.PrivateKey

	if !anonymous && params.PayerData.Exists() {
		payerdata = &lnurl.PayerDataValues{}

		if params.PayerData.LightningAddress != nil {
			payerdata.LightningAddress = u.Username + "@" + getHost()
		}
		if params.PayerData.FreeName != nil {
			payerdata.FreeName = u.Username
		}
		if params.PayerData.KeyAuth != nil {
			key, sig, err := u.SignKeyAuth(
				params.CallbackURL().Host, params.PayerData.KeyAuth.K1)
			if err != nil {
				send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
				return
			}

			payerdata.KeyAuth = &lnurl.PayerDataKeyAuthValues{
				K1:  params.PayerData.KeyAuth.K1,
				Key: key,
				Sig: sig,
			}
		}
		if params.PayerData.PubKey != nil {
			proofOfPayerKey, _ = btcec.NewPrivateKey(btcec.S256())
			payerdata.PubKey = hex.EncodeToString(
				proofOfPayerKey.PubKey().SerializeCompressed(),
			)
		}
	}

	// call callback with params and get invoice (already verified)
	res, err := params.Call(msats, comment, payerdata)
	if err != nil {
		if lnurlerr, ok := err.(lnurl.LNURLErrorResponse); ok {
			send(ctx, u, t.LNURLERROR, t.T{
				"Host":   params.CallbackURL().Host,
				"Reason": lnurlerr.Reason,
			})
		}

		send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
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

			// send all metadata about this payment as a file to be kept on telegram
			zipbuf := new(bytes.Buffer)
			zip := zip.NewWriter(zipbuf)

			if f, err := zip.Create("metadata.json"); err != nil {
				goto zipfinished
			} else {
				if _, err = f.Write([]byte(params.MetadataEncoded())); err != nil {
					goto zipfinished
				}
			}

			if payerdata != nil {
				if f, err := zip.Create("payerdata.json"); err != nil {
					goto zipfinished
				} else {
					if _, err = f.Write([]byte(res.PayerDataJSON)); err != nil {
						goto zipfinished
					}
				}

				if proofOfPayerKey != nil {
					if f, err := zip.Create("proof-of-payer.key"); err != nil {
						goto zipfinished
					} else {
						if _, err = f.Write(proofOfPayerKey.Serialize()); err != nil {
							goto zipfinished
						}
					}
				}
			}

			err = zip.Close()
			if err != nil {
				goto zipfinished
			}
			//zippedmeta, err := zipfiles(res.ParsedInvoice.DescriptionHash+".json",
			//    "metadata.json", []byte(params.Metadata.Encoded),
			//    "payerdata.json", []byte(res.PayerDataJSON),
			//    ".json", []byte(res.PayerDataJSON),
			//)
		zipfinished:
			if err != nil {
				log.Warn().Err(err).Msg("failed to zip metadata")
				send(ctx, u, t.ERROR, t.T{
					"Err": "Failed to send lnurl-pay metadata. Please report."})
				return
			}

			send(ctx, u, t.LNURLPAYMETADATA, t.T{
				"Domain":         params.CallbackURL().Host,
				"Hash":           res.ParsedInvoice.PaymentHash,
				"HashFirstChars": res.ParsedInvoice.PaymentHash[:5],
			}, tempAssetURL(".zip", zipbuf.Bytes()))

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
					"Domain":        params.CallbackURL().Host,
					"Text":          text,
					"Value":         value,
					"URL":           res.SuccessAction.URL,
					"DecipherError": decerr,
				}, ctx.Value("message"))
			}
		}()
	} else {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()}, processingMessageId)
	}
}

func lnurlBalanceCheckRoutine() {
	ctx := context.WithValue(context.Background(), "origin", "background")

	for {
		log.Debug().Msg("doing global balanceCheck")

		var checks []struct {
			UserID  int    `db:"account"`
			URL     string `db:"url"`
			Service string `db:"service"`
		}
		err := pg.Select(&checks, `SELECT account, service, url FROM balance_check`)
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

			log.Debug().Str("service", check.Service).Stringer("user", &u).
				Msg("")
			handleLNURL(context.WithValue(ctx, "initiator", u),
				check.URL, handleLNURLOpts{balanceCheckService: &check.Service})
		}

		// every day, check everybody's balance on other services
		time.Sleep(time.Hour * 24)
	}
}
