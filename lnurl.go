package main

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"git.alhur.es/fiatjaf/lntxbot/bech32"
	"git.alhur.es/fiatjaf/lntxbot/t"
	napping "gopkg.in/jmcvetta/napping.v3"
)

type LNURLWithdrawResponse struct {
	Callback           string `json:"callback"`
	K1                 string `json:"k1"`
	MaxWithdrawable    int64  `json:"maxWithdrawable"`
	DefaultDescription string `json:"defaultDescription"`
	Tag                string `json:"tag"`
	LNURLResponse
}

type LNURLResponse struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func handleLNURL(u User, lnurl string, messageId int) {
	actualurl, err := bech32.LNURL(lnurl)
	if err != nil {
		u.notify(t.LNURLINVALID, t.T{"Err": err.Error()})
		return
	}

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

func serveLNURL() {
	http.HandleFunc("/lnurl/", func(w http.ResponseWriter, r *http.Request) {

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
