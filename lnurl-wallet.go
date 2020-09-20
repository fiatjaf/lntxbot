package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/fiatjaf/go-lnurl"
	decodepay "github.com/fiatjaf/ln-decodepay"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/tidwall/gjson"
	"gopkg.in/jmcvetta/napping.v3"
)

type handleLNURLOpts struct {
	messageId          int
	loginSilently      bool
	payWithoutPromptIf *int64
}

func handleLNURL(u User, lnurltext string, opts handleLNURLOpts) {
	iparams, err := lnurl.HandleLNURL(lnurltext)
	if err != nil {
		if lnurlerr, ok := err.(lnurl.LNURLErrorResponse); ok {
			u.notify(t.LNURLERROR, t.T{
				"Host":   lnurlerr.URL.Host,
				"Reason": lnurlerr.Reason,
			})
		} else {
			u.notify(t.ERROR, t.T{
				"Err": fmt.Sprintf("failed to fetch lnurl params: %s", err.Error()),
			})
		}
		return
	}

	log.Debug().Interface("params", iparams).Msg("got lnurl params")

	switch params := iparams.(type) {
	case lnurl.LNURLAuthParams:
		handleLNURLAuth(u, opts, params)
	case lnurl.LNURLWithdrawResponse:
		handleLNURLWithdraw(u, opts, params)
	case lnurl.LNURLPayResponse1:
		handleLNURLPay(u, opts, params)
	case lnurl.LNURLAllowanceResponse:
		handleLNURLAllowance(u, opts, params)
	default:
		u.notifyAsReply(t.LNURLUNSUPPORTED, nil, opts.messageId)
	}

	return
}

func handleLNURLAuth(u User, opts handleLNURLOpts, params lnurl.LNURLAuthParams) {
	// lnurl-auth: create a key based on the user id and sign with it
	seedhash := sha256.Sum256([]byte(fmt.Sprintf("lnurlkeyseed:%s:%d:%s", params.Host, u.Id, s.TelegramBotToken)))
	sk, pk := btcec.PrivKeyFromBytes(btcec.S256(), seedhash[:])
	k1, err := hex.DecodeString(params.K1)
	if err != nil {
		u.notify(t.ERROR, t.T{"Err": err.Error()})
		return
	}
	sig, err := sk.Sign(k1)
	if err != nil {
		u.notify(t.ERROR, t.T{"Err": err.Error()})
		return
	}

	signature := hex.EncodeToString(sig.Serialize())
	pubkey := hex.EncodeToString(pk.SerializeCompressed())

	var sentsigres lnurl.LNURLResponse
	_, err = napping.Get(params.Callback, &url.Values{
		"sig": {signature},
		"key": {pubkey},
	}, &sentsigres, &sentsigres)
	if err != nil {
		u.notify(t.ERROR, t.T{"Err": err.Error()})
		return
	}
	if sentsigres.Status == "ERROR" {
		u.notify(t.LNURLERROR, t.T{
			"Host":   params.Host,
			"Reason": sentsigres.Reason,
		})
		return
	}

	if !opts.loginSilently {
		u.notify(t.LNURLAUTHSUCCESS, t.T{
			"Host":      params.Host,
			"PublicKey": pubkey,
		})

		go u.track("lnurl-auth", map[string]interface{}{"domain": params.Host})
	}
}

func handleLNURLWithdraw(u User, opts handleLNURLOpts, params lnurl.LNURLWithdrawResponse) {
	// lnurl-withdraw: make an invoice with the highest possible value and send
	bolt11, _, _, err := u.makeInvoice(makeInvoiceArgs{
		IgnoreInvoiceSizeLimit: false,
		Msatoshi:               params.MaxWithdrawable,
		Desc:                   params.DefaultDescription,
		MessageId:              opts.messageId,
		SkipQR:                 true,
	})
	if err != nil {
		u.notify(t.ERROR, t.T{"Err": err.Error()})
		return
	}
	log.Debug().Str("bolt11", bolt11).Str("k1", params.K1).Msg("sending invoice to lnurl callback")
	var sentinvres lnurl.LNURLResponse
	_, err = napping.Get(params.Callback, &url.Values{
		"k1": {params.K1},
		"pr": {bolt11},
	}, &sentinvres, &sentinvres)
	if err != nil {
		u.notify(t.ERROR, t.T{"Err": err.Error()})
		return
	}
	if sentinvres.Status == "ERROR" {
		u.notify(t.LNURLERROR, t.T{
			"Host":   params.CallbackURL.Host,
			"Reason": sentinvres.Reason,
		})
		return
	}
	go u.track("lnurl-withdraw", map[string]interface{}{"sats": params.MaxWithdrawable})
}

func handleLNURLPay(u User, opts handleLNURLOpts, params lnurl.LNURLPayResponse1) {
	// display metadata and ask for amount
	var fixedAmount int64 = 0
	if params.MaxSendable == params.MinSendable {
		fixedAmount = params.MaxSendable
	}

	go u.track("lnurl-pay", map[string]interface{}{
		"domain": params.CallbackURL.Host,
		"fixed":  float64(fixedAmount) / 1000,
		"max":    float64(params.MaxSendable) / 1000,
		"min":    float64(params.MinSendable) / 1000,
	})

	if fixedAmount > 0 &&
		opts.payWithoutPromptIf != nil &&
		fixedAmount < *opts.payWithoutPromptIf+3000 {
		// we have the amount already

		if params.CommentAllowed > 0 {
			// need a comment
			lnurlpayAskForComment(u, params.Callback, params.EncodedMetadata, fixedAmount, opts.messageId)
		} else {
			// we have everything, proceed to pay
			lnurlpayFinish(
				u,
				fixedAmount,
				"",
				params.Callback,
				params.EncodedMetadata,
				opts.messageId,
			)
		}
	} else {
		// must ask for amount
		tmpldata := t.T{
			"Domain":      params.CallbackURL.Host,
			"FixedAmount": float64(fixedAmount) / 1000,
			"Max":         float64(params.MaxSendable) / 1000,
			"Min":         float64(params.MinSendable) / 1000,
		}

		baseChat := tgbotapi.BaseChat{
			ChatID:           u.TelegramChatId,
			ReplyToMessageID: opts.messageId,
		}

		if fixedAmount > 0 {
			baseChat.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(
						translate(t.CANCEL, u.Locale),
						fmt.Sprintf("cancel=%d", u.Id)),
					tgbotapi.NewInlineKeyboardButtonData(
						translateTemplate(t.PAYAMOUNT, u.Locale,
							t.T{"Sats": float64(fixedAmount) / 1000}),
						fmt.Sprintf("lnurlpay=%d", fixedAmount)),
				),
			)
		} else {
			baseChat.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true}
		}

		var chattable tgbotapi.Chattable
		tmpldata["Text"] = params.Metadata.Description()
		text := translateTemplate(t.LNURLPAYPROMPT, u.Locale, tmpldata)

		chattable = tgbotapi.MessageConfig{
			BaseChat:              baseChat,
			DisableWebPagePreview: true,
			ParseMode:             "HTML",
			Text:                  text,
		}
		if imagebytes := params.Metadata.ImageBytes(); imagebytes != nil {
			if err == nil {
				chattable = tgbotapi.PhotoConfig{
					BaseFile: tgbotapi.BaseFile{
						BaseChat: baseChat,
						File: tgbotapi.FileBytes{
							Name:  "image",
							Bytes: imagebytes,
						},
						MimeType: "image/" + params.Metadata.ImageExtension(),
					},
					ParseMode: "HTML",
					Caption:   text,
				}
			}
		}

		sent, err := tgsend(chattable)
		if err != nil {
			log.Warn().Err(err).Msg("error sending lnurl-pay message")
			return
		}

		data, _ := json.Marshal(struct {
			Type         string `json:"type"`
			Metadata     string `json:"metadata"`
			URL          string `json:"url"`
			NeedsComment bool   `json:"needs_comment"`
		}{"lnurlpay-amount", params.EncodedMetadata, params.Callback, params.CommentAllowed > 0})
		rds.Set(fmt.Sprintf("reply:%d:%d", u.Id, sent.MessageID), data, time.Hour*1)
	}
}

func handleLNURLPayAmount(u User, msats int64, data gjson.Result, messageId int) {
	// get data from redis object
	callback := data.Get("url").String()
	metadata := data.Get("metadata").String()
	needsComment := data.Get("needs_comment").Bool()

	if needsComment {
		// ask for comment
		lnurlpayAskForComment(u, callback, metadata, msats, messageId)
	} else {
		// proceed to fetch invoice and pay
		lnurlpayFinish(u, msats, "", callback, metadata, messageId)
	}
}

func handleLNURLPayComment(u User, comment string, data gjson.Result, messageId int) {
	// get data from redis object
	callback := data.Get("url").String()
	metadata := data.Get("metadata").String()
	msats := data.Get("msatoshi").Int()

	// proceed to fetch invoice and pay
	lnurlpayFinish(u, msats, comment, callback, metadata, messageId)
}

func lnurlpayAskForComment(u User, callback, metadata string, msats int64, messageId int) {
	callbackURL, _ := url.Parse(callback)

	sent, err := tgsend(tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:           u.TelegramChatId,
			ReplyToMessageID: messageId,
			ReplyMarkup:      tgbotapi.ForceReply{ForceReply: true},
		},
		DisableWebPagePreview: true,
		ParseMode:             "HTML",
		Text: translateTemplate(t.LNURLPAYPROMPTCOMMENT, u.Locale, t.T{
			"Domain": callbackURL.Host,
		}),
	})
	if err != nil {
		log.Warn().Err(err).Msg("error sending lnurl-pay message")
		return
	}

	data, _ := json.Marshal(struct {
		Type     string `json:"type"`
		Metadata string `json:"metadata"`
		MSatoshi int64  `json:"msatoshi"`
		URL      string `json:"url"`
	}{"lnurlpay-comment", metadata, msats, callback})
	rds.Set(fmt.Sprintf("reply:%d:%d", u.Id, sent.MessageID), data, time.Hour*1)
}

func lnurlpayFinish(u User, msats int64, comment, callback, metadata string, messageId int) {
	params := &url.Values{
		"amount": {fmt.Sprintf("%d", msats)},
	}
	if comment != "" {
		params.Set("comment", comment)
	}

	// call callback with params and get invoice
	var res lnurl.LNURLPayResponse2
	_, err := napping.Get(callback, params, &res, &res)
	if err != nil {
		u.notify(t.ERROR, t.T{"Err": err.Error()})
		return
	}
	if res.Status == "ERROR" {
		callbackURL, _ := url.Parse(callback)
		if callbackURL == nil {
			callbackURL = &url.URL{Host: "<unknown>"}
		}

		u.notify(t.LNURLERROR, t.T{
			"Host":   callbackURL.Host,
			"Reason": res.Reason,
		})
		return
	}

	log.Debug().Interface("res", res).Msg("got lnurl-pay values")

	// check invoice amount
	inv, err := decodepay.Decodepay(res.PR)
	if err != nil {
		u.notify(t.ERROR, t.T{"Err": err.Error()})
		return
	}

	if inv.DescriptionHash != calculateHash(metadata) {
		u.notify(t.ERROR, t.T{"Err": "Got invoice with wrong description_hash"})
		return
	}

	if int64(inv.MSatoshi) != msats {
		u.notify(t.ERROR, t.T{"Err": "Got invoice with wrong amount."})
		return
	}

	processingMessage := sendTelegramMessage(u.TelegramChatId,
		res.PR+"\n\n"+translate(t.PROCESSING, u.Locale),
	)

	// pay it
	hash, err := u.payInvoice(messageId, res.PR, 0)
	if err == nil {
		deleteMessage(&processingMessage)

		// wait until lnurl-pay is paid successfully.
		go func() {
			preimage := <-waitPaymentSuccess(hash)
			bpreimage, _ := hex.DecodeString(preimage)
			callbackURL, _ := url.Parse(callback)

			// send raw metadata, for later checking with the description_hash
			file := tgbotapi.DocumentConfig{
				BaseFile: tgbotapi.BaseFile{
					BaseChat: tgbotapi.BaseChat{ChatID: u.TelegramChatId},
					File: tgbotapi.FileBytes{
						Name:  calculateHash(metadata) + ".json",
						Bytes: []byte(metadata),
					},
					MimeType:    "text/json",
					UseExisting: false,
				},
			}
			file.Caption = translateTemplate(t.LNURLPAYMETADATA, u.Locale, t.T{
				"Domain":         callbackURL.Host,
				"Hash":           inv.PaymentHash,
				"HashFirstChars": inv.PaymentHash[:5],
			})
			file.ParseMode = "HTML"
			tgsend(file)

			// notify user with success action end applicable
			if res.SuccessAction != nil {
				var text string
				var decerr error

				switch res.SuccessAction.Tag {
				case "message":
					text = res.SuccessAction.Message
				case "url":
					text = res.SuccessAction.Description
				case "aes":
					text, decerr = res.SuccessAction.Decipher(bpreimage)
				}

				// give it a time so it's the last message to be sent
				time.Sleep(2 * time.Second)

				u.notifyAsReply(t.LNURLPAYSUCCESS, t.T{
					"Domain":        callbackURL.Host,
					"Text":          text,
					"URL":           res.SuccessAction.URL,
					"DecipherError": decerr,
				}, messageId)
			}
		}()
	} else {
		u.notifyAsReply(t.ERROR, t.T{"Err": err.Error()}, processingMessage.MessageID)
	}
}

func handleLNURLAllowance(u User, opts handleLNURLOpts, params lnurl.LNURLAllowanceResponse) {
	tmpldata := t.T{
		"Domain":      params.SocketURL.Host,
		"Amount":      float64(params.RecommendedAllowanceAmount) / 1000,
		"Description": params.Description,
	}

	baseChat := tgbotapi.BaseChat{
		ChatID:           u.TelegramChatId,
		ReplyToMessageID: opts.messageId,
	}

	baseChat.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				translate(t.CANCEL, u.Locale),
				fmt.Sprintf("cancel=%d", u.Id)),
			tgbotapi.NewInlineKeyboardButtonData(
				translateTemplate(t.PAYAMOUNT, u.Locale, tmpldata),
				fmt.Sprintf("lnurlall=%d", params.RecommendedAllowanceAmount/1000)),
		),
	)

	var chattable tgbotapi.Chattable
	text := "" // translateTemplate(t.LNURLALLOWANCEPROMPT, u.Locale, tmpldata)

	chattable = tgbotapi.MessageConfig{
		BaseChat:              baseChat,
		DisableWebPagePreview: true,
		ParseMode:             "HTML",
		Text:                  text,
	}
	if imagebytes := params.ImageBytes(); imagebytes != nil {
		if err == nil {
			chattable = tgbotapi.PhotoConfig{
				BaseFile: tgbotapi.BaseFile{
					BaseChat: baseChat,
					File: tgbotapi.FileBytes{
						Name:  "image",
						Bytes: imagebytes,
					},
					MimeType: "image/png",
				},
				ParseMode: "HTML",
				Caption:   text,
			}
		}
	}

	sent, err := tgsend(chattable)
	if err != nil {
		log.Warn().Err(err).Msg("error sending lnurl-allowance message")
		return
	}

	data, _ := json.Marshal(struct {
		Type   string `json:"type"`
		Socket string `json:"socket"`
		K1     string `json:"k1"`
	}{"lnurlpay", params.Socket, params.K1})
	rds.Set(fmt.Sprintf("reply:%d:%d", u.Id, sent.MessageID), data, time.Hour*1)
}

func handleLNURLAllowanceConfirmation(u User, msats int64, data gjson.Result, messageId int) {
	// // get data from redis object
	// socket := data.Get("socket").String()
	// k1 := data.Get("k1").String()

	// // proceed to establish a session
	// session, err := allowance_socket.Connect(socket, msats, k1)
	// if err != nil {
	// 	u.notify(t.ERROR, t.T{"Err": err.Error()})
	// 	return
	// }

	// // continuously send balance
	// go func () {
	//     err := session.Send(lnurl.AllowanceBalance{

	// })
	//     if err != nil{
	//     return
	//     }
	// }()

	// for {
	//     select {
	//     case session.
	//     }
	// }
}
