package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/go-lnurl"
	"github.com/gorilla/mux"
)

func handleLNCreateLNURLWithdraw(u User, sats int, messageId int) (lnurlEncoded string) {
	maxsats := strconv.Itoa(sats)
	ok := u.checkBalanceFor(sats, "lnurl-withdraw", nil)
	if !ok {
		return
	}

	challenge := calculateHash(s.TelegramBotToken + ":" + strconv.Itoa(messageId) + ":" + maxsats)

	nexturl := fmt.Sprintf("%s/lnurl/withdraw?message=%d&challenge=%s", s.ServiceURL, messageId, challenge)
	rds.Set("lnurlwithdraw:"+challenge, fmt.Sprintf(`%d-%s`, u.Id, maxsats), s.InvoiceTimeout)

	lnurlEncoded, err := lnurl.LNURLEncode(nexturl)
	if err != nil {
		log.Error().Err(err).Msg("error encoding lnurl on withdraw")
		return
	}

	u.sendMessageWithPicture(qrURL(lnurlEncoded), `<a href="lightning:`+lnurlEncoded+`">`+lnurlEncoded+"</a>")
	return
}

func serveLNURL() {
	router.Path("/lnurl/withdraw").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debug().Str("url", r.URL.String()).Msg("lnurl-withdraw first request")

		qs := r.URL.Query()
		challenge := qs.Get("challenge")
		messageIdstr := qs.Get("message")
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid user id."))
			return
		}

		challenge = qs.Get("challenge")
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
			Callback:           fmt.Sprintf("%s/lnurl/withdraw/invoice/%s", s.ServiceURL, messageIdstr),
			K1:                 challenge,
			MaxWithdrawable:    1000 * int64(chMax),
			MinWithdrawable:    1000 * int64(chMax),
			DefaultDescription: fmt.Sprintf("%s lnurl withdraw @%s", u.AtName(), s.ServiceId),
			Tag:                "withdrawRequest",
			LNURLResponse:      lnurl.OkResponse(),
		})
	})

	router.Path("/lnurl/withdraw/invoice/{messageId}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debug().Str("url", r.URL.String()).Msg("lnurl second request")

		messageIdstr := mux.Vars(r)["messageId"]
		messageId, _ := strconv.Atoi(messageIdstr)

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
		u, err := loadUser(chUserId)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Couldn't load withdrawee user."))
			return
		}

		// double-check the challenge (it's a hash of the parameters + our secret)
		if challenge != calculateHash(s.TelegramBotToken+":"+messageIdstr+":"+strconv.Itoa(chMax)) {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid amount for this lnurl."))
			return
		}

		if err := rds.Del("lnurlwithdraw:" + challenge).Err(); err != nil {
			// if error stop here to prevent extra withdrawals
			log.Error().Err(err).Str("challenge", challenge).Msg("error deleting used challenge on lnurl withdraw")
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Redis error. Please report."))
			return
		}

		inv, err := ln.Call("decodepay", bolt11)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid payment request."))
			return
		}

		if inv.Get("msatoshi").Int() > int64(chMax)*1000 {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Amount too big."))
			return
		}

		// print the bolt11 just because
		nextMessageId := sendTelegramMessageAsReply(u.TelegramChatId, bolt11, messageId).MessageID

		go u.track("outgoing lnurl-withdraw redeemed", map[string]interface{}{
			"sats": inv.Get("msatoshi").Float() / 1000,
		})

		// do the pay flow with these odd opts and fake message.
		opts := docopt.Opts{
			"pay":       true,
			"<invoice>": bolt11,
			"now":       true,
		}
		handlePay(u, opts, nextMessageId, nil)

		json.NewEncoder(w).Encode(lnurl.OkResponse())
	})

	router.Path("/lnurl/pay").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debug().Str("url", r.URL.String()).Msg("lnurl-pay first request")

		qs := r.URL.Query()
		userid := qs.Get("userid")
		username := qs.Get("username")

		u, jmeta, err := lnurlPayStuff(userid, username)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid username or id."))
			return
		}

		go u.track("incoming lnurl-pay attempt", nil)

		json.NewEncoder(w).Encode(lnurl.LNURLPayResponse1{
			LNURLResponse: lnurl.OkResponse(),
			Tag:           "payRequest",
			Callback: fmt.Sprintf("%s/lnurl/pay/callback?%s",
				s.ServiceURL, qs.Encode()),
			MaxSendable:     1000000000,
			MinSendable:     100000,
			EncodedMetadata: string(jmeta),
			CommentAllowed:  422,
		})
	})

	router.Path("/lnurl/pay/callback").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		qs := r.URL.Query()
		userid := qs.Get("userid")
		username := qs.Get("username")
		apptag := qs.Get("apptag")
		amount := qs.Get("amount")

		receiver, jmeta, err := lnurlPayStuff(userid, username)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid username or id."))
			return
		}

		var tag string
		if apptag == "golightning" {
			tag = apptag
		}

		msatoshi, err := strconv.ParseInt(amount, 10, 64)
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Invalid msatoshi amount."))
			return
		}

		hhash := sha256.Sum256(jmeta)
		bolt11, _, err := receiver.makeInvoice(makeInvoiceArgs{
			IgnoreInvoiceSizeLimit: true,
			Msatoshi:               msatoshi,
			DescHash:               hex.EncodeToString(hhash[:]),
			Tag:                    tag,
			Extra: map[string]interface{}{
				"comment": qs.Get("comment"),
			},
		})
		if err != nil {
			log.Warn().Err(err).Msg("failed to generate lnurl-pay invoice")
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Failed to generate invoice."))
			return
		}

		json.NewEncoder(w).Encode(lnurl.LNURLPayResponse2{
			LNURLResponse: lnurl.OkResponse(),
			PR:            bolt11,
			Routes:        make([][]lnurl.RouteInfo, 0),
			Disposable:    lnurl.FALSE,
		})
	})
}

func lnurlPayStuff(userid string, username string) (receiver User, jmeta []byte, err error) {
	if userid != "" {
		var id int
		id, err = strconv.Atoi(userid)
		if err == nil {
			receiver, err = loadUser(id)
		}
	} else if username != "" {
		receiver, err = ensureTelegramUsername(username)
	}
	if err != nil {
		return
	}

	metadata := [][]string{
		[]string{
			"text/plain",
			fmt.Sprintf("Fund %s account on t.me/%s.",
				receiver.AtName(), s.ServiceId),
		},
	}

	if username != "" { /* we may have only a userid */
		if imageURL, err := getUserPictureURL(username); err == nil {
			if b64, err := base64FileFromURL(imageURL); err == nil {
				metadata = append(metadata, []string{"image/jpeg;base64", b64})
			}
		}
	}

	jmeta, err = json.Marshal(metadata)
	return
}
