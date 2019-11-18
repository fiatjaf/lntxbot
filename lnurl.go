package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/btcsuite/btcd/btcec"
	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/go-lnurl"
	"github.com/fiatjaf/ln-decodepay/gjson"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/gorilla/mux"
	"github.com/skip2/go-qrcode"
	"gopkg.in/jmcvetta/napping.v3"
)

func handleLNURL(u User, lnurltext string, messageId int) {
	iparams, err := lnurl.HandleLNURL(lnurltext)
	if err != nil {
		u.notify(t.ERROR, t.T{"Err": err.Error()})
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
		_, err = napping.Get(params.Callback, &url.Values{
			"sig": {signature},
			"key": {pubkey},
		}, &sentsigres, nil)
		if err != nil {
			u.notify(t.ERROR, t.T{"Err": err.Error()})
			return
		}
		if sentsigres.Status == "ERROR" {
			u.notify(t.ERROR, t.T{"Err": sentsigres.Reason})
			return
		}
		u.notify(t.LNURLAUTHSUCCESS, t.T{
			"Host":      params.Host,
			"PublicKey": pubkey,
		})
	case lnurl.LNURLWithdrawResponse:
		// lnurl-withdraw: make an invoice with the highest possible value and send
		bolt11, _, _, err := u.makeInvoice(int(params.MaxWithdrawable/1000), params.DefaultDescription,
			"", nil, messageId, "", "", true)
		if err != nil {
			u.notify(t.ERROR, t.T{"Err": err.Error()})
			return
		}
		log.Debug().Str("bolt11", bolt11).Str("k1", params.K1).Msg("sending invoice to lnurl callback")
		var sentinvres lnurl.LNURLResponse
		_, err = napping.Get(params.Callback, &url.Values{
			"k1": {params.K1},
			"pr": {bolt11},
		}, &sentinvres, nil)
		if err != nil {
			u.notify(t.ERROR, t.T{"Err": err.Error()})
			return
		}
		if sentinvres.Status == "ERROR" {
			u.notify(t.ERROR, t.T{"Err": sentinvres.Reason})
			return
		}
	case lnurl.LNURLPayResponse1:
		// display metadata and ask for amount
		var fixedAmount int64 = 0
		if params.MaxSendable == params.MinSendable {
			fixedAmount = params.MaxSendable
		}

		tmpldata := t.T{
			"Domain":      params.CallbackURL.Host,
			"FixedAmount": float64(fixedAmount) / 1000,
			"Max":         float64(params.MaxSendable) / 1000,
			"Min":         float64(params.MinSendable) / 1000,
		}

		baseChat := tgbotapi.BaseChat{
			ChatID:           u.ChatId,
			ReplyToMessageID: messageId,
		}

		if fixedAmount > 0 {
			baseChat.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(
						translate(t.CANCEL, u.Locale),
						fmt.Sprintf("cancel=%d", u.Id)),
					tgbotapi.NewInlineKeyboardButtonData(
						translate(t.CONFIRM, u.Locale),
						fmt.Sprintf("lnurlpay=%d", fixedAmount)),
				),
			)
		} else {
			baseChat.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true}
		}

		var chattable tgbotapi.Chattable
		for _, pair := range params.Metadata {
			switch pair[0] {
			case "text/plain":
				tmpldata["Text"] = pair[1]
				message := tgbotapi.MessageConfig{
					BaseChat:              baseChat,
					ParseMode:             "HTML",
					DisableWebPagePreview: true,
					Text: translateTemplate(t.LNURLPAYPROMPT, u.Locale, tmpldata),
				}
				chattable = message
			case "text/html":
				tmpldata["HTML"] = pair[1]
				message := tgbotapi.MessageConfig{
					BaseChat:              baseChat,
					ParseMode:             "HTML",
					DisableWebPagePreview: true,
					Text: translateTemplate(t.LNURLPAYPROMPT, u.Locale, tmpldata),
				}
				chattable = message
			}
		}

		sent, err := bot.Send(chattable)
		if err != nil {
			log.Warn().Err(err).Msg("error sending lnurl-pay message")
			return
		}

		key := fmt.Sprintf("reply:%d:%d", u.Id, sent.MessageID)
		data, _ := json.Marshal(struct {
			Type            string `json:"type"`
			DescriptionHash string `json:"h"`
			URL             string `json:"url"`
		}{"lnurlpay", calculateHash(params.EncodedMetadata), params.Callback})
		rds.Set(key, data, time.Hour*1)
	default:
		u.notifyAsReply(t.LNURLUNSUPPORTED, nil, messageId)
	}

	return
}

func handleLNURLPayConfirmation(u User, msats int64, callback string, descriptionHash string, messageId int) {
	// call callback with params and get invoice
	var res lnurl.LNURLPayResponse2
	_, err = napping.Get(callback, &url.Values{"amount": {fmt.Sprintf("%d", msats)}}, &res, nil)
	if err != nil {
		u.notify(t.ERROR, t.T{"Err": err.Error()})
		return
	}
	if res.Status == "ERROR" {
		u.notify(t.ERROR, t.T{"Err": res.Reason})
		return
	}

	// check invoice amount
	decoded, err := decodepay_gjson.Decodepay(res.PR)
	if err != nil {
		u.notify(t.ERROR, t.T{"Err": err.Error()})
		return
	}

	if decoded.Get("description_hash").String() != descriptionHash {
		u.notify(t.ERROR, t.T{"Err": "Got invoice with wrong description_hash"})
		return
	}

	if decoded.Get("msatoshi").Int() != msats {
		u.notify(t.ERROR, t.T{"Err": "Got invoice with wrong amount."})
		return
	}

	// pay it
	opts := docopt.Opts{
		"pay":       true,
		"<invoice>": res.PR,
		"now":       true,
	}
	handlePay(u, opts, messageId, nil)
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

		_, jmeta, err := lnurlPayDuplicatedStuff(userid, username)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid username or id."))
			return
		}

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

		receiver, jmeta, err := lnurlPayDuplicatedStuff(userid, username)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid username or id."))
			return
		}

		var tag string
		if apptag == "golightning" {
			tag = apptag
		}

		preimage := make([]byte, 32)
		rand.Read(preimage)

		msatoshi, err := strconv.ParseInt(amount, 10, 64)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid msatoshi amount."))
			return
		}

		bolt11, err := ln.InvoiceWithDescriptionHash(
			makeLabel(receiver.Id, 0, hex.EncodeToString(preimage), tag),
			msatoshi,
			string(jmeta),
			nil,
			nil,
		)
		if err != nil {
			log.Warn().Err(err).Msg("failed to generate lnurl-pay invoice")
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Failed to generate invoice."))
			return
		}

		json.NewEncoder(w).Encode(lnurl.LNURLPayResponse2{
			LNURLResponse: lnurl.OkResponse(),
			PR:            bolt11,
			Routes:        make([][]lnurl.RouteInfo, 0),
		})
	})
}

func lnurlPayDuplicatedStuff(userid string, username string) (receiver User, jmeta []byte, err error) {
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

	jmeta, err = json.Marshal([][]string{
		[]string{
			"text/plain",
			fmt.Sprintf("Donation to %s on t.me/%s.",
				receiver.AtName(), s.ServiceId),
		},
	})

	return
}
