package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	decodepay "github.com/fiatjaf/ln-decodepay"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// powered by deezy.io

func handleSendToAddress(ctx context.Context, address string, msats int64) {
	u := ctx.Value("initiator").(*User)

	params, _ := json.Marshal(struct {
		AmountSats          int64  `json:"amount_sats"`
		OnChainAddress      string `json:"on_chain_address"`
		OnChainSatsPerVByte int    `json:"on_chain_sats_per_vbyte"`
	}{msats / 1000, address, 2})

	resp, err := http.Post("https://api.deezy.io/v1/swap", "application/json", bytes.NewBuffer(params))
	if err != nil {
		send(ctx, u, t.ERROR, t.T{
			"Err": fmt.Sprintf("failed to call deezy.io API: %s", err.Error()),
		})
		return
	}
	if resp.StatusCode != 200 {
		b, _ := ioutil.ReadAll(resp.Body)
		text := string(b)
		send(ctx, u, t.ERROR, t.T{
			"Err": fmt.Sprintf("deezy.io API returned an error (%d): %s", resp.StatusCode, text),
		})
		return
	}

	var val struct {
		Bolt11Invoice string `json:"bolt11_invoice"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&val); err != nil || val.Bolt11Invoice == "" {
		send(ctx, u, t.ERROR, t.T{
			"Err": fmt.Sprintf("deezy.io API returned a broken response"),
		})
		return
	}

	processingMessageId := send(ctx, u, val.Bolt11Invoice+"\n\n"+translate(ctx, t.PROCESSING))

	if inv, err := decodepay.Decodepay(val.Bolt11Invoice); err != nil {
		send(ctx, u, t.ERROR,
			t.T{"Err": fmt.Errorf("error parsing invoice: %w", err)},
			processingMessageId)
		return
	} else if inv.MSatoshi > (msats + 800) {
		send(ctx, u, t.ERROR,
			t.T{"Err": "invoice is too expensive, so we're stopping here just in case, let us know if this is wrong"},
			processingMessageId)
		return
	}

	if _, err := u.payInvoice(ctx, val.Bolt11Invoice, 0); err != nil {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()}, processingMessageId)
	} else {
		deleteMessage(&tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: u.TelegramChatId},
			MessageID: processingMessageId.(int),
		})
	}
}
