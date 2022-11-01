package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

var CURRENCIES = []string{
	"AED",
	"AFN",
	"ALL",
	"AMD",
	"ANG",
	"AOA",
	"ARS",
	"ARSBLUE",
	"AUD",
	"AWG",
	"AZN",
	"BAM",
	"BBD",
	"BDT",
	"BGN",
	"BHD",
	"BIF",
	"BMD",
	"BND",
	"BOB",
	"BRL",
	"BSD",
	"BTN",
	"BWP",
	"BYN",
	"BYR",
	"BZD",
	"CAD",
	"CDF",
	"CHF",
	"CLF",
	"CLP",
	"CNH",
	"CNY",
	"COP",
	"CRC",
	"CUC",
	"CVE",
	"CZK",
	"DJF",
	"DKK",
	"DOP",
	"DZD",
	"EGP",
	"ERN",
	"ETB",
	"EUR",
	"FJD",
	"FKP",
	"GBP",
	"GEL",
	"GGP",
	"GHS",
	"GIP",
	"GMD",
	"GNF",
	"GTQ",
	"GYD",
	"HKD",
	"HNL",
	"HRK",
	"HTG",
	"HUF",
	"IDR",
	"ILS",
	"IMP",
	"INR",
	"IQD",
	"ISK",
	"JEP",
	"JMD",
	"JOD",
	"JPY",
	"KES",
	"KGS",
	"KHR",
	"KMF",
	"KRW",
	"KWD",
	"KYD",
	"KZT",
	"LAK",
	"LBP",
	"LKR",
	"LRD",
	"LSL",
	"LYD",
	"MAD",
	"MDL",
	"MGA",
	"MKD",
	"MMK",
	"MNT",
	"MOP",
	"MRO",
	"MUR",
	"MVR",
	"MWK",
	"MXN",
	"MYR",
	"MZN",
	"NAD",
	"NGN",
	"NIO",
	"NOK",
	"NPR",
	"NZD",
	"OMR",
	"PAB",
	"PEN",
	"PGK",
	"PHP",
	"PKR",
	"PLN",
	"PYG",
	"QAR",
	"RON",
	"RSD",
	"RUB",
	"RWF",
	"SAR",
	"SBD",
	"SCR",
	"SEK",
	"SGD",
	"SHP",
	"SLL",
	"SOS",
	"SRD",
	"SSP",
	"STD",
	"SVC",
	"SZL",
	"THB",
	"TJS",
	"TMT",
	"TND",
	"TOP",
	"TRY",
	"TTD",
	"TWD",
	"TZS",
	"UAH",
	"UGX",
	"USD",
	"UYU",
	"UZS",
	"VEF",
	"VES",
	"VND",
	"VUV",
	"WST",
	"XAF",
	"XAG",
	"XAU",
	"XCD",
	"XDR",
	"XOF",
	"XPD",
	"XPF",
	"XPT",
	"YER",
	"ZAR",
	"ZMW",
	"ZWL",
}

func getMsatsPerFiatUnit(currencyCode string) (int64, error) {
	lower := strings.ToLower(currencyCode)
	upper := strings.ToUpper(currencyCode)

	ctx, cancel := context.WithCancel(context.Background())

	defer func() {
		cancel()
	}()

	modifier := 1.0
	// when fetching the ARSBLUE rate we get the USD rate then multiply it for the Dollar Blue rate
	if upper == "ARSBLUE" {
		var err error
		modifier, err = doGetPrice(ctx, "https://api.bluelytics.com.ar/v2/evolution.json?days=2", "1.value_buy")
		if err != nil {
			return 0, fmt.Errorf("failed to fetch blue price: %w", err)
		}
		upper = "USD"
		lower = "usd"
	}

	bitfinex := getPrice(ctx, "https://api.bitfinex.com/v1/pubticker/btc"+lower, "last_price")
	bitstamp := getPrice(ctx, "https://www.bitstamp.net/api/v2/ticker/btc"+lower, "last")
	coinbase := getPrice(ctx, "https://api.coinbase.com/v2/exchange-rates?currency=BTC", "data.rates."+upper)
	coinmate := getPrice(ctx, "https://coinmate.io/api/ticker?currencyPair=BTC_"+upper, "data.last")
	kraken := getPrice(ctx, "https://api.kraken.com/0/public/Ticker?pair=XBT"+upper, "result.XXBTZ"+upper+".c.0")

	var fiatPerBTC float64

	select {
	case fiatPerBTC = <-bitfinex:
	case fiatPerBTC = <-bitstamp:
	case fiatPerBTC = <-coinbase:
	case fiatPerBTC = <-coinmate:
	case fiatPerBTC = <-kraken:
	case <-time.After(time.Second * 3):
		return 0, errors.New("couldn't get BTC price for " + currencyCode)
	}

	// modify this with anything we had set previously
	fiatPerBTC = modifier * fiatPerBTC

	msatPerFiat := 100000000000 / fiatPerBTC
	return int64(msatPerFiat), nil
}

func getPrice(ctx context.Context, url string, pattern string) <-chan float64 {
	result := make(chan float64)
	go func() {
		price, err := doGetPrice(ctx, url, pattern)
		if err != nil {
			return
		}
		result <- price
	}()
	return result
}

func doGetPrice(ctx context.Context, url string, pattern string) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("status code %d", resp.StatusCode)
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	fiatPerBTC := gjson.GetBytes(data, pattern).Float()
	if fiatPerBTC <= 0 {
		trimmed := data
		if len(data) > 200 {
			trimmed = data[:200]
		}
		return 0, fmt.Errorf("couldn't find '%s' in '%s'", pattern, trimmed)
	}

	return fiatPerBTC, nil
}
