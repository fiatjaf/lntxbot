package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
)

type GoLightningResponse struct {
	GUID    string `json:"guid"`
	Id      string `json:"id"`
	Address string `json:"bitcoinAddress"`
	Price   string `json:"btcPrice"`
	Error   string `json:"error"`
}

func prepareGoLightningTransaction(user User, messageId int, sats int) (glresp GoLightningResponse, err error) {
	bolt11, _, _, err := user.makeInvoice(sats, "refill from golightning.club", "", nil, messageId, "", true)
	if err != nil {
		return
	}

	resp, err := http.PostForm("https://api.golightning.club/new", url.Values{"bolt11": {bolt11}})
	if err != nil {
		return
	}
	defer resp.Body.Close()
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := ioutil.ReadAll(resp.Body)
		err = errors.New("GoLightning call failed (" + string(b) + ").")
		return
	}

	err = json.NewDecoder(resp.Body).Decode(&glresp)
	if err != nil {
		return
	}

	if glresp.Error != "" {
		err = errors.New(glresp.Error)
		return
	}

	return
}
