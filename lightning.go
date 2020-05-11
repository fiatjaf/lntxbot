package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"

	lightning "github.com/fiatjaf/lightningd-gjson-rpc"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/tidwall/gjson"
)

type BalanceGetter interface {
	Get(interface{}, string, ...interface{}) error
}

func getBalance(txn BalanceGetter, userId int) int64 {
	var balance int64
	err = txn.Get(&balance, "SELECT balance::numeric(13) FROM lightning.balance WHERE account_id = $1", userId)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Warn().Err(err).Int("account", userId).Msg("failed to fetch balance")
		}
		return 0
	}
	return balance
}

func findInvoiceOnNode(hash, preimage string) (gjson.Result, bool) {
	if hash == "" {
		preimagehex, _ := hex.DecodeString(preimage)
		sum := sha256.Sum256(preimagehex)
		hash = hex.EncodeToString(sum[:])
	}

	invs, err := ln.Call("listinvoices")
	if err == nil {
		for _, inv := range invs.Get("invoices").Array() {
			if inv.Get("payment_hash").String() == hash {
				return inv, true
			}
		}
	}

	return gjson.Result{}, false
}

var nodeAliases = cmap.New()

func getNodeAlias(id string) string {
begin:
	if alias, ok := nodeAliases.Get(id); ok {
		return alias.(string)
	}

	if id == "" {
		return "~"
	}

	res, err := ln.Call("listnodes", id)
	if err != nil {
		return "~"
	}

	alias := res.Get("nodes.0.alias").String()
	if alias == "" {
		alias = "~"
	}

	nodeAliases.Set(id, alias)
	goto begin
}

func messageFromError(err error) string {
	switch terr := err.(type) {
	case lightning.ErrorTimeout:
		return fmt.Sprintf("Operation has timed out after %d seconds.", terr.Seconds)
	case lightning.ErrorCommand:
		return terr.Message
	case lightning.ErrorConnect, lightning.ErrorConnectionBroken:
		return "Problem connecting to our node. Please try again in a minute."
	case lightning.ErrorJSONDecode:
		return "Error reading response from lightningd."
	default:
		return err.Error()
	}
}
