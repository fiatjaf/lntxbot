package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fiatjaf/lightningd-gjson-rpc"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/kballard/go-shellquote"
	"github.com/skip2/go-qrcode"
	"github.com/tidwall/gjson"
	"gopkg.in/jmcvetta/napping.v3"
)

func makeLabel(chatId int64, messageId interface{}) string {
	return fmt.Sprintf("%s.%d.%v", s.ServiceId, chatId, messageId)
}

func messageIdFromLabel(label string) int {
	parts := strings.Split(label, ".")
	if len(parts) == 3 {
		id, _ := strconv.Atoi(parts[2])
		return id
	}
	return 0
}

func qrImagePath(label string) string {
	return filepath.Join(os.TempDir(), s.ServiceId+".invoice."+label+".png")
}

func searchForInvoice(message tgbotapi.Message) (bolt11 string, ok bool) {
	text := message.Text
	if text == "" {
		text = message.Caption
	}

	if bolt11, ok = getBolt11(text); ok {
		return
	}

	// receiving a picture, try to decode the qr code
	if message.Photo != nil && len(*message.Photo) > 0 {
		log.Debug().Msg("got photo, looking for qr code.")

		photos := *message.Photo
		photo := photos[len(photos)-1]

		photourl, err := bot.GetFileDirectURL(photo.FileID)
		if err != nil {
			log.Warn().Err(err).Str("fileid", photo.FileID).
				Msg("failed to get photo URL.")
			return
		}

		p := &url.Values{}
		p.Set("fileurl", photourl)
		var r []struct {
			Type   string `json:"type"`
			Symbol []struct {
				Data  string `json:"data"`
				Error string `json:"error"`
			} `json:"symbol"`
		}
		_, err = napping.Get("https://api.qrserver.com/v1/read-qr-code/", p, &r, nil)
		if err != nil {
			log.Warn().Err(err).Str("url", photourl).Msg("failed to call qrserver")
			return
		}
		if len(r) == 0 || len(r[0].Symbol) == 0 {
			log.Warn().Str("url", photourl).Msg("invalid response from  qrserver")
			return
		}
		if r[0].Symbol[0].Error != "" {
			log.Debug().Str("err", r[0].Symbol[0].Error).
				Str("url", photourl).Msg("qrserver failed to decode")
			return
		}

		text = r[0].Symbol[0].Data
		log.Debug().Str("data", text).Msg("got qr code data")
		return getBolt11(text)
	}

	return
}

func getBolt11(text string) (bolt11 string, ok bool) {
	text = strings.ToLower(text)

	argv, err := shellquote.Split(text)
	if err != nil {
		return
	}

	for _, arg := range argv {
		if strings.HasPrefix(arg, "lightning:") {
			arg = arg[10:]
		}

		if strings.HasPrefix(arg, "lnbc") {
			return arg, true
		}
	}

	return
}

func decodeInvoice(invoice string) (inv gjson.Result, err error) {
	inv, err = ln.Call("decodepay", invoice)
	if err != nil {
		return
	}
	if inv.Get("code").Int() != 0 {
		return inv, errors.New(inv.Get("message").String())
	}

	return
}

func makeInvoice(
	u User,
	label string,
	sats int,
	desc string,
	preimage string,
) (bolt11 string, qrpath string, err error) {
	log.Debug().Str("label", label).Str("desc", desc).Int("sats", sats).Str("preimage", preimage).
		Msg("generating invoice")

	if preimage == "" {
		preimage, err = randomPreimage()
		if err != nil {
			return
		}
	}

	// save invoice creator and preimage on redis
	rds.Set("recinvoice:"+label+":creator", u.Id, s.InvoiceTimeout)
	rds.Set("recinvoice:"+label+":preimage", preimage, s.InvoiceTimeout)

	// make invoice
	res, err := ln.CallWithCustomTimeout(time.Second*40, "invoice", map[string]interface{}{
		"msatoshi":    sats * 1000,
		"label":       label,
		"description": desc + " [" + s.ServiceId + "/" + u.AtName() + "]",
		"expiry":      int(s.InvoiceTimeout / time.Second),
		"preimage":    preimage,
	})
	if err != nil {
		return
	}
	bolt11 = res.Get("bolt11").String()

	// save this bolt11 on redis so we know if someone tries
	// to pay it from this same wallet/bot
	rds.Set("recinvoice.internal:"+bolt11, label, s.InvoiceTimeout)

	// generate qr code
	err = qrcode.WriteFile(strings.ToUpper(bolt11), qrcode.Medium, 256, qrImagePath(label))
	if err != nil {
		log.Warn().Err(err).Str("invoice", bolt11).
			Msg("failed to generate qr.")
		err = nil
	} else {
		qrpath = qrImagePath(label)
	}

	return
}

func messageFromError(err error, prefix string) string {
	var msg string
	switch terr := err.(type) {
	case lightning.ErrorTimeout:
		msg = fmt.Sprintf("Operation has timed out after %d seconds.", terr.Seconds)
	case lightning.ErrorCommand:
		msg = terr.Msg
	case lightning.ErrorConnect, lightning.ErrorConnectionBroken:
		msg = "Problem connecting to our node. Please try again in a minute."
	case lightning.ErrorJSONDecode:
		msg = "Error reading response from lightningd."
	default:
		msg = err.Error()
	}
	return prefix + ": " + msg
}

func randomPreimage() (string, error) {
	hex := []rune("0123456789abcdef")
	b := make([]rune, 64)
	for i := range b {
		r, err := rand.Int(rand.Reader, big.NewInt(16))
		if err != nil {
			return "", err
		}
		b[i] = hex[r.Int64()]
	}
	return string(b), nil
}

func payInvoice(u User, messageId int, bolt11, label string, optmsats int) (payment_sent bool) {
	// check if this is an internal invoice (it will have a different label)
	intlabel, err := rds.Get("recinvoice.internal:" + bolt11).Result()
	if err == nil && intlabel != "" {
		// this is an internal invoice. do not pay.
		// delete it and just transfer balance.
		rds.Del("recinvoice.internal:" + bolt11)
		ln.Call("delinvoice", intlabel, "unpaid")

		targetId, err := rds.Get("recinvoice:" + intlabel + ":creator").Int64()
		if err != nil {
			log.Warn().Err(err).
				Str("intlabel", intlabel).
				Msg("failed to get internal invoice target from redis")
			u.notify("Failed to find invoice payee.")
			return false
		}
		target, err := loadUser(int(targetId), 0)
		if err != nil {
			log.Warn().Err(err).
				Str("intlabel", intlabel).
				Int64("id", targetId).
				Msg("failed to get load internal invoice target from postgres")
			u.notify("Failed to find invoice payee")
			return false
		}

		amount, hash, errMsg, err := u.payInternally(
			messageId,
			target,
			bolt11,
			intlabel,
			optmsats,
		)
		if err != nil {
			log.Warn().Err(err).
				Str("intlabel", intlabel).
				Msg("failed to pay pay internally")
			u.notify("Failed to pay: " + errMsg)

			return false
		}

		// internal payment succeeded
		target.notifyAsReply(
			fmt.Sprintf("Payment received: %d satoshis. /tx%s.", amount/1000, hash[:5]),
			messageIdFromLabel(intlabel),
		)

		return true
	}

	err = u.payInvoice(messageId, bolt11, label, optmsats)
	if err != nil {
		u.notifyAsReply(err.Error(), messageId)
		return false
	}
	return true
}

func processCoinflip(sats int, winnerId int, participants []int, mId int) (winner User, err error) {
	txn, err := pg.BeginTxx(context.TODO(),
		&sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return
	}
	defer txn.Rollback()

	msats := sats * 1000
	var (
		vdesc  = &sql.NullString{}
		vlabel = &sql.NullString{}
	)
	vdesc.Scan("coinflip")

	for _, partId := range participants {
		if partId == winnerId {
			continue
		}

		_, err = txn.Exec(`
INSERT INTO lightning.transaction
  (from_id, to_id, amount, description, label, trigger_message)
VALUES ($1, $2, $3, $4, $5, $6)
    `, partId, winnerId, msats, vdesc, vlabel, mId)
		if err != nil {
			return
		}

		var balance int
		err = txn.Get(&balance, `
SELECT balance::int FROM lightning.balance WHERE account_id = $1
    `, partId)
		if err != nil {
			return
		}

		if balance < 0 {
			err = errors.New("insufficient balance")
			return
		}
	}

	err = txn.Commit()
	if err != nil {
		return
	}

	winner, _ = loadUser(winnerId, 0)
	return
}
