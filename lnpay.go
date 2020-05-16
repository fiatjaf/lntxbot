package main

import (
	"errors"
	"fmt"
	"net/http"

	"gopkg.in/jmcvetta/napping.v3"
)

type PaywallUserData struct {
	WalletId  string `json:"wal"`
	WalletKey string `json:"waka"`
}

var paywallClient = napping.Session{
	Client: http.DefaultClient,
	Header: &http.Header{
		"X-Api-Key":    {s.LNPayKey},
		"Content-Type": {"application/json"},
		"Accept":       {"application/json"},
	},
}

func createPaywallLink(user User, sats int, url, memo string) (link string, err error) {
	var userdata PaywallUserData
	user.getAppData("paywall", &userdata)

	if userdata.WalletKey == "" {
		// create wallet now
		id, key, err := createPaywallWallet(user)
		if err != nil {
			return "", err
		}
		userdata = PaywallUserData{id, key}

		err = user.setAppData("paywall", userdata)
		if err != nil {
			return "", fmt.Errorf("failed to save lnpay.co wallet credentials: %w", err)
		}
	}

	var result struct {
		Link string `json:"paywall_link"`
	}
	resp, err := paywallClient.Post("https://lnpay.co/v1/paywall", struct {
		Destination string `json:"destination_url"`
		Memo        string `json:"memo"`
		NumSatoshis int    `json:"num_satoshis"`
	}{url, memo, sats}, &result, nil)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		return "", errors.New("failed to create paywall")
	}

	return result.Link, nil
}

func getPaywallBalance(user User) (sats int, err error) {
	var userdata PaywallUserData
	user.getAppData("paywall", &userdata)

	if userdata.WalletKey == "" {
		return 0, nil
	}

	var result struct {
		Balance int `json:"balance"`
	}
	resp, err := paywallClient.Get("https://lnpay.com/v1/wallet/"+userdata.WalletKey,
		nil, &result, nil)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		return 0, errors.New("failed to query balance")
	}

	return result.Balance, nil
}

func getPaywallWithdrawLNURL(user User) (string, error) {
	var userdata PaywallUserData
	user.getAppData("paywall", &userdata)

	if userdata.WalletKey == "" {
		return "", errors.New("no balance")
	}

	return fmt.Sprintf("https://lnpay.co/v1/wallet/%s/lnurl-process?ott=ui-w",
		userdata.WalletKey), nil
}

func createPaywallWallet(user User) (id string, key string, err error) {
	var result struct {
		Id         string `json:"id"`
		AccessKeys struct {
			Admin string `json:"Wallet Admin"`
		} `json:"access_keys"`
	}
	resp, err := paywallClient.Post("https://lnpay.com/v1/wallet", struct {
		Label string `json:"user_label"`
	}{fmt.Sprintf("@lntxbot account: %d", user.Id)}, &result, nil)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		return "", "", errors.New("failed to create lnpay.co wallet")
	}

	return result.Id, result.AccessKeys.Admin, nil
}
