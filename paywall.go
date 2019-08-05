package main

import (
	"fmt"

	"gopkg.in/jmcvetta/napping.v3"
)

type PaywallData struct {
	AuthKey string `json:"auth_key"`
}

type PaywallUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Balance  int    `json:"balance"`
	AuthKey  string `json:"auth_key"`
}

type PaywallWithdraw struct {
	Id             int64  `json:"id"`
	PaymentRequest string `json:"payment_request"`
}

type PaywallLink struct {
	Id             int64  `json:"id"`
	CreatedAt      int64  `json:"created_at"`
	DestinationURL string `json:"destination_url"`
	LndValue       int    `json:"lnd_value"`
	Expires        int64  `json:"expires"`
	ShortURL       string `json:"short_url"`
	Memo           string `json:"memo"`
}

type PaywallError struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Code    int    `json:"code"`
	Status  int    `json:"status"`
	Type    string `json:"type"`
}

func (perr PaywallError) Error() string {
	return "paywall.link error: " + perr.Name + ", " + perr.Message
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
	resp, err := napping.Post("https://paywall.link/v1/user?access-token="+s.PaywallLinkKey, user, &user, &perr)
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
	return
}

func getPaywallBalance(user User) (sats int, err error) {
	key, err := getPaywallKey(user)
	if err != nil {
		return
	}

	var perr PaywallError
	var puser PaywallUser
	resp, err := napping.Get("https://paywall.link/v1/user?access-token="+key, nil, &user, &perr)
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
	resp, err := napping.Get("https://paywall.link/v1/user/paywalls?access-token="+key, nil, &links, &perr)
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

	bolt11, _, _, err := user.makeInvoice(balance, "withdraw from paywall.link", "", nil, nil, "", true)
	if err != nil {
		return
	}

	withdrawal := PaywallWithdraw{PaymentRequest: bolt11}
	var perr PaywallError
	resp, err := napping.Post("https://paywall.link/v1/user/send?access-token="+key, withdrawal, &withdrawal, &perr)
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
	resp, err := napping.Post("https://paywall.link/v1/user/paywalls?access-token="+key, link, &link, &perr)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = perr
		return
	}

	return
}
