package main

import (
	"errors"

	"github.com/lucsky/cuid"
	"github.com/tidwall/gjson"
)

func makeLabel() string {
	return s.ServiceId + "." + cuid.Slug()
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
