package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/go-lnurl"
	decodepay "github.com/fiatjaf/ln-decodepay"
	"github.com/fiatjaf/lntxbot/t"
	"github.com/gorilla/mux"
)

func handleCreateLNURLWithdraw(ctx context.Context, opts docopt.Opts) (enc string) {
	u := ctx.Value("initiator").(User)

	maxMSats, err := parseSatoshis(opts)
	if err != nil {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
		return
	}
	maxSats := maxMSats / 1000

	go u.track("lnurl generate", map[string]interface{}{"sats": maxSats})

	challenge := hashString("%s:%d:%d", s.TelegramBotToken, u.Id, maxSats)
	nexturl := fmt.Sprintf("%s/lnurl/withdraw?challenge=%s", s.ServiceURL, challenge)
	rds.Set("lnurlwithdraw:"+challenge,
		fmt.Sprintf(`%d-%d`, u.Id, maxSats), 30*time.Minute)

	enc, err = lnurl.LNURLEncode(nexturl)
	if err != nil {
		log.Error().Err(err).Msg("error encoding lnurl on withdraw")
		return
	}

	send(ctx, u, qrURL(enc), `<code>`+enc+"</code>")
	return
}

func serveLNURL() {
	ctx := context.WithValue(context.Background(), "origin", "external")

	router.Path("/lnurl/withdraw").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debug().Str("url", r.URL.String()).Msg("lnurl-withdraw first request")

		qs := r.URL.Query()
		challenge := qs.Get("challenge")
		if challenge == "" {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Malformed lnurl."))
			return
		}

		val, err := rds.Get("lnurlwithdraw:" + challenge).Result()
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Unknown lnurl."))
			return
		}

		// get user id and maxWithdrawable from redis value
		parts := strings.Split(val, "-")
		if len(parts) != 2 {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Internal mismatch."))
			return
		}
		chUserId, err1 := strconv.Atoi(parts[0])
		chMax, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Internal mismatch."))
			return
		}
		u, err := loadUser(chUserId)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Couldn't load withdrawee user."))
			return
		}

		json.NewEncoder(w).Encode(lnurl.LNURLWithdrawResponse{
			Callback:        fmt.Sprintf("%s/lnurl/withdraw/invoice", s.ServiceURL),
			K1:              challenge,
			MaxWithdrawable: 1000 * int64(chMax),
			MinWithdrawable: 1000 * int64(chMax),
			DefaultDescription: fmt.Sprintf(
				"%s lnurl withdraw from %s", u.AtName(ctx), s.ServiceId),
			Tag:           "withdrawRequest",
			LNURLResponse: lnurl.OkResponse(),
		})
	})

	router.Path("/lnurl/withdraw/invoice").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(context.Background(), "origin", "external")
		log.Debug().Str("url", r.URL.String()).Msg("lnurl second request")

		qs := r.URL.Query()
		challenge := qs.Get("k1")
		bolt11 := qs.Get("pr")

		val, err := rds.Get("lnurlwithdraw:" + challenge).Result()
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Unknown lnurl."))
			return
		}

		// get user id and maxWithdrawable from redis value
		parts := strings.Split(val, "-")
		if len(parts) != 2 {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Internal mismatch."))
			return
		}
		chUserId, err1 := strconv.Atoi(parts[0])
		chMax, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Internal mismatch."))
			return
		}
		payer, err := loadUser(chUserId)
		if err != nil {
			json.NewEncoder(w).Encode(
				lnurl.ErrorResponse("Couldn't load withdrawee user."))
			return
		}

		// double-check the challenge (it's a hash of the parameters + our secret)
		if challenge != hashString("%s:%d:%d", s.TelegramBotToken, payer.Id, chMax) {
			json.NewEncoder(w).Encode(
				lnurl.ErrorResponse("Invalid amount for this lnurl."))
			return
		}

		if err := rds.Del("lnurlwithdraw:" + challenge).Err(); err != nil {
			// if error stop here to prevent extra withdrawals
			log.Error().Err(err).Str("challenge", challenge).
				Msg("error deleting used challenge on lnurl withdraw")
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Redis error. Please report."))
			return
		}

		inv, err := decodepay.Decodepay(bolt11)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid payment request."))
			return
		}

		if inv.MSatoshi > int64(chMax)*1000 {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Amount too big."))
			return
		}

		// print the bolt11 just because
		send(ctx, payer, bolt11, ctx.Value("message"))

		go payer.track("outgoing lnurl-withdraw redeemed", map[string]interface{}{
			"sats": float64(inv.MSatoshi) / 1000,
		})

		// do the pay flow with these odd opts and fake message.
		opts := docopt.Opts{
			"pay":       true,
			"<invoice>": bolt11,
			"now":       true,
		}
		handlePay(ctx, payer, opts)
		json.NewEncoder(w).Encode(lnurl.OkResponse())
	})

	router.Path("/lnurl/pay").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debug().Str("url", r.URL.String()).Msg("lnurl-pay first request")

		qs := r.URL.Query()
		username := qs.Get("username")
		if username == "" {
			username = qs.Get("userid")
		}

		u, metadata, err := lnurlPayUserMetadata(ctx, username)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid username or id."))
			return
		}

		go u.track("incoming lnurl-pay attempt", nil)

		json.NewEncoder(w).Encode(lnurl.LNURLPayParams{
			LNURLResponse: lnurl.OkResponse(),
			Tag:           "payRequest",
			Callback: fmt.Sprintf("%s/.well-known/lnurlp/%s",
				s.ServiceURL, username),
			MaxSendable:    1000000000,
			MinSendable:    100000,
			Metadata:       metadata,
			CommentAllowed: 422,
		})
	})

	router.Path("/.well-known/lnurlp/{username}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(context.Background(), "origin", "external")
		username := mux.Vars(r)["username"]
		qs := r.URL.Query()

		receiver, metadata, err := lnurlPayUserMetadata(ctx, username)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid username or id."))
			return
		}

		if qs.Get("amount") == "" {
			log.Debug().Str("url", r.URL.String()).Msg("lnurl-pay first request")

			go receiver.track("incoming lnurl-pay attempt", nil)

			json.NewEncoder(w).Encode(lnurl.LNURLPayParams{
				LNURLResponse: lnurl.OkResponse(),
				Tag:           "payRequest",
				Callback: fmt.Sprintf("%s/.well-known/lnurlp/%s",
					s.ServiceURL, username),
				MaxSendable:    1000000000,
				MinSendable:    100000,
				Metadata:       metadata,
				CommentAllowed: 422,
				PayerData: lnurl.PayerDataSpec{
					FreeName:         &lnurl.PayerDataItemSpec{},
					LightningAddress: &lnurl.PayerDataItemSpec{},
					Email:            &lnurl.PayerDataItemSpec{},
				},
			})
		} else {
			log.Debug().Str("url", r.URL.String()).Msg("lnurl-pay second request")

			amount := qs.Get("amount")
			msatoshi, err := strconv.ParseInt(amount, 10, 64)
			if err != nil {
				json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid msatoshi amount."))
				return
			}

			var hhash [32]byte
			payerdata := qs.Get("payerdata")
			if payerdata == "" {
				hhash = metadata.Hash()
			} else {
				hhash = metadata.HashWithPayerData(payerdata)
			}

			var payerData lnurl.PayerDataValues
			json.Unmarshal([]byte(payerdata), &payerData)

			bolt11, _, err := receiver.makeInvoice(ctx, &MakeInvoiceArgs{
				IgnoreInvoiceSizeLimit: true,
				Msatoshi:               msatoshi,
				DescriptionHash:        hex.EncodeToString(hhash[:]),
				Extra: InvoiceExtra{
					Comment:   qs.Get("comment"),
					PayerData: payerData,
				},
			})
			if err != nil {
				log.Warn().Err(err).Msg("failed to generate lnurl-pay invoice")
				json.NewEncoder(w).Encode(
					lnurl.ErrorResponse("Failed to generate invoice."))
				return
			}

			json.NewEncoder(w).Encode(lnurl.LNURLPayValues{
				LNURLResponse: lnurl.OkResponse(),
				PR:            bolt11,
				Routes:        []struct{}{},
				Disposable:    lnurl.FALSE,
			})
		}
	})
}

func lnurlPayUserMetadata(
	ctx context.Context,
	username string,
) (receiver User, metadata lnurl.Metadata, err error) {
	isTelegramUsername := false

	if id, errx := strconv.Atoi(username); errx == nil {
		// case in which username is a number
		receiver, err = loadUser(id)
	} else {
		// case in which username is a real username
		receiver, err = ensureTelegramUsername(username)
		isTelegramUsername = true
	}
	if err != nil {
		return
	}

	metadata.Description = fmt.Sprintf("Fund %s account on t.me/%s.",
		receiver.AtName(ctx), s.ServiceId)

	if isTelegramUsername {
		// get user avatar from public t.me/ page
		if url, err := getTelegramUserPictureURL(username); err == nil {
			var err error
			metadata.Image.Bytes, err = imageBytesFromURL(url)
			if err == nil {
				metadata.Image.Ext = "jpeg"
			}
		}

		// add internet identifier
		metadata.LightningAddress = fmt.Sprintf("%s@%s",
			username, getHost())
	}

	return
}
