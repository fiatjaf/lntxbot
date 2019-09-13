package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/kr/pretty"
	"github.com/renstrom/fuzzysearch/fuzzy"
	"gopkg.in/jmcvetta/napping.v3"
)

type BitrefillData struct {
	Country    string   `json:"country"`
	PaidOrders []string `json:"orders"`
}

var BITREFILLCOUNTRIES = []string{"AE", "AF", "AG", "AI", "AL", "AM", "AN", "AO", "AR", "AS", "AT", "AU", "AW", "AZ", "BB", "BD", "BE", "BF", "BH", "BI", "BJ", "BM", "BO", "BR", "BS", "BT", "BW", "BY", "BZ", "CA", "CD", "CF", "CG", "CH", "CI", "CL", "CM", "CN", "CO", "CR", "CU", "CV", "CW", "CY", "CZ", "DE", "DK", "DM", "DO", "DZ", "EC", "EG", "ES", "ET", "EU", "FI", "FJ", "FR", "GB", "GD", "GE", "GF", "GH", "GM", "GN", "GP", "GR", "GT", "GW", "GY", "HN", "HT", "ID", "IE", "IN", "IQ", "IT", "JM", "JO", "JP", "KE", "KG", "KH", "KM", "KN", "KR", "KW", "KY", "KZ", "LA", "LB", "LC", "LK", "LR", "LT", "LU", "MA", "MC", "MD", "MG", "ML", "MM", "MN", "MQ", "MR", "MS", "MW", "MX", "MY", "MZ", "NA", "NE", "NG", "NI", "NL", "NO", "NP", "NR", "OM", "PA", "PE", "PG", "PH", "PK", "PL", "PR", "PS", "PT", "PY", "QA", "RO", "RU", "RW", "SA", "SD", "SE", "SG", "SL", "SN", "SO", "SR", "SV", "SY", "SZ", "TC", "TG", "TH", "TJ", "TN", "TO", "TR", "TT", "TZ", "UA", "UG", "US", "UY", "UZ", "VC", "VE", "VG", "VN", "VU", "WS", "XI", "XK", "YE", "ZA", "ZM", "ZW"}

var bitrefillInventory = make(map[string]BitrefillInventoryItem)
var bitrefillInventoryKeys []string
var bitrefillCountryInventoryKeys = make(map[string][]string)
var bitrefill napping.Session

type BitrefillCountryOperators struct {
	Alpha2              string                   `json:"alpha2"`
	Name                string                   `json:"name"`
	Slug                string                   `json:"slug"`
	CountryCallingCodes []string                 `json:"countryCallingCodes"`
	Operators           []BitrefillInventoryItem `json:"operators"`
}

type BitrefillInventoryItem struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	CountryCode string `json:"countryCode"`
	Type        string `json:"type"`
	Stats       struct {
		Popularity  float64 `json:"popularity"`
		PackageSize float64 `json:"packageSize"`
	} `json:"stats"`
	RecipientType string             `json:"recipientType"`
	IsPinBased    bool               `json:"isPinBased"`
	IsRanged      bool               `json:"isRanged"`
	Currency      string             `json:"currency"`
	Packages      []BitrefillPackage `json:"packages"`
	Range         interface{}        `json:"range"`
	LogoImage     string             `json:"logoImage"`
	ExtraInfo     string             `json:"extraInfo"`
	Promotions    struct {
		Current  interface{} `json:"current"`
		Upcoming interface{} `json:"upcoming"`
	} `json:"promotions"`
}

type BitrefillPackage struct {
	Value        interface{} `json:"value"`
	EurPrice     float64     `json:"eurPrice"`
	SatoshiPrice int         `json:"satoshiPrice"`
	UsdPrice     float64     `json:"usdPrice"`
	UserPrice    int         `json:"userPrice"`
}

type BitrefillErrorResponse struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

func initializeBitrefill() {
	bitrefill = napping.Session{
		Header: &http.Header{
			"Authorization": {"Basic " + s.BitrefillBasicAuth},
		},
	}

	for _, countryCode := range BITREFILLCOUNTRIES {
		var countryresp BitrefillCountryOperators
		var brerr interface{}
		resp, err := bitrefill.Get("https://api.bitrefill.com/v1/inventory/"+countryCode, nil, &countryresp, &brerr)
		if err != nil {
			log.Warn().Str("country", countryCode).Err(err).Msg("error fetching bitrefill country inventory")
			return
		}
		if resp.Status() >= 300 {
			log.Warn().Str("country", countryCode).Interface("error-response", brerr).
				Msg("error fetching bitrefill country inventory")

			return
		}

		countryKeys := make([]string, len(countryresp.Operators))
		for i, item := range countryresp.Operators {
			bitrefillInventory[item.Slug] = item
			bitrefillInventoryKeys = append(bitrefillInventoryKeys, item.Slug)
			countryKeys[i] = item.Slug
		}
		bitrefillCountryInventoryKeys[countryCode] = countryKeys
	}
}

func serveBitrefillWebhook() {
	http.HandleFunc("/app/bitrefill/webhook", func(w http.ResponseWriter, r *http.Request) {
		b, _ := ioutil.ReadAll(r.Body)
		log.Print(string(b))
	})
}

func queryBitrefillInventory(query string, countryCode *string) []BitrefillInventoryItem {
	haystack := bitrefillInventoryKeys
	if countryCode != nil {
		haystack = bitrefillCountryInventoryKeys[*countryCode]
	}

	keys := fuzzy.FindFold(query, haystack)

	results := make([]BitrefillInventoryItem, 0, len(keys))
	for i, key := range keys {
		score := fuzzy.LevenshteinDistance(key, query)

		if score > 10 {
			log.Debug().Str("query", query).Str("key", key).Int("score", score).
				Msg("bitrefill item bad score, breaking")
			break
		}

		if i > 16 {
			break
		}

		results = append(results, bitrefillInventory[key])
	}

	return results
}

func handleBitrefillItem(user User, item BitrefillInventoryItem) {
	npacks := len(item.Packages)
	inlinekeyboard := make([][]tgbotapi.InlineKeyboardButton, npacks/2+npacks%2)
	for i, pack := range item.Packages {
		if i%2 == 0 {
			inlinekeyboard[i/2] = make([]tgbotapi.InlineKeyboardButton, 0, 2)
		}

		inlinekeyboard[i/2] = append(inlinekeyboard[i/2], tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%v %s (%d sat)", pack.Value, item.Currency, pack.SatoshiPrice),
			fmt.Sprintf("app=bitrefill-place-%s-%d", strings.Replace(item.Slug, "-", "~", -1), i),
		))
	}

	user.notifyWithKeyboard(t.BITREFILLPACKAGESHEADER, t.T{
		"Item": item.Name,
	}, &tgbotapi.InlineKeyboardMarkup{inlinekeyboard}, 0)
}

func placeBitrefillOrder(
	user User,
	item BitrefillInventoryItem,
	pack BitrefillPackage,
	number *string,
) (orderId string, bolt11 string, err error) {
	var resporder struct {
		OrderId        string `json:"orderId"`
		ExpirationTime int64  `json:"expirationTime"`
		InvoiceTime    int64  `json:"invoiceTime"`
		ItemDesc       string `json:"itemDesc"`
		Payment        struct {
			LightningInvoice string `json:"lightningInvoice"`
		} `json:"payment"`
	}
	var resperr BitrefillErrorResponse
	resp, err := bitrefill.Post("https://api.bitrefill.com/v1/order/", struct {
		OperatorSlug  string      `json:"operatorSlug"`
		ValuePackage  interface{} `json:"valuePackage"`
		Number        *string     `json:"number"`
		Email         string      `json:"email"`
		PaymentMethod string      `json:"paymentMethod"`
		WebhookURL    string      `json:"webhook_url"`
		UserRef       string      `json:"userRef"`
	}{
		OperatorSlug:  item.Slug,
		ValuePackage:  pack.Value,
		Number:        number,
		Email:         "bitrefill-lntxbot@alhur.es",
		PaymentMethod: "lightning",
		WebhookURL:    s.ServiceURL + "/app/bitrefill/webhook",
		UserRef:       fmt.Sprintf("%d-%s", user.Id, time.Now().Format("Mon Jan 2 2006-01-02-15:04")),
	}, &resporder, &resperr)
	if err != nil {
		log.Error().Err(err).Msg("error placing bitrefill order")
		return
	}
	if resp.Status() >= 300 {
		log.Error().Str("err-resp", resperr.Message).Str("status", resperr.Status).Msg("error placing bitrefill order")
		err = errors.New(resperr.Message)
		return
	}

	pretty.Log(resporder)

	// save invoice to redis
	orderExpiration := time.Unix(0, resporder.ExpirationTime)
	rds.Set("bitrefillorder:"+resporder.OrderId, resporder.Payment.LightningInvoice, time.Until(orderExpiration))

	return resporder.OrderId, resporder.Payment.LightningInvoice, nil
}

func purchaseBitrefillOrder(user User, orderId string) error {
	// get invoice from redis
	bolt11, err := rds.Get("bitrefillorder:" + orderId).Result()
	if err != nil {
		return err
	}

	// pay invoice
	inv, err := ln.Call("decodepay", bolt11)
	if err != nil {
		return errors.New("Failed to decode invoice.")
	}
	err = user.actuallySendExternalPayment(
		0, bolt11, inv, inv.Get("msatoshi").Int(),
		fmt.Sprintf("%s.bitrefill.%s.%d", s.ServiceId, orderId, user.Id), map[string]interface{}{},
		func(
			u User,
			messageId int,
			msatoshi float64,
			msatoshi_sent float64,
			preimage string,
			tag string,
			hash string,
		) {
			// on success
			paymentHasSucceeded(u, messageId, msatoshi, msatoshi_sent, preimage, "bitrefill", hash)

			// save to user bitrefill data
			var data BitrefillData
			err := user.getAppData("bitrefill", &data)
			if err != nil {
				u.notify(t.BITREFILLFAILEDSAVE, t.T{"OrderId": orderId, "Err": err.Error()})
				return
			}
			data.PaidOrders = append(data.PaidOrders, orderId)

			// limit stored orders to 50
			if len(data.PaidOrders) > 50 {
				data.PaidOrders = data.PaidOrders[len(data.PaidOrders)-50:]
			}

			err = user.setAppData("bitrefill", data)
			if err != nil {
				u.notify(t.BITREFILLFAILEDSAVE, t.T{"OrderId": orderId, "Err": err.Error()})
				return
			}

			// acknowledge purchase
			var resperr BitrefillErrorResponse
			resp, err := napping.Post("https://api.bitrefill.com/v1/order/"+orderId+"/purchase", struct {
				WebhookURL string `json:"webhook_url"`
			}{s.ServiceURL + "/app/bitrefill/webhook"}, nil, &resperr)

			if err != nil {
				log.Error().Err(err).Msg("error acknowledging bitrefill order")
				return
			}
			if resp.Status() >= 300 {
				log.Error().Str("err-resp", resperr.Message).Str("status", resperr.Status).
					Msg("error acknowledging bitrefill order")
				return
			}

			// start polling the order
			go pollBitrefillOrder(user, orderId, 5)
		},
		paymentHasFailed,
	)

	return err
}

func pollBitrefillOrder(user User, orderId string, countdown int) {
	var orderinfo struct {
		PaymentReceived bool  `json:"paymentReceived"`
		Delivered       bool  `json:"delivered"`
		Value           int   `json:"value"`
		Number          int64 `json:"number"`
		PinInfo         *struct {
			Instructions string `json:"instructions"`
			Pin          string `json:"pin"`
			Other        string `json:"other"`
		} `json:"pinInfo"`
		LinkInfo *struct {
			Link  string `json:"link"`
			Other string `json:"other"`
		} `json:"linkInfo"`
		ErrorType    string `json:"errorType"`
		ErrorMessage string `json:"errorMessage"`
	}
	var resperr BitrefillErrorResponse
	resp, err := bitrefill.Get("https://api.bitrefill.com/v1/order/"+orderId, nil, &orderinfo, &resperr)
	if err != nil {
		log.Error().Err(err).Msg("error polling bitrefill order")
		return
	}
	if resp.Status() >= 300 {
		log.Error().Str("err-resp", resperr.Message).Str("status", resperr.Status).
			Msg("error polling bitrefill order")
		return
	}

	pretty.Log(orderinfo)

	// got a valid response
	if orderinfo.ErrorType != "" {
		// but it can still contain an error
		log.Warn().Str("type", orderinfo.ErrorType).Str("id", orderId).Str("message", orderinfo.ErrorMessage).
			Msg("bitrefill purchase failed")
		user.notify(t.BITREFILLPURCHASEFAILED, t.T{"ErrorMessage": orderinfo.ErrorMessage})
		return
	} else if orderinfo.Delivered {
		// no, it's a success!
		user.notify(t.BITREFILLPURCHASEDONE, t.T{"OrderId": orderId, "Info": orderinfo})
		return
	} else if orderinfo.PaymentReceived == false {
		// should never happen
		log.Error().Str("id", orderId).Msg("polling unpaid bitrefill order, this shouldn't happen")
		return
	}

	if countdown > 0 {
		pollBitrefillOrder(user, orderId, countdown-1)
	}
}
