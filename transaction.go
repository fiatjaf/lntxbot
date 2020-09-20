package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type InOut string

const (
	In   InOut = "i"
	Out  InOut = "o"
	Both InOut = ""
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
	Preimage       sql.NullString `db:"preimage"`
	Description    string         `db:"description"`
	Tag            sql.NullString `db:"tag"`
	Payee          sql.NullString `db:"payee_node"`

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

func (t Transaction) HashReduced() string {
	return t.Hash[:5]
}

func (t Transaction) Icon() string {
	switch t.Tag.String {
	case "ticket":
		return "🎟️"
	case "giveaway", "gifts", "giveflip":
		return "🎁"
	case "coinflip":
		return "🎲"
	case "fundraise":
		return "🎷"
	case "reveal":
		return "🔎"
	case "sats4ads":
		return "📢"
	case "satellite":
		return "📡"
	case "microbet":
		return "⚽"
	case "golightning":
		return "⛓️"
	case "bitclouds":
		return "☁️"
	case "lntorub":
		return "💸"
	default:
		switch {
		case t.TelegramPeer.Valid:
			return ""
		case t.IsPending():
			return "🕓"
		case t.IsUnclaimed():
			return "💤"
		case t.Anonymous:
			return "🕵"
		default:
			return "⚡"
		}
	}
}

type Try struct {
	Route   []Hop
	Error   string
	Success bool
}

type Hop struct {
	Peer      string
	Channel   string
	Direction int64
	Msatoshi  int64
	Delay     int64
}

func renderLogInfo(u User, hash string) (logInfo string) {
	if len(hash) < 5 {
		return ""
	}

	lastCall, err := rds.Get("tries:" + hash[:5]).Result()
	if err != nil {
		return ""
	}

	var tries []Try
	err = json.Unmarshal([]byte(lastCall), &tries)
	if err != nil {
		return translateTemplate(t.ERROR, u.Locale, t.T{"Err": "failed to parse log"})
	}

	if len(tries) == 0 {
		return translateTemplate(t.ERROR, u.Locale, t.T{"Err": "no routes attempted"})
	}

	return translateTemplate(t.TXLOG, u.Locale, t.T{
		"Tries": tries,
	})
}

func handleSingleTransaction(u User, hashfirstchars string, messageId int) {
	txn, err := u.getTransaction(hashfirstchars)
	if err != nil {
		log.Warn().Err(err).Str("user", u.Username).Str("hash", hashfirstchars).
			Msg("failed to get transaction")
		u.notifyAsReply(t.TXNOTFOUND, t.T{"HashFirstChars": hashfirstchars}, messageId)
		return
	}

	txstatus := translateTemplate(t.TXINFO, u.Locale, t.T{
		"Txn":     txn,
		"LogInfo": renderLogInfo(u, txn.Hash),
	})
	msgId := sendTelegramMessageAsReply(u.TelegramChatId, txstatus, txn.TriggerMessage).MessageID

	if txn.Status == "PENDING" && txn.Time.Before(time.Now().AddDate(0, 0, -14)) {
		// allow people to cancel pending if they're old enough
		editWithKeyboard(u.TelegramChatId, msgId, txstatus+"\n\n"+translate(t.RECHECKPENDING, u.Locale),
			tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(translate(t.YES, u.Locale), "check="+hashfirstchars),
				),
			),
		)
	}

	if txn.IsUnclaimed() {
		editWithKeyboard(u.TelegramChatId, msgId, txstatus+"\n\n"+translate(t.RETRACTQUESTION, u.Locale),
			tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(translate(t.YES, u.Locale), "remunc="+hashfirstchars),
				),
			),
		)
	}
}

func handleTransactionList(u User, page int, tag string, filter InOut, cb *tgbotapi.CallbackQuery) {
	// show list of transactions
	if page == 0 {
		page = 1
	}
	limit := 25
	offset := limit * (page - 1)

	txns, err := u.listTransactions(limit, offset, 16, tag, filter)
	if err != nil {
		log.Warn().Err(err).Str("user", u.Username).Int("page", page).
			Msg("failed to list transactions")
		return
	}

	text := translateTemplate(t.TXLIST, u.Locale, t.T{
		"Offset":       offset,
		"Limit":        limit,
		"From":         offset + 1,
		"To":           offset + limit,
		"Transactions": txns,
	})

	keyboard := tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			[]tgbotapi.InlineKeyboardButton{},
		},
	}

	if page > 1 {
		keyboard.InlineKeyboard[0] = append(
			keyboard.InlineKeyboard[0],
			tgbotapi.NewInlineKeyboardButtonData(
				"newer", fmt.Sprintf("txl=%d-%s-%s", page-1, filter, tag)),
		)
	}

	if len(txns) > 0 {
		keyboard.InlineKeyboard[0] = append(
			keyboard.InlineKeyboard[0],
			tgbotapi.NewInlineKeyboardButtonData(
				"older", fmt.Sprintf("txl=%d-%s-%s", page+1, filter, tag)),
		)
	}

	var chattable tgbotapi.Chattable
	if cb == nil {
		chattable = tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:      u.TelegramChatId,
				ReplyMarkup: &keyboard,
			},
			Text:                  text,
			DisableWebPagePreview: true,
			ParseMode:             "HTML",
		}
	} else {
		baseEdit := getBaseEdit(cb)
		baseEdit.ReplyMarkup = &keyboard
		chattable = tgbotapi.EditMessageTextConfig{
			BaseEdit:              baseEdit,
			Text:                  text,
			DisableWebPagePreview: true,
			ParseMode:             "HTML",
		}
	}

	bot.Send(chattable)
}
