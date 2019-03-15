package main

import (
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"time"
)

type Transaction struct {
	Time         time.Time      `db:"time"`
	Status       string         `db:"status"`
	TelegramPeer sql.NullString `db:"telegram_peer"`
	Amount       float64        `db:"amount"`
	Fees         float64        `db:"fees"`
	Hash         string         `db:"payment_hash"`
	Preimage     string         `db:"preimage"`
	Description  string         `db:"description"`
}

func (t Transaction) PeerActionDescription() string {
	if !t.TelegramPeer.Valid {
		return ""
	}

	name := "@" + t.TelegramPeer.String
	if _, err := strconv.Atoi(t.TelegramPeer.String); err == nil {
		name = fmt.Sprintf("[user-%[1]s](tg://user?id=%[1]s)", t.TelegramPeer.String)
	}

	if t.Status == "RECEIVED" {
		return "from " + name
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
	return fmt.Sprintf("%.3f", math.Abs(t.Amount))
}

func (t Transaction) PaddedSatoshis() string {
	if math.Abs(t.Amount) > 999999 {
		return fmt.Sprintf("%7.0f", t.Amount)
	}
	return fmt.Sprintf("%7.1f", t.Amount)
}

func (t Transaction) FeeSatoshis() string {
	return fmt.Sprintf("%.3f", t.Fees)
}

func (t Transaction) HashReduced() string {
	return t.Hash[:5]
}
