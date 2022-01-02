package main

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/lucsky/cuid"
	"github.com/skip2/go-qrcode"
	chqr "github.com/tuotoo/qrcode"
	"gopkg.in/jmcvetta/napping.v3"
)

func serveQRCodes() {
	router.PathPrefix("/qr/").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			value := r.URL.Path[4:]

			if strings.HasPrefix(value, "base64,") {
				d, err := base64.StdEncoding.DecodeString(value[7:])
				if err != nil {
					log.Warn().Err(err).Str("value", value).Msg("failed to decode b64")
					http.Error(w, "failed to decode base64 data for qr", 400)
					return
				}
				value = string(d)
			}

			if strings.HasPrefix(value, "lnurl1") || strings.HasPrefix(value, "lnbc") {
				value = strings.ToUpper(value)
			}

			qr, err := qrcode.New(value, qrcode.Low)
			if err != nil {
				log.Warn().Err(err).Str("value", value).Msg("failed to encode qr")
				http.Error(w, "failed to encode "+value+" as a QR code.", 400)
				return
			}

			w.Header().Set("Content-Type", "image/png")
			err = qr.Write(256, w)
			if err != nil {
				log.Warn().Err(err).Str("value", value).Msg("failed to write qr")
				http.Error(w, "failed to write "+value+" as a QR code.", 400)
				return
			}
		},
	)
}

func qrURL(value string) *url.URL {
	if strings.Index(value, "/") >= 0 {
		value = "base64," + base64.StdEncoding.EncodeToString([]byte(value))
	}
	u, _ := url.Parse(s.ServiceURL + "/qr/" + value)
	return u
}

func decodeQR(fileurl string) (data string, err error) {
	chineselibrary := make(chan string)
	qrserver := make(chan string)
	qrcodeonline := make(chan string)

	go func() {
		resp, err := http.Get(fileurl)
		if err != nil {
			log.Warn().Err(err).Str("method", "chineselibrary").Str("url", fileurl).Msg("failed to download")
			return
		}
		defer resp.Body.Close()

		path := "/tmp/qr/" + cuid.Slug()
		file, err := os.Create(path)
		if err != nil {
			log.Warn().Err(err).Str("method", "chineselibrary").Str("url", fileurl).Msg("failed to create file")
			return
		}

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			log.Warn().Err(err).Str("method", "chineselibrary").Str("url", fileurl).Msg("failed to save downloaded")
			file.Close()
			return
		}
		file.Close()

		file, err = os.Open(path)
		if err != nil {
			log.Warn().Err(err).Str("method", "chineselibrary").Str("url", fileurl).
				Msg("failed to open for reading")
			return
		}
		defer file.Close()

		qrmatrix, err := chqr.Decode(file)
		if err != nil {
			log.Warn().Err(err).Str("method", "chineselibrary").Str("url", fileurl).Msg("failed to decode")
			return
		}

		chineselibrary <- qrmatrix.Content
	}()

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
			log.Warn().Err(err).Str("method", "api.qrserver.com").Str("url", fileurl).Msg("failed to call")
			return
		}
		if len(r) == 0 || len(r[0].Symbol) == 0 {
			log.Warn().Str("method", "api.qrserver.com").Str("url", fileurl).Msg("invalid response")
			return
		}
		if r[0].Symbol[0].Error != "" {
			log.Debug().Str("err", r[0].Symbol[0].Error).
				Str("method", "api.qrserver.com").Str("url", fileurl).Msg("failed to decode")
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
			log.Warn().Err(err).Str("method", "qrcode.online").Str("url", fileurl).Msg("failed to call")
			return
		}
		if r.Text == "" {
			log.Warn().Str("method", "qrcode.online").Str("err", r.Error).Str("url", fileurl).Msg("error decoding")
			return
		}

		qrcodeonline <- r.Text
	}()

	select {
	case text := <-chineselibrary:
		return text, nil
	case text := <-qrserver:
		return text, nil
	case text := <-qrcodeonline:
		return text, nil
	case <-time.After(6 * time.Second):
		return "", errors.New("unable to decode.")
	}
}
