package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
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
	Label          sql.NullString `db:"label"`
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
	switch t.Tag.String {
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
	case "golightning", "bitflash":
		return "⛓️"
	case "bitclouds":
		return "☁️"
	case "lntorub":
		return "💸"
	case "poker":
		return "♠️"
	case "paywall":
		return "🧱"
	default:
		switch {
		case strings.HasPrefix(t.Label.String, "newmember:"):
			return "🎟️"
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

func (t Transaction) PayeeAlias() string {
	return getNodeAlias(t.Payee.String)
}

func (t Transaction) PayeeLink() string {
	return nodeLink(t.Payee.String)
}

func decimalize(v float64) string {
	return fmt.Sprintf("%.15g", v)
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

func renderLogInfo(hash string) (logInfo string) {
	if len(hash) < 5 {
		return ""
	}

	lastCall, err := rds.Get("tries:" + hash[:5]).Result()
	if err != nil {
		return ""
	}

	logInfo += "<b>Routes tried:</b>"

	var tries []Try
	err = json.Unmarshal([]byte(lastCall), &tries)
	if err != nil {
		logInfo += " [error fetching]"
	}

	if len(tries) == 0 {
		logInfo += " No routes found."
	}

	for j, try := range tries {
		letter := string([]rune{rune(j) + 97})
		logInfo += fmt.Sprintf("\n  <b>%s</b>. ", letter)
		if try.Success {
			logInfo += "<i>Succeeded.</i>"
		} else {
			logInfo += "<i>Failed.</i>"
		}

		routeStr := ""
		for l, hop := range try.Route {
			routeStr += fmt.Sprintf("\n    <code>%s</code>. %s, %dmsat, delay: %d",
				strings.ToLower(roman(l+1)),
				nodeLink(hop.Peer), hop.Msatoshi, hop.Delay)
		}
		logInfo += routeStr

		if try.Error != "" {
			logInfo += fmt.Sprintf("\nError: %s. ", try.Error)
		}
	}

	return
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
		"LogInfo": renderLogInfo(txn.Hash),
	})
	msgId := sendMessageAsReply(u.ChatId, txstatus, txn.TriggerMessage).MessageID

	if txn.Status == "PENDING" {
		// allow people to cancel pending if they're old enough
		editWithKeyboard(u.ChatId, msgId, txstatus+"\n\n"+translate(t.RECHECKPENDING, u.Locale),
			tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(translate(t.YES, u.Locale), "check="+hashfirstchars),
				),
			),
		)
	}

	if txn.IsUnclaimed() {
		editWithKeyboard(u.ChatId, msgId, txstatus+"\n\n"+translate(t.RETRACTQUESTION, u.Locale),
			tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(translate(t.YES, u.Locale), "remunc="+hashfirstchars),
				),
			),
		)
	}
}

func handleTransactionList(u User, page int, filter InOut, cb *tgbotapi.CallbackQuery) {
	// show list of transactions
	if page == 0 {
		page = 1
	}
	limit := 25
	offset := limit * (page - 1)

	txns, err := u.listTransactions(limit, offset, 16, filter)
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
				"newer", fmt.Sprintf("txlist=%d-%s", page-1, filter)),
		)
	}

	if len(txns) > 0 {
		keyboard.InlineKeyboard[0] = append(
			keyboard.InlineKeyboard[0],
			tgbotapi.NewInlineKeyboardButtonData(
				"older", fmt.Sprintf("txlist=%d-%s", page+1, filter)),
		)
	}

	var chattable tgbotapi.Chattable
	if cb == nil {
		chattable = tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:      u.ChatId,
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
