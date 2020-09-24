package main

import (
	"fmt"
	"net/http"

	lnurl "github.com/fiatjaf/go-lnurl"
)

func servePages() {
	// lnurl-pay powered donation page
	router.PathPrefix("/@").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := r.URL.Path[2:]
		image, err := getTelegramUserPictureURL(username)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		lnurl, err := lnurl.LNURLEncode(fmt.Sprintf(
			"%s/lnurl/pay?username=%s", s.ServiceURL, username))
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		if err = tmpl.ExecuteTemplate(w, "donation", struct {
			Username string
			Image    string
			LNURLPay string
		}{username, image, lnurl}); err != nil {
			log.Error().Err(err).Str("username", username).Msg("failed to render template")
		}
	})
}
