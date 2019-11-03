package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/btcsuite/btcd/btcec"
	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/go-lnurl"
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
			"K1":        params.K1,
			"PublicKey": pubkey,
			"Signature": signature,
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
	default:
		u.notifyAsReply(t.LNURLUNSUPPORTED, nil, messageId)
	}

	return
}

func handleLNURLPay(u User, satoshis int, messageId int) (lnurlEncoded string) {
	maxsats := strconv.Itoa(satoshis)
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
	http.HandleFunc("/lnurl/withdraw", func(w http.ResponseWriter, r *http.Request) {
		log.Debug().Str("url", r.URL.String()).Msg("lnurl first request")

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

	http.HandleFunc("/lnurl/withdraw/invoice/", func(w http.ResponseWriter, r *http.Request) {
		log.Debug().Str("url", r.URL.String()).Msg("lnurl second request")

		path := strings.Split(r.URL.Path, "/")
		messageIdstr := path[len(path)-1]
		messageId, _ := strconv.Atoi(messageIdstr)

		qs := r.URL.Query()
		challenge := qs.Get("k1")
		bolt11 := qs.Get("pr")

		opts := docopt.Opts{
			"pay":       true,
			"<invoice>": bolt11,
			"now":       false,
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
		opts["now"] = true
		handlePay(u, opts, nextMessageId, nil)

		json.NewEncoder(w).Encode(lnurl.OkResponse())
	})
}
