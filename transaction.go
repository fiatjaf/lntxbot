package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/fiatjaf/lightningd-gjson-rpc"
	"github.com/go-telegram-bot-api/telegram-bot-api"
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
		case "reveal":
			return "🔎"
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

func (t Transaction) PayeeLink() string {
	return nodeLink(t.Payee.String)
}

func decimalize(v float64) string {
	if v == math.Trunc(v) {
		return fmt.Sprintf("%.0f", v)
	} else {
		return fmt.Sprintf("%.3f", v)
	}
}

func renderLogInfo(hash string) (logInfo string) {
	lastCall, err := rds.Get("tries:" + hash).Result()
	if err != nil {
		return ""
	}

	logInfo += "<b>Routes tried:</b>"

	var tries []lightning.Try
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
		arrihop, ok := try.Route.([]interface{})
		if !ok {
			return "\n    [error]"
		}
		for l, ihop := range arrihop {
			hop := ihop.(map[string]interface{})
			peer := hop["id"].(string)
			msat := int(hop["msatoshi"].(float64))
			delay := int(hop["delay"].(float64))
			routeStr += fmt.Sprintf("\n    <code>%s</code>. %s, %dmsat, delay: %d",
				strings.ToLower(roman(l+1)), nodeLink(peer), msat, delay)
		}
		logInfo += routeStr

		if try.Error != nil {
			logInfo += fmt.Sprintf("\nError: %s (%d). ", try.Error.Message, try.Error.Code)
			if try.Error.Data != nil {
				data, _ := try.Error.Data.(map[string]interface{})
				ichannel, _ := data["erring_channel"]
				inode, _ := data["erring_node"]
				channel, _ := ichannel.(string)
				node, _ := inode.(string)
				logInfo += fmt.Sprintf("<b>Erring:</b> %s, %s", channel, nodeLink(node))
			}
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
		"LogInfo": renderLogInfo(hashfirstchars),
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

func handleTransactionList(u User, page int, cb *tgbotapi.CallbackQuery) {
	// show list of transactions
	if page == 0 {
		page = 1
	}
	limit := 25
	offset := limit * (page - 1)

	txns, err := u.listTransactions(limit, offset, 16, Both)
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
			tgbotapi.NewInlineKeyboardButtonData("newer", fmt.Sprintf("txlist=%d", page-1)),
		)
	}

	if len(txns) > 0 {
		keyboard.InlineKeyboard[0] = append(
			keyboard.InlineKeyboard[0],
			tgbotapi.NewInlineKeyboardButtonData("older", fmt.Sprintf("txlist=%d", page+1)),
		)
	}

	var chattable tgbotapi.Chattable
	if cb == nil {
		chattable = tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:      u.ChatId,
				ReplyMarkup: &keyboard,
			},
			Text: text,
			DisableWebPagePreview: true,
			ParseMode:             "HTML",
		}
	} else {
		baseEdit := getBaseEdit(cb)
		baseEdit.ReplyMarkup = &keyboard
		chattable = tgbotapi.EditMessageTextConfig{
			BaseEdit: baseEdit,
			Text:     text,
			DisableWebPagePreview: true,
			ParseMode:             "HTML",
		}
	}

	bot.Send(chattable)
}
