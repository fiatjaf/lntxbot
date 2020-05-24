package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/go-lnurl"
	decodepay_gjson "github.com/fiatjaf/ln-decodepay/gjson"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/gorilla/mux"
	"github.com/skip2/go-qrcode"
	"github.com/tidwall/gjson"
	"gopkg.in/jmcvetta/napping.v3"
)

type handleLNURLOpts struct {
	messageId          int
	loginSilently      bool
	payWithoutPromptIf *int64
}

func handleLNURL(u User, lnurltext string, opts handleLNURLOpts) {
	iparams, err := lnurl.HandleLNURL(lnurltext)
	if err != nil {
		if lnurlerr, ok := err.(lnurl.LNURLErrorResponse); ok {
			u.notify(t.LNURLERROR, t.T{
				"Host":   lnurlerr.URL.Host,
				"Reason": lnurlerr.Reason,
			})
		} else {
			u.notify(t.ERROR, t.T{
				"Err": fmt.Sprintf("failed to fetch lnurl params: %s", err.Error()),
			})
		}
		return
	}

	log.Debug().Interface("params", iparams).Msg("got lnurl params")

	switch params := iparams.(type) {
	case lnurl.LNURLAuthParams:
		// lnurl-auth: create a key based on the user id and sign with it
		seedhash := sha256.Sum256([]byte(fmt.Sprintf("lnurlkeyseed:%s:%d:%s", params.Host, u.Id, s.BotToken)))
		sk, pk := btcec.PrivKeyFromBytes(btcec.S256(), seedhash[:])
		k1, err := hex.DecodeString(params.K1)
		if err != nil {
			u.notify(t.ERROR, t.T{"Err": err.Error()})
			return
		}
		sig, err := sk.Sign(k1)
		if err != nil {
			u.notify(t.ERROR, t.T{"Err": err.Error()})
			return
		}

		signature := hex.EncodeToString(sig.Serialize())
		pubkey := hex.EncodeToString(pk.SerializeCompressed())

		var sentsigres lnurl.LNURLResponse
		resp, err := napping.Get(params.Callback, &url.Values{
			"sig": {signature},
			"key": {pubkey},
		}, &sentsigres, nil)
		if err != nil {
			u.notify(t.ERROR, t.T{"Err": err.Error()})
			return
		}
		if resp.Status() >= 300 {
			u.notify(t.ERROR, t.T{"Err": fmt.Sprintf(
				"Got status %d on callback %s", resp.Status(), params.Callback)})
			return
		}
		if sentsigres.Status == "ERROR" {
			u.notify(t.LNURLERROR, t.T{
				"Host":   params.Host,
				"Reason": sentsigres.Reason,
			})
			return
		}

		if !opts.loginSilently {
			u.notify(t.LNURLAUTHSUCCESS, t.T{
				"Host":      params.Host,
				"PublicKey": pubkey,
			})

			go u.track("lnurl-auth", map[string]interface{}{"domain": params.Host})
		}
	case lnurl.LNURLWithdrawResponse:
		// lnurl-withdraw: make an invoice with the highest possible value and send
		bolt11, _, _, err := u.makeInvoice(makeInvoiceArgs{
			Msatoshi:  params.MaxWithdrawable,
			Desc:      params.DefaultDescription,
			MessageId: opts.messageId,
			SkipQR:    true,
		})
		if err != nil {
			u.notify(t.ERROR, t.T{"Err": err.Error()})
			return
		}
		log.Debug().Str("bolt11", bolt11).Str("k1", params.K1).Msg("sending invoice to lnurl callback")
		var sentinvres lnurl.LNURLResponse
		resp, err := napping.Get(params.Callback, &url.Values{
			"k1": {params.K1},
			"pr": {bolt11},
		}, &sentinvres, nil)
		if err != nil {
			u.notify(t.ERROR, t.T{"Err": err.Error()})
			return
		}
		if resp.Status() >= 300 {
			u.notify(t.ERROR, t.T{"Err": fmt.Sprintf(
				"Got status %d on callback %s", resp.Status(), params.Callback)})
			return
		}
		if sentinvres.Status == "ERROR" {
			u.notify(t.LNURLERROR, t.T{
				"Host":   params.CallbackURL.Host,
				"Reason": sentinvres.Reason,
			})
			return
		}
		go u.track("lnurl-withdraw", map[string]interface{}{"sats": params.MaxWithdrawable})
	case lnurl.LNURLPayResponse1:
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
			lnurlpayFetchInvoiceAndPay(
				u,
				fixedAmount,
				params.Callback,
				params.EncodedMetadata,
				lnurltext,
				opts.messageId,
			)
		} else {
			tmpldata := t.T{
				"Domain":      params.CallbackURL.Host,
				"FixedAmount": float64(fixedAmount) / 1000,
				"Max":         float64(params.MaxSendable) / 1000,
				"Min":         float64(params.MinSendable) / 1000,
			}

			baseChat := tgbotapi.BaseChat{
				ChatID:           u.ChatId,
				ReplyToMessageID: opts.messageId,
			}

			if fixedAmount > 0 {
				baseChat.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData(
							translate(t.CANCEL, u.Locale),
							fmt.Sprintf("cancel=%d", u.Id)),
						tgbotapi.NewInlineKeyboardButtonData(
							translateTemplate(t.PAYAMOUNT, u.Locale,
								t.T{"Sats": float64(fixedAmount) / 1000}),
							fmt.Sprintf("lnurlpay=%d", fixedAmount)),
					),
				)
			} else {
				baseChat.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true}
			}

			var chattable tgbotapi.Chattable
			tmpldata["Text"] = params.ParsedMetadata["text/plain"]
			text := translateTemplate(t.LNURLPAYPROMPT, u.Locale, tmpldata)

			chattable = tgbotapi.MessageConfig{
				BaseChat:              baseChat,
				DisableWebPagePreview: true,
				ParseMode:             "HTML",
				Text:                  text,
			}
			if b64image, ok := params.ParsedMetadata["image/jpeg;base64"]; ok {
				contents, err := base64.StdEncoding.DecodeString(b64image)
				if err == nil {
					chattable = tgbotapi.PhotoConfig{
						BaseFile: tgbotapi.BaseFile{
							BaseChat: baseChat,
							File: tgbotapi.FileBytes{
								Name:  "image",
								Bytes: contents,
							},
							MimeType: "image/jpeg",
						},
						ParseMode: "HTML",
						Caption:   text,
					}
				}
			}
			if b64image, ok := params.ParsedMetadata["image/png;base64"]; ok {
				contents, err := base64.StdEncoding.DecodeString(b64image)
				if err == nil {
					chattable = tgbotapi.PhotoConfig{
						BaseFile: tgbotapi.BaseFile{
							BaseChat: baseChat,
							File: tgbotapi.FileBytes{
								Name:  "image",
								Bytes: contents,
							},
							MimeType: "image/png",
						},
						ParseMode: "HTML",
						Caption:   text,
					}
				}
			}

			sent, err := tgsend(chattable)
			if err != nil {
				log.Warn().Err(err).Msg("error sending lnurl-pay message")
				return
			}

			data, _ := json.Marshal(struct {
				Type     string `json:"type"`
				Metadata string `json:"metadata"`
				URL      string `json:"url"`
				LNURL    string `json:"lnurl"`
			}{"lnurlpay", params.EncodedMetadata, params.Callback, lnurltext})
			rds.Set(fmt.Sprintf("reply:%d:%d", u.Id, sent.MessageID), data, time.Hour*1)
		}
	default:
		u.notifyAsReply(t.LNURLUNSUPPORTED, nil, opts.messageId)
	}

	return
}

func handleLNURLPayConfirmation(u User, msats int64, data gjson.Result, messageId int) {
	// get data from redis object
	callback := data.Get("url").String()
	metadata := data.Get("metadata").String()
	encodedLnurl := data.Get("lnurl").String()

	// proceed to fetch invoice and pay
	lnurlpayFetchInvoiceAndPay(u, msats, callback, metadata, encodedLnurl, messageId)
}

func lnurlpayFetchInvoiceAndPay(
	u User,
	msats int64,
	callback,
	metadata,
	encodedLnurl string,
	messageId int,
) {
	// transform lnurl into bech32ed lnurl if necessary
	encodedLnurl, _ = lnurl.LNURLEncode(encodedLnurl)

	// call callback with params and get invoice
	var res lnurl.LNURLPayResponse2
	resp, err := napping.Get(callback, &url.Values{"amount": {fmt.Sprintf("%d", msats)}}, &res, nil)
	if err != nil {
		u.notify(t.ERROR, t.T{"Err": err.Error()})
		return
	}
	if resp.Status() >= 300 {
		u.notify(t.ERROR, t.T{"Err": fmt.Sprintf(
			"Got status %d on callback %s", resp.Status(), callback)})
		return
	}
	if res.Status == "ERROR" {
		callbackURL, _ := url.Parse(callback)
		if callbackURL == nil {
			callbackURL = &url.URL{Host: "<unknown>"}
		}

		u.notify(t.LNURLERROR, t.T{
			"Host":   callbackURL.Host,
			"Reason": res.Reason,
		})
		return
	}

	log.Debug().Interface("res", res).Msg("got lnurl-pay values")

	// check invoice amount
	decoded, err := decodepay_gjson.Decodepay(res.PR)
	if err != nil {
		u.notify(t.ERROR, t.T{"Err": err.Error()})
		return
	}

	if decoded.Get("description_hash").String() != calculateHash(metadata) {
		u.notify(t.ERROR, t.T{"Err": "Got invoice with wrong description_hash"})
		return
	}

	if decoded.Get("msatoshi").Int() != msats {
		u.notify(t.ERROR, t.T{"Err": "Got invoice with wrong amount."})
		return
	}

	processingMessage := sendMessage(u.ChatId,
		res.PR+"\n\n"+translate(t.PROCESSING, u.Locale),
	)

	// pay it
	hash, err := u.payInvoice(messageId, res.PR, 0)
	if err == nil {
		deleteMessage(&processingMessage)

		// wait until lnurl-pay is paid successfully.
		go func() {
			preimage := <-waitPaymentSuccess(hash)
			bpreimage, _ := hex.DecodeString(preimage)
			callbackURL, _ := url.Parse(callback)

			// send raw metadata, for later checking with the description_hash
			file := tgbotapi.DocumentConfig{
				BaseFile: tgbotapi.BaseFile{
					BaseChat: tgbotapi.BaseChat{ChatID: u.ChatId},
					File: tgbotapi.FileBytes{
						Name:  encodedLnurl + ".json",
						Bytes: []byte(metadata),
					},
					MimeType:    "text/json",
					UseExisting: false,
				},
			}
			file.Caption = translateTemplate(t.LNURLPAYMETADATA, u.Locale, t.T{
				"Domain":         callbackURL.Host,
				"LNURL":          encodedLnurl,
				"Hash":           decoded.Get("payment_hash").String(),
				"HashFirstChars": decoded.Get("payment_hash").String()[:5],
			})
			file.ParseMode = "HTML"
			bot.Send(file)

			// notify user with success action end applicable
			if res.SuccessAction != nil {
				var text string
				var decerr error

				switch res.SuccessAction.Tag {
				case "message":
					text = res.SuccessAction.Message
				case "url":
					text = res.SuccessAction.Description
				case "aes":
					text, decerr = res.SuccessAction.Decipher(bpreimage)
				}

				// give it a time so it's the last message to be sent
				time.Sleep(2 * time.Second)

				u.notifyAsReply(t.LNURLPAYSUCCESS, t.T{
					"Domain":        callbackURL.Host,
					"Text":          text,
					"URL":           res.SuccessAction.URL,
					"DecipherError": decerr,
				}, messageId)
			}
		}()
	} else {
		u.notifyAsReply(t.ERROR, t.T{"Err": err.Error()}, processingMessage.MessageID)
	}
}

func handleLNCreateLNURLWithdraw(u User, sats int, messageId int) (lnurlEncoded string) {
	maxsats := strconv.Itoa(sats)
	ok := u.checkBalanceFor(sats, "lnurl-withdraw", nil)
	if !ok {
		return
	}

	challenge := calculateHash(s.BotToken + ":" + strconv.Itoa(messageId) + ":" + maxsats)

	nexturl := fmt.Sprintf("%s/lnurl/withdraw?message=%d&challenge=%s", s.ServiceURL, messageId, challenge)
	rds.Set("lnurlwithdraw:"+challenge, fmt.Sprintf(`%d-%s`, u.Id, maxsats), s.InvoiceTimeout)

	lnurlEncoded, err := lnurl.LNURLEncode(nexturl)
	if err != nil {
		log.Error().Err(err).Msg("error encoding lnurl on withdraw")
		return
	}

	qrpath := qrImagePath(challenge)
	err = qrcode.WriteFile(strings.ToUpper(lnurlEncoded), qrcode.Medium, 256, qrpath)
	if err != nil {
		log.Error().Err(err).Str("user", u.Username).Str("lnurl", lnurlEncoded).
			Msg("failed to generate lnurl qr. failing.")
		return
	}

	sendMessageWithPicture(u.ChatId, qrpath, `<a href="lightning:`+lnurlEncoded+`">`+lnurlEncoded+"</a>")
	return
}

func serveLNURL() {
	router.Path("/lnurl/withdraw").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debug().Str("url", r.URL.String()).Msg("lnurl-withdraw first request")

		qs := r.URL.Query()
		challenge := qs.Get("challenge")
		messageIdstr := qs.Get("message")
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid user id."))
			return
		}

		challenge = qs.Get("challenge")
		if challenge == "" {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Malformed lnurl."))
			return
		}

		val, err := rds.Get("lnurlwithdraw:" + challenge).Result()
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Unknown lnurl."))
			return
		}

		// get user id and maxWithdrawable from redis value
		parts := strings.Split(val, "-")
		if len(parts) != 2 {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Internal mismatch."))
			return
		}
		chUserId, err1 := strconv.Atoi(parts[0])
		chMax, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Internal mismatch."))
			return
		}
		u, err := loadUser(chUserId, 0)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Couldn't load withdrawee user."))
			return
		}

		json.NewEncoder(w).Encode(lnurl.LNURLWithdrawResponse{
			Callback:           fmt.Sprintf("%s/lnurl/withdraw/invoice/%s", s.ServiceURL, messageIdstr),
			K1:                 challenge,
			MaxWithdrawable:    1000 * int64(chMax),
			MinWithdrawable:    1000 * int64(chMax),
			DefaultDescription: fmt.Sprintf("%s lnurl withdraw @%s", u.AtName(), s.ServiceId),
			Tag:                "withdrawRequest",
			LNURLResponse:      lnurl.OkResponse(),
		})
	})

	router.Path("/lnurl/withdraw/invoice/{messageId}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debug().Str("url", r.URL.String()).Msg("lnurl second request")

		messageIdstr := mux.Vars(r)["messageId"]
		messageId, _ := strconv.Atoi(messageIdstr)

		qs := r.URL.Query()
		challenge := qs.Get("k1")
		bolt11 := qs.Get("pr")

		val, err := rds.Get("lnurlwithdraw:" + challenge).Result()
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Unknown lnurl."))
			return
		}

		// get user id and maxWithdrawable from redis value
		parts := strings.Split(val, "-")
		if len(parts) != 2 {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Internal mismatch."))
			return
		}
		chUserId, err1 := strconv.Atoi(parts[0])
		chMax, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Internal mismatch."))
			return
		}
		u, err := loadUser(chUserId, 0)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Couldn't load withdrawee user."))
			return
		}

		// double-check the challenge (it's a hash of the parameters + our secret)
		if challenge != calculateHash(s.BotToken+":"+messageIdstr+":"+strconv.Itoa(chMax)) {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid amount for this lnurl."))
			return
		}

		if err := rds.Del("lnurlwithdraw:" + challenge).Err(); err != nil {
			// if error stop here to prevent extra withdrawals
			log.Error().Err(err).Str("challenge", challenge).Msg("error deleting used challenge on lnurl withdraw")
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Redis error. Please report."))
			return
		}

		inv, err := ln.Call("decodepay", bolt11)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid payment request."))
			return
		}

		if inv.Get("msatoshi").Int() > int64(chMax)*1000 {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Amount too big."))
			return
		}

		// print the bolt11 just because
		nextMessageId := sendMessageAsReply(u.ChatId, bolt11, messageId).MessageID

		go u.track("outgoing lnurl-withdraw redeemed", map[string]interface{}{
			"sats": inv.Get("msatoshi").Float() / 1000,
		})

		// do the pay flow with these odd opts and fake message.
		opts := docopt.Opts{
			"pay":       true,
			"<invoice>": bolt11,
			"now":       true,
		}
		handlePay(u, opts, nextMessageId, nil)

		json.NewEncoder(w).Encode(lnurl.OkResponse())
	})

	router.Path("/lnurl/pay").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debug().Str("url", r.URL.String()).Msg("lnurl-pay first request")

		qs := r.URL.Query()
		userid := qs.Get("userid")
		username := qs.Get("username")

		u, jmeta, err := lnurlPayStuff(userid, username)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid username or id."))
			return
		}

		go u.track("incoming lnurl-pay attempt", nil)

		json.NewEncoder(w).Encode(lnurl.LNURLPayResponse1{
			LNURLResponse: lnurl.OkResponse(),
			Tag:           "payRequest",
			Callback: fmt.Sprintf("%s/lnurl/pay/callback?%s",
				s.ServiceURL, qs.Encode()),
			MaxSendable:     1000000000,
			MinSendable:     1000,
			EncodedMetadata: string(jmeta),
		})
	})

	router.Path("/lnurl/pay/callback").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		qs := r.URL.Query()
		userid := qs.Get("userid")
		username := qs.Get("username")
		apptag := qs.Get("apptag")
		amount := qs.Get("amount")

		receiver, jmeta, err := lnurlPayStuff(userid, username)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid username or id."))
			return
		}

		var tag string
		if apptag == "golightning" {
			tag = apptag
		}

		msatoshi, err := strconv.ParseInt(amount, 10, 64)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid msatoshi amount."))
			return
		}

		hhash := sha256.Sum256(jmeta)
		bolt11, _, _, err := receiver.makeInvoice(makeInvoiceArgs{
			Msatoshi: msatoshi,
			DescHash: hex.EncodeToString(hhash[:]),
			Tag:      tag,
			SkipQR:   true,
		})
		if err != nil {
			log.Warn().Err(err).Msg("failed to generate lnurl-pay invoice")
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Failed to generate invoice."))
			return
		}

		json.NewEncoder(w).Encode(lnurl.LNURLPayResponse2{
			LNURLResponse: lnurl.OkResponse(),
			PR:            bolt11,
			Routes:        make([][]lnurl.RouteInfo, 0),
			Disposable:    lnurl.FALSE,
		})
	})
}

func lnurlPayStuff(userid string, username string) (receiver User, jmeta []byte, err error) {
	if userid != "" {
		var id int
		id, err = strconv.Atoi(userid)
		if err == nil {
			receiver, err = loadUser(id, 0)
		}
	} else if username != "" {
		receiver, err = ensureUsername(username)
	}
	if err != nil {
		return
	}

	metadata := [][]string{
		[]string{
			"text/plain",
			fmt.Sprintf("Fund %s account on t.me/%s.",
				receiver.AtName(), s.ServiceId),
		},
	}

	if username != "" { /* we may have only a userid */
		if imageURL, err := getUserPictureURL(username); err == nil {
			if b64, err := base64FileFromURL(imageURL); err == nil {
				metadata = append(metadata, []string{"image/jpeg;base64", b64})
			}
		}
	}

	jmeta, err = json.Marshal(metadata)
	return
}
