package main

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"gopkg.in/jmcvetta/napping.v3"
)

func decodeQR(fileurl string) (data string, err error) {
	qrserver := make(chan string)
	qrcodeonline := make(chan string)

	go func() {
		var r []struct {
			Type   string `json:"type"`
			Symbol []struct {
				Data  string `json:"data"`
				Error string `json:"error"`
			} `json:"symbol"`
		}
		_, err = napping.Get("https://api.qrserver.com/v1/read-qr-code/", &url.Values{"fileurl": {fileurl}}, &r, nil)
		if err != nil {
			log.Warn().Err(err).Str("url", fileurl).Msg("failed to call api.qrserver.com")
			return
		}
		if len(r) == 0 || len(r[0].Symbol) == 0 {
			log.Warn().Str("url", fileurl).Msg("invalid response from api.qrserver.com")
			return
		}
		if r[0].Symbol[0].Error != "" {
			log.Debug().Str("err", r[0].Symbol[0].Error).
				Str("url", fileurl).Msg("api.qrserver.com failed to decode")
			return
		}

		text := r[0].Symbol[0].Data
		qrserver <- text
	}()

	go func() {
		var r struct {
			Text  string `json:"text"`
			Error string `json:"error"`
		}
		_, err := napping.Send(&napping.Request{
			Url:    "https://qrcode.online/ajax",
			Method: "GET",
			Params: &url.Values{"url": {fileurl}, "action": {"readurl"}},
			Header: &http.Header{"X-Requested-With": {"XMLHttpRequest"}},
			Result: &r,
		})

		if err != nil {
			log.Warn().Err(err).Str("url", fileurl).Msg("failed to call qrcode.online")
			return
		}
		if r.Text == "" {
			log.Warn().Str("err", r.Error).Str("url", fileurl).Msg("error from qrcode.online")
			return
		}

		qrcodeonline <- r.Text
	}()

	select {
	case text := <-qrserver:
		return text, nil
	case text := <-qrcodeonline:
		return text, nil
	case <-time.After(4 * time.Second):
		return "", errors.New("unable to decode.")
	}
}
