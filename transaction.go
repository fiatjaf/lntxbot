package main

import (
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"time"
)

type Transaction struct {
	Time           time.Time      `db:"time"`
	Status         string         `db:"status"`
	TelegramPeer   sql.NullString `db:"telegram_peer"`
	Anonymous      bool           `db:"anonymous"`
	TriggerMessage int            `db:"trigger_message"`
	Amount         float64        `db:"amount"`
	Fees           float64        `db:"fees"`
	Hash           string         `db:"payment_hash"`
	Preimage       string         `db:"preimage"`
	Description    string         `db:"description"`
	Payee          sql.NullString `db:"payee_node"`
	PendingBolt11  sql.NullString `db:"pending_bolt11"`

	unclaimed *bool
}

func (t Transaction) PeerActionDescription() string {
	if !t.TelegramPeer.Valid {
		return ""
	}

	name := "@" + t.TelegramPeer.String
	if _, err := strconv.Atoi(t.TelegramPeer.String); err == nil {
		name = fmt.Sprintf(`tg://user?id=%[1]s`, t.TelegramPeer.String)
	}

	if t.Status == "RECEIVED" {
		if t.Anonymous {
			return "from someone"
		} else {
			return "from " + name
		}
	} else {
		return "to " + name
	}
}

func (t Transaction) StatusSmall() string {
	switch t.Status {
	case "RECEIVED":
		return "R"
	case "SENT":
		return "S"
	case "PENDING":
		return "-"
	default:
		return t.Status
	}
}

func (t Transaction) IsReceive() bool {
	return t.Status == "RECEIVED"
}

func (t Transaction) IsPending() bool {
	return t.Status == "PENDING"
}

func (t Transaction) IsUnclaimed() bool {
	if !t.TelegramPeer.Valid {
		return false
	}

	if t.unclaimed != nil {
		return *t.unclaimed
	}

	var unclaimed bool
	err := pg.Get(&unclaimed, `
SELECT is_unclaimed(tx)
FROM lightning.transaction AS tx
WHERE tx.payment_hash = $1
    `, t.Hash)
	if err != nil {
		log.Error().Err(err).Str("hash", t.Hash).
			Msg("failed to query unclaimedship of transaction")
		unclaimed = false
	}

	t.unclaimed = &unclaimed
	return unclaimed
}

func (t Transaction) HasPreimage() bool {
	return t.Preimage != ""
}

func (t Transaction) TimeFormat() string {
	return t.Time.Format("2 Jan 2006 at 3:04PM")
}

func (t Transaction) TimeFormatSmall() string {
	return t.Time.Format("2 Jan 15:04")
}

func (t Transaction) Satoshis() string {
	return decimalize(math.Abs(t.Amount))
}

func (t Transaction) PaddedSatoshis() string {
	if t.Amount > 99999 {
		return fmt.Sprintf("%7.0f", t.Amount)
	}
	if t.Amount < -9999 {
		return fmt.Sprintf("%7.0f", t.Amount)
	}
	return fmt.Sprintf("%7.1f", t.Amount)
}

func (t Transaction) FeeSatoshis() string {
	return decimalize(t.Fees)
}

func (t Transaction) HashReduced() string {
	return t.Hash[:5]
}

func (t Transaction) Icon() string {
	switch {
	case t.TelegramPeer.Valid:
		if t.IsUnclaimed() {
			return "💤 "
		}

		switch t.Description {
		case "giveaway":
			return "🎁"
		case "coinflip":
			return "🎲"
		case "fundraise":
			return "📢"
		default:
			if t.Anonymous {
				return "🕵"
			}

			return ""
		}
	case t.IsPending():
		return "🕓"
	default:
		return "⚡"
	}
}

func (t Transaction) PayeeAlias() string {
	return getNodeAlias(t.Payee.String)
}

func decimalize(v float64) string {
	if v == math.Trunc(v) {
		return fmt.Sprintf("%.0f", v)
	} else {
		return fmt.Sprintf("%.3f", v)
	}
}
