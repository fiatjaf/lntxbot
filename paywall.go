package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"gopkg.in/jmcvetta/napping.v3"
)

type PaywallData struct {
	AuthKey string `json:"auth_key"`
}

type PaywallUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Balance  int    `json:"balance,omitempty"`
	AuthKey  string `json:"auth_key,omitempty"`
}

type PaywallWithdraw struct {
	Id             int64  `json:"id,omitempty"`
	InvoiceRequest string `json:"invoice_request"`
}

type PaywallLink struct {
	Id             int64  `json:"id,omitempty"`
	CreatedAt      int64  `json:"created_at,omitempty"`
	DestinationURL string `json:"destination_url"`
	LndValue       int    `json:"lnd_value"`
	Expires        int64  `json:"expires,omitempty"`
	ShortURL       string `json:"short_url,omitempty"`
	Memo           string `json:"memo"`
	Settled        int    `json:"settled"`
}

type PaywallWebhookEvent struct {
	AuthKey   string `json:"auth_key"`
	PaywallId int    `json:"paywall_id"`
	Link      string `json:"paywall_link"`
	LndValue  string `json:"lnd_value"`
	PaidAt    int    `json:"paid_at"`
	Settled   int    `json:"settled"`
}

type PaywallError struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Code    int    `json:"code"`
	Status  int    `json:"status"`
	Type    string `json:"type"`
}

func (perr PaywallError) Error() string {
	return "paywall.link error " + perr.Type + " (" + strconv.Itoa(perr.Code) + "): " +
		perr.Name + ", " + perr.Message
}

var paywallClient = napping.Session{
	Client: http.DefaultClient,
	Header: &http.Header{
		"Content-Type": {"application/json"},
		"Accept":       {"application/json"},
	},
}

func getPaywallKey(user User) (key string, err error) {
	// check user exists on paywall.link
	var data PaywallData
	err = user.getAppData("paywall", &data)
	if err != nil {
		return
	}
	if data.AuthKey != "" {
		return data.AuthKey, nil
	}

	// doesn't exist, let's create
	puser := PaywallUser{
		Username: fmt.Sprintf("%s__%d", s.ServiceId, user.Id),
		Password: calculateHash(fmt.Sprintf("%d~%s", user.Id, s.BotToken)),
	}
	var perr PaywallError
	resp, err := paywallClient.Post("https://paywall.link/v1/user?access-token="+s.PaywallLinkKey, puser, &puser, &perr)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = perr
		return
	}

	// created, let's save the auth_key
	data.AuthKey = puser.AuthKey
	err = user.setAppData("paywall", data)
	return data.AuthKey, err
}

func getPaywallBalance(user User) (sats int, err error) {
	key, err := getPaywallKey(user)
	if err != nil {
		return
	}

	var perr PaywallError
	var puser PaywallUser
	resp, err := paywallClient.Get("https://paywall.link/v1/user?access-token="+key, nil, &puser, &perr)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = perr
		return
	}

	return puser.Balance, nil
}

func listPaywallLinks(user User) (links []PaywallLink, err error) {
	key, err := getPaywallKey(user)
	if err != nil {
		return
	}

	var perr PaywallError
	resp, err := paywallClient.Get("https://paywall.link/v1/user/paywalls?access-token="+key, nil, &links, &perr)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = perr
		return
	}

	return links, nil
}

func withdrawPaywall(user User) (err error) {
	balance, err := getPaywallBalance(user)
	if err != nil {
		return
	}

	if balance <= 0 {
		return
	}

	key, err := getPaywallKey(user)
	if err != nil {
		return
	}

	bolt11, _, _, err := user.makeInvoice(balance, "withdraw from paywall.link", "", nil, nil, "paywall", true)
	if err != nil {
		return
	}

	withdrawal := PaywallWithdraw{InvoiceRequest: bolt11}
	var perr PaywallError
	resp, err := paywallClient.Post("https://paywall.link/v1/user/send?access-token="+key, withdrawal, &withdrawal, &perr)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = perr
		return
	}

	return nil
}

func createPaywallLink(user User, sats int, url, memo string) (link PaywallLink, err error) {
	key, err := getPaywallKey(user)
	if err != nil {
		return
	}

	link = PaywallLink{
		DestinationURL: url,
		Memo:           memo,
		LndValue:       sats,
	}
	var perr PaywallError
	resp, err := paywallClient.Post("https://paywall.link/v1/user/paywall?access-token="+key, link, &link, &perr)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = perr
		return
	}

	return
}

func servePaywallWebhook() {
	router.Path("/app/paywall/webhook").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// parse the incoming data
		var event PaywallWebhookEvent
		err := json.NewDecoder(r.Body).Decode(&event)
		if err != nil {
			log.Warn().Err(err).Msg("error decoding paywall webhook")
			return
		}

		// get our user that matches the incoming authkey
		var puser PaywallUser
		var perr PaywallError
		resp, err := paywallClient.Get("https://paywall.link/v1/user?access-token="+event.AuthKey,
			nil, &puser, &perr)
		if err != nil {
			log.Warn().Err(err).Interface("ev", event).Msg("error getting user data after paywall webhook")
			return
		}
		if resp.Status() >= 300 {
			err = perr
			log.Warn().Err(err).Interface("ev", event).Msg("error getting user data after paywall webhook")
			return
		}

		parts := strings.Split(puser.Username, "__")
		if len(parts) != 2 {
			log.Warn().Err(err).Interface("puser", puser).Msg("invalid user data after paywall webhook")
			return
		}

		userId, err := strconv.Atoi(parts[1])
		if err != nil {
			log.Warn().Err(err).Interface("puser", puser).
				Msg("failed to parse user id from user data on paywall webhook")
			return
		}

		user, err := loadUser(userId, 0)
		if err != nil {
			log.Warn().Err(err).Int("id", userId).Msg("failed to load user on paywall webhook")
			return
		}

		// now grab data about the paywall which was paid
		var link PaywallLink
		resp, err = paywallClient.Get(
			"https://paywall.link/v1/user/paywall/"+strconv.Itoa(event.PaywallId)+"?access-token="+event.AuthKey,
			nil, &link, &perr)
		if err != nil {
			log.Warn().Err(err).Interface("ev", event).Msg("error getting paywall link data after paywall webhook")
			return
		}
		if resp.Status() >= 300 {
			err = perr
			log.Warn().Err(err).Interface("ev", event).Msg("error getting paywall link data after paywall webhook")
			return
		}

		// finally we can notify the user who got the payment
		user.notify(t.PAYWALLPAIDEVENT, t.T{
			"Link":        event.Link,
			"Sats":        event.LndValue,
			"Times":       event.Settled,
			"Destination": link.DestinationURL,
			"Memo":        link.Memo,
		})
	})
}
