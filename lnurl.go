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

func handleLNURLPay(u User, opts docopt.Opts, messageId int) (lnurlEncoded string) {
	maxsats, _ := opts.String("<satoshis>")
	challenge := calculateHash(s.BotToken + ":" + strconv.Itoa(messageId) + ":" + maxsats)

	nexturl := fmt.Sprintf("%s/lnurl/withdraw?user=%d&message=%d&challenge=%s",
		s.ServiceURL, u.Id, messageId, challenge)
	if maxsats != "" {
		nexturl += "&max=" + maxsats

		// if max is set it means we won't require confirmation before sending the money
		// we will send it to anyone in possession of this challenge string
		rds.Set("lnurlwithdrawnoconf:"+challenge, fmt.Sprintf(`%d-%s`, u.Id, maxsats), s.InvoiceTimeout)
	}

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
		userId, err := strconv.Atoi(qs.Get("user"))
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid user id."))
			return
		}

		u, err := loadUser(userId, 0)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Couldn't load user."))
			return
		}

		// the existence of a "max" parameter means this lnurl withdraw is limited
		// and can be executed without user confirmation.
		max32, _ := strconv.Atoi(qs.Get("max"))
		max := int64(max32)

		// these are only used in the lnurl 1st response which can be used by wallets but don't matter much
		var minmsats int64 = 1000
		var maxmsats int64 = max * 1000

		if max > 0 {
			minmsats = maxmsats // means it's fixed.

			// if max is set we must check the challenge as it means the withdraw will be made
			// without conf and we don't want people with invalid challenges draining our money
			challenge = qs.Get("challenge")
			if challenge == "" {
				json.NewEncoder(w).Encode(lnurl.ErrorResponse("Expired secret."))
				return
			}

			// we'll check the challenge and also set it to "used" so we can check again in the next callback
			// but then we don't need the data anymore, just see if it exists.
			if rds.Exists("lnurlwithdrawnoconf:" + challenge).Val() {
				val, err := rds.GetSet("lnurlwithdrawnoconf:"+challenge, "used").Result()
				if err != nil {
					json.NewEncoder(w).Encode(lnurl.ErrorResponse("Bizarre Redis error. Please report."))
					return
				} else if val == "used" {
					json.NewEncoder(w).Encode(lnurl.ErrorResponse("lnurl already used. Please request a new one."))
					return
				}

				parts := strings.Split(val, "-")
				if len(parts) != 2 {
					json.NewEncoder(w).Encode(lnurl.ErrorResponse("Internal mismatch."))
					return
				}
				chUserId, err1 := strconv.Atoi(parts[0])
				chMax, err2 := strconv.Atoi(parts[1])
				if err1 != nil || err2 != nil || chUserId != u.Id || chMax != max32 {
					json.NewEncoder(w).Encode(lnurl.ErrorResponse("Internal mismatch."))
					return
				}

				// everything is fine if we got here
			} else {
				json.NewEncoder(w).Encode(lnurl.ErrorResponse("lnurl already used."))
				return
			}
		} else {
			// otherwise we set max to the total balance (as msats)
			maxmsats = u.getAbsoluteWithdrawable()
		}

		json.NewEncoder(w).Encode(lnurl.LNURLWithdrawResponse{
			Callback:           fmt.Sprintf("%s/lnurl/withdraw/invoice/%d/%d/%s", s.ServiceURL, u.Id, max32, messageIdstr),
			K1:                 challenge,
			MaxWithdrawable:    maxmsats,
			MinWithdrawable:    minmsats,
			DefaultDescription: fmt.Sprintf("%s lnurl withdraw @%s", u.AtName(), s.ServiceId),
			Tag:                "withdrawRequest",
			LNURLResponse:      lnurl.OkResponse(),
		})
	})

	http.HandleFunc("/lnurl/withdraw/invoice/", func(w http.ResponseWriter, r *http.Request) {
		log.Debug().Str("url", r.URL.String()).Msg("lnurl second request")

		path := strings.Split(r.URL.Path, "/")
		if len(path) < 7 {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid URL."))
			return
		}
		urlUserId, err1 := strconv.Atoi(path[4])
		maxsats := path[5]
		urlMax, err2 := strconv.Atoi(maxsats)
		messageIdstr := path[6]
		messageId, _ := strconv.Atoi(messageIdstr)
		if err1 != nil || err2 != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid user or maximum amount."))
			return
		}

		qs := r.URL.Query()
		challenge := qs.Get("k1")
		bolt11 := qs.Get("pr")

		opts := docopt.Opts{
			"pay":       true,
			"<invoice>": bolt11,
			"now":       false,
		}

		if rds.Exists("lnurlwithdrawnoconf:" + challenge).Val() {
			if rds.Get("lnurlwithdrawnoconf:"+challenge).Val() != "used" {
				log.Error().Err(err).Str("challenge", challenge).
					Msg("challenge is not 'used' on second callback on lnurl withdraw")
				json.NewEncoder(w).Encode(lnurl.ErrorResponse("Bizarre error, please report."))
				return
			}

			if challenge != calculateHash(s.BotToken+":"+messageIdstr+":"+maxsats) {
				// double-check the challenge (it's a hash of the parameters + our secret)
				json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid amount for this lnurl."))
				return
			}

			// stop here to prevent extra withdrawals
			if err := rds.Del("lnurlwithdrawnoconf:" + challenge).Err(); err != nil {
				log.Error().Err(err).Str("challenge", challenge).Msg("error deleting used challenge on lnurl withdraw")
				json.NewEncoder(w).Encode(lnurl.ErrorResponse("Redis error. Please report."))
				return
			}

			// everything fine with the secret challenge, allow automatic pay
			opts["now"] = true
		} else {
			// otherwise we will ask for confirmation as in the normal /pay flow.
		}

		u, err := loadUser(urlUserId, 0)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Failed to load user."))
			return
		}

		inv, err := ln.Call("decodepay", bolt11)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid payment request."))
			return
		}

		if urlMax > 0 {
			if inv.Get("msatoshi").Int() > int64(urlMax)*1000 {
				json.NewEncoder(w).Encode(lnurl.ErrorResponse("Amount too big."))
				return
			}
		} else {
			if inv.Get("msatoshi").Int() > u.getAbsoluteWithdrawable() {
				json.NewEncoder(w).Encode(lnurl.ErrorResponse("Amount too big."))
				return
			}
		}

		// print the bolt11 just because
		nextMessageId := sendMessageAsReply(u.ChatId, bolt11, messageId).MessageID

		// do the pay flow with these odd opts and fake message.
		handlePay(u, opts, nextMessageId, nil)

		json.NewEncoder(w).Encode(lnurl.OkResponse())
	})
}
