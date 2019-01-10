package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
	"github.com/tidwall/gjson"
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

func makeInvoice(u User, label string, sats int, desc string) (bolt11 string, qrpath string, err error) {
	log.Debug().Str("label", label).Str("desc", desc).Int("sats", sats).
		Msg("generating invoice")

	// save invoice creator on redis
	rds.Set("recinvoice:"+label+":creator", u.Id, s.InvoiceTimeout)

	// make invoice
	res, err := ln.Call("invoice", strconv.Itoa(sats*1000),
		label, desc, strconv.Itoa(int(s.InvoiceTimeout/time.Second)))
	if err != nil {
		return
	}
	bolt11 = res.Get("bolt11").String()

	// save this bolt11 on redis so we know if someone tries
	// to pay it from this same wallet/bot
	rds.Set("recinvoice.internal:"+bolt11, label, s.InvoiceTimeout)

	// generate qr code
	err = qrcode.WriteFile(bolt11, qrcode.Medium, 256, qrImagePath(label))
	if err != nil {
		log.Warn().Err(err).Str("invoice", bolt11).
			Msg("failed to generate qr.")
		err = nil
	} else {
		qrpath = qrImagePath(label)
	}

	return
}
