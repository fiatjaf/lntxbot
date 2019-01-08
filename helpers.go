package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

func makeLabel(chatId int64, messageId int) string {
	return fmt.Sprintf("%s.%d.%d", s.ServiceId, chatId, messageId)
}

func messageIdFromLabel(label string) int {
	parts := strings.Split(label, ".")
	if len(parts) == 3 {
		id, _ := strconv.Atoi(parts[2])
		return id
	}
	return 0
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
