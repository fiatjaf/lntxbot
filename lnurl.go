package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"git.alhur.es/fiatjaf/lntxbot/bech32"
	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/docopt/docopt-go"
	"github.com/skip2/go-qrcode"
	"gopkg.in/jmcvetta/napping.v3"
)

type LNURLWithdrawResponse struct {
	Callback           string `json:"callback"`
	K1                 string `json:"k1"`
	MaxWithdrawable    int64  `json:"maxWithdrawable"`
	AmountIsFixed      bool   `json:"amountIsFixed"`
	DefaultDescription string `json:"defaultDescription"`
	Tag                string `json:"tag"`
	LNURLResponse
}

type LNURLResponse struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

func handleLNURLReceive(u User, lnurl string, messageId int) {
	actualurl, err := bech32.LNURLDecode(lnurl)
	if err != nil {
		u.notify(t.LNURLINVALID, t.T{"Err": err.Error()})
		return
	}
	log.Debug().Str("url", actualurl).Msg("withdrawing from lnurl")

	var withdrawres LNURLWithdrawResponse
	_, err = napping.Get(actualurl, nil, &withdrawres, nil)
	if err != nil {
		u.notify(t.LNURLFAIL, t.T{"Err": err.Error()})
		return
	}

	if withdrawres.Status == "ERROR" {
		u.notify(t.LNURLFAIL, t.T{"Err": withdrawres.Reason})
		return
	}

	log.Debug().Interface("data", withdrawres).Msg("making invoice for lnurl server")
	bolt11, _, _, err := u.makeInvoice(int(withdrawres.MaxWithdrawable/1000), withdrawres.DefaultDescription,
		"", nil, messageId, "", true)
	if err != nil {
		return
	}

	var sentinvres LNURLResponse
	_, err = napping.Get(withdrawres.Callback, &url.Values{
		"k1": {withdrawres.K1},
		"pr": {bolt11},
	}, &sentinvres, nil)
	if err != nil {
		u.notify(t.LNURLFAIL, t.T{"Err": err.Error()})
		return
	}

	if sentinvres.Status == "ERROR" {
		u.notify(t.LNURLFAIL, t.T{"Err": sentinvres.Reason})
		return
	}

	return
}

func handleLNURLPay(u User, opts docopt.Opts, messageId int) {
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

	lnurl, err := bech32.LNURLEncode(nexturl)
	if err != nil {
		log.Error().Err(err).Msg("error encoding lnurl on withdraw")
		return
	}

	qrpath := qrImagePath(challenge)
	err = qrcode.WriteFile(strings.ToUpper(lnurl), qrcode.Medium, 256, qrpath)
	if err != nil {
		log.Error().Err(err).Str("user", u.Username).Str("lnurl", lnurl).
			Msg("failed to generate lnurl qr. failing.")
		return
	}

	sendMessageWithPicture(u.ChatId, qrpath, `<a href="lightning:`+lnurl+`">`+lnurl+"</a>")
}

func serveLNURL() {
	http.HandleFunc("/lnurl/withdraw", func(w http.ResponseWriter, r *http.Request) {
		log.Debug().Str("url", r.URL.String()).Msg("lnurl first request")

		qs := r.URL.Query()
		challenge := qs.Get("challenge")
		messageIdstr := qs.Get("message")
		userId, err := strconv.Atoi(qs.Get("user"))
		if err != nil {
			json.NewEncoder(w).Encode(LNURLResponse{Status: "ERROR", Reason: "Invalid user id."})
			return
		}

		u, err := loadUser(userId, 0)
		if err != nil {
			json.NewEncoder(w).Encode(LNURLResponse{Status: "ERROR", Reason: "Couldn't load user."})
			return
		}

		fixed := false
		max32, _ := strconv.Atoi(qs.Get("max"))
		max := int64(max32)
		maxmsats := max * 1000
		if max > 0 {
			fixed = true

			// if max is set we must check the challenge as it means the withdraw will be made
			// without conf and we don't want people with invalid challenges draining our money
			challenge = qs.Get("challenge")
			if challenge == "" {
				json.NewEncoder(w).Encode(LNURLResponse{Status: "ERROR", Reason: "Expired secret."})
				return
			}

			// we'll check the challenge and also set it to "used" so we can check again in the next callback
			// but then we don't need the data anymore, just see if it exists.
			if rds.Exists("lnurlwithdrawnoconf:" + challenge).Val() {
				val, err := rds.GetSet("lnurlwithdrawnoconf:"+challenge, "used").Result()
				if err != nil {
					json.NewEncoder(w).Encode(LNURLResponse{Status: "ERROR", Reason: "Bizarre Redis error. Please report."})
					return
				} else if val == "used" {
					json.NewEncoder(w).Encode(LNURLResponse{Status: "ERROR", Reason: "lnurl already used. Please request a new one."})
					return
				}

				parts := strings.Split(val, "-")
				if len(parts) != 2 {
					json.NewEncoder(w).Encode(LNURLResponse{Status: "ERROR", Reason: "Internal mismatch."})
					return
				}
				chUserId, err1 := strconv.Atoi(parts[0])
				chMax, err2 := strconv.Atoi(parts[1])
				if err1 != nil || err2 != nil || chUserId != u.Id || chMax != max32 {
					json.NewEncoder(w).Encode(LNURLResponse{Status: "ERROR", Reason: "Internal mismatch."})
					return
				}

				// everything is fine if we got here
			} else {
				json.NewEncoder(w).Encode(LNURLResponse{Status: "ERROR", Reason: "lnurl already used."})
				return
			}
		} else {
			// otherwise we set max to the total balance (as msats)
			maxmsats = u.getAbsoluteWithdrawable()
		}

		json.NewEncoder(w).Encode(LNURLWithdrawResponse{
			Callback:           fmt.Sprintf("%s/lnurl/withdraw/invoice/%d/%d/%s", s.ServiceURL, u.Id, max32, messageIdstr),
			K1:                 challenge,
			MaxWithdrawable:    maxmsats,
			AmountIsFixed:      fixed,
			DefaultDescription: fmt.Sprintf("%s lnurl withdraw @%s", u.AtName(), s.ServiceId),
			Tag:                "withdrawRequest",
			LNURLResponse:      LNURLResponse{Status: "OK"},
		})
	})

	http.HandleFunc("/lnurl/withdraw/invoice/", func(w http.ResponseWriter, r *http.Request) {
		log.Debug().Str("url", r.URL.String()).Msg("lnurl second request")

		path := strings.Split(r.URL.Path, "/")
		if len(path) < 7 {
			json.NewEncoder(w).Encode(LNURLResponse{Status: "ERROR", Reason: "Invalid URL."})
			return
		}
		urlUserId, err1 := strconv.Atoi(path[4])
		maxsats := path[5]
		urlMax, err2 := strconv.Atoi(maxsats)
		messageIdstr := path[6]
		messageId, _ := strconv.Atoi(messageIdstr)
		if err1 != nil || err2 != nil {
			json.NewEncoder(w).Encode(LNURLResponse{Status: "ERROR", Reason: "Invalid user or maximum amount."})
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
				json.NewEncoder(w).Encode(LNURLResponse{Status: "ERROR", Reason: "Bizarre error, please report."})
				return
			}

			if challenge != calculateHash(s.BotToken+":"+messageIdstr+":"+maxsats) {
				// double-check the challenge (it's a hash of the parameters + our secret)
				json.NewEncoder(w).Encode(LNURLResponse{Status: "ERROR", Reason: "Invalid amount for this lnurl."})
				return
			}

			// stop here to prevent extra withdrawals
			if err := rds.Del("lnurlwithdrawnoconf:" + challenge).Err(); err != nil {
				log.Error().Err(err).Str("challenge", challenge).Msg("error deleting used challenge on lnurl withdraw")
				json.NewEncoder(w).Encode(LNURLResponse{Status: "ERROR", Reason: "Redis error. Please report."})
				return
			}

			// everything fine with the secret challenge, allow automatic pay
			opts["now"] = true
		} else {
			// otherwise we will ask for confirmation as in the normal /pay flow.
		}

		u, err := loadUser(urlUserId, 0)
		if err != nil {
			json.NewEncoder(w).Encode(LNURLResponse{Status: "ERROR", Reason: "Failed to load user."})
			return
		}

		inv, err := ln.Call("decodepay", bolt11)
		if err != nil {
			json.NewEncoder(w).Encode(LNURLResponse{Status: "ERROR", Reason: "Invalid payment request."})
			return
		}

		if urlMax > 0 {
			if inv.Get("msatoshi").Int() > int64(urlMax)*1000 {
				json.NewEncoder(w).Encode(LNURLResponse{Status: "ERROR", Reason: "Amount too big."})
				return
			}
		} else {
			if inv.Get("msatoshi").Int() > u.getAbsoluteWithdrawable() {
				json.NewEncoder(w).Encode(LNURLResponse{Status: "ERROR", Reason: "Amount too big."})
				return
			}
		}

		// print the bolt11 just because
		nextMessageId := sendMessageAsReply(u.ChatId, bolt11, messageId).MessageID

		// do the pay flow with these odd opts and fake message.
		handlePay(u, opts, nextMessageId, nil)

		json.NewEncoder(w).Encode(LNURLResponse{Status: "OK"})
	})
}

var lnurlregex = regexp.MustCompile(`,*?((lnurl)([0-9]{1,}[a-z0-9]+){1})`)

func getLNURL(text string) (url string, ok bool) {
	text = strings.ToLower(text)
	results := lnurlregex.FindStringSubmatch(text)

	if len(results) == 0 {
		return
	}

	return results[1], true
}
