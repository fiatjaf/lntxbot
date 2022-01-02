package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/btcsuite/btcd/btcec"
	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/lntxbot/t"
)

type Info struct {
	AccountId     string  `db:"account_id"`
	BalanceMsat   int64   `db:"balance_msat"`
	Balance       float64 `db:"balance"`
	UsableBalance float64 `db:"usable"`
	TotalSent     float64 `db:"totalsent"`
	TotalReceived float64 `db:"totalrecv"`
	TotalFees     float64 `db:"fees"`
}

type TaggedBalance struct {
	Tag     string  `db:"tag"`
	Balance float64 `db:"balance"`
}

func (u User) LinkingKey(domain string) (*btcec.PrivateKey, *btcec.PublicKey) {
	seedhash := sha256.Sum256([]byte(
		fmt.Sprintf("lnurlkeyseed:%s:%d:%s",
			domain, u.Id, s.TelegramBotToken)))
	return btcec.PrivKeyFromBytes(btcec.S256(), seedhash[:])
}

func (u User) SignKeyAuth(domain string, k1hex string) (key string, sig string, err error) {
	// lnurl-auth: create a key based on the user id and sign with it
	sk, pk := u.LinkingKey(domain)

	k1, err := hex.DecodeString(k1hex)
	if err != nil {
		return "", "", fmt.Errorf("invalid k1 hex '%s': %w", k1hex, err)
	}

	signature, err := sk.Sign(k1)
	if err != nil {
		return "", "", fmt.Errorf("error signing k1: %w", err)
	}

	sig = hex.EncodeToString(signature.Serialize())
	key = hex.EncodeToString(pk.SerializeCompressed())

	return key, sig, nil
}

func (u User) getInfo() (info Info, err error) {
	err = pg.Get(&info, `
SELECT
  b.account_id,
  b.balance AS balance_msat,
  b.balance/1000 AS balance,
  b.balance * 0.995 / 1000 AS usable,
  (
    SELECT coalesce(sum(amount), 0)::float/1000 FROM lightning.transaction AS t
    WHERE b.account_id = t.to_id
  ) AS totalrecv,
  (
    SELECT coalesce(sum(amount), 0)::float/1000 FROM lightning.transaction AS t
    WHERE b.account_id = t.from_id
  ) AS totalsent,
  (
    SELECT coalesce(sum(fees), 0)::float/1000 FROM lightning.transaction AS t
    WHERE b.account_id = t.from_id
  ) AS fees
FROM lightning.balance AS b
WHERE b.account_id = $1
GROUP BY b.account_id, b.balance
    `, u.Id)
	if err == sql.ErrNoRows {
		info = Info{
			AccountId:     strconv.Itoa(u.Id),
			Balance:       0,
			UsableBalance: 0,
			TotalSent:     0,
			TotalReceived: 0,
			TotalFees:     0,
		}
		err = nil
	}

	return
}

func (u User) getTaggedBalances() (balances []TaggedBalance, err error) {
	err = pg.Select(&balances, `
SELECT
  tag,
  sum(amount)::float/1000 AS balance
FROM lightning.account_txn
WHERE account_id = $1 AND tag IS NOT NULL
GROUP BY tag
    `, u.Id)
	return
}

func (u User) checkBalanceFor(ctx context.Context, msats int64, purpose string) bool {
	if _, ok := s.Banned[u.Id]; ok {
		log.Debug().Stringer("user", &u).Msg("got balance check on banned user")
		return false
	}

	if info, err := u.getInfo(); err != nil || info.BalanceMsat < msats {
		send(ctx, u, t.INSUFFICIENTBALANCE, t.T{
			"Purpose": purpose,
			"Sats":    float64(msats/1000) - info.Balance + 1,
		}, WITHALERT)
		return false
	}
	return true
}

func (u User) listTransactions(limit, offset, descCharLimit int, tag string, inOrOut InOut) (txns []Transaction, err error) {
	var filter string
	switch inOrOut {
	case In:
		filter += " AND amount > 0 "
	case Out:
		filter += " AND amount < 0 "
	case Both:
		filter += ""
	}

	err = pg.Select(&txns, `
SELECT * FROM (
  SELECT
    time,
    telegram_peer,
    anonymous,
    status,
    CASE WHEN char_length(coalesce(description, '')) <= $4
      THEN coalesce(description, '')
      ELSE substring(coalesce(description, '') from 0 for ($4 - 1)) || '…'
    END AS description,
    tag,
    fees::float/1000 AS fees,
    amount::float/1000 AS amount,
    payment_hash,
    preimage
  FROM lightning.account_txn
  WHERE account_id = $1 `+filter+` AND (CASE WHEN $5 != '' THEN tag = $5 ELSE true END)
  ORDER BY time DESC
  LIMIT $2
  OFFSET $3
) AS latest ORDER BY time ASC
    `, u.Id, limit, offset, descCharLimit, tag)
	if err != nil {
		return
	}

	for i := range txns {
		txns[i].Description = escapeHTML(txns[i].Description)
	}

	return
}

func handleBalance(ctx context.Context, opts docopt.Opts) {
	u := ctx.Value("initiator").(User)

	go u.track("balance", map[string]interface{}{"apps": opts["apps"].(bool)})

	if opts["apps"].(bool) {
		// balance of apps
		taggedbalances, err := u.getTaggedBalances()
		if err != nil {
			log.Warn().Err(err).Stringer("user", &u).Msg("failed to get info")
			send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
			return
		}

		send(ctx, u, t.TAGGEDBALANCEMSG, t.T{"Balances": taggedbalances})
	} else {
		// normal balance
		info, err := u.getInfo()
		if err != nil {
			log.Warn().Err(err).Stringer("user", &u).Msg("failed to get info")
			send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
			return
		}

		send(ctx, u, t.BALANCEMSG, t.T{
			"Sats":     info.Balance,
			"Usable":   info.UsableBalance,
			"Received": info.TotalReceived,
			"Sent":     info.TotalSent,
			"Fees":     info.TotalFees,
		})
	}
}
