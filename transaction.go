package main

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/eclair-go"
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
	case "expensive":
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
	Peer    string
	Channel string
}

func renderLogInfo(ctx context.Context, hash string, showHash bool) (logInfo string) {
	info, err := ln.Call("getsentinfo", eclair.Params{"paymentHash": hash})
	if err != nil {
		return translateTemplate(ctx, t.ERROR, t.T{"Err": err})
	}

	if info.Get("#").Int() == 0 {
		return translateTemplate(ctx, t.ERROR, t.T{"Err": "payment not attempted"})
	}

	tries := make([]Try, info.Get("#").Int())
	for i, attempt := range info.Array() {
		route := make([]Hop, attempt.Get("route.#").Int())

		for r, hop := range attempt.Get("route").Array() {
			route[r] = Hop{
				Peer:    hop.Get("nextNodeId").String(),
				Channel: hop.Get("shortChannelId").String(),
			}
		}

		tries[i] = Try{
			Success: attempt.Get("status.type").String() == "sent",
			Error:   attempt.Get("status.failures.0.failureMessage").String(),
			Route:   route,
		}
	}

	params := t.T{"Tries": tries}
	if showHash {
		params["PaymentHash"] = hash
	}

	return translateTemplate(ctx, t.TXLOG, params)
}

func handleSingleTransaction(ctx context.Context, opts docopt.Opts) {
	u := ctx.Value("initiator").(User)

	// individual transaction query
	hashfirstchars := opts["<hash>"].(string)
	if len(hashfirstchars) < 5 {
		send(ctx, t.ERROR, t.T{"Err": "hash too small."})
		return
	}
	go u.track("view tx", nil)

	txn, err := u.getTransaction(hashfirstchars)
	if err != nil {
		log.Warn().Err(err).Str("user", u.Username).Str("hash", hashfirstchars).
			Msg("failed to get transaction")
		send(ctx, u, t.TXNOTFOUND, t.T{"HashFirstChars": hashfirstchars},
			ctx.Value("message"))
		return
	}

	logInfo := ""
	if txn.Payee.Valid {
		logInfo = renderLogInfo(ctx, txn.Hash, false)
	}

	text := translateTemplate(ctx, t.TXINFO, t.T{
		"Txn":     txn,
		"LogInfo": logInfo,
	})

	var actionPrompt interface{}
	if txn.IsUnclaimed() {
		text = text + "\n\n" + translate(ctx, t.RETRACTQUESTION)
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(translate(ctx, t.YES),
					"remunc="+hashfirstchars),
			),
		)
		actionPrompt = &keyboard
	}
	send(ctx, text, txn.TriggerMessage, actionPrompt)
}

func handleTransactionList(ctx context.Context, opts docopt.Opts) {
	page, _ := opts.Int("--page")
	filter := Both
	if opts["--in"].(bool) {
		filter = In
	} else if opts["--out"].(bool) {
		filter = Out
	}
	tag, _ := opts.String("<tag>")

	displayTransactionList(ctx, page, tag, filter)
}

func displayTransactionList(ctx context.Context, page int, tag string, filter InOut) {
	u := ctx.Value("initiator").(User)

	// show list of transactions
	if page == 0 {
		page = 1
	}

	go u.track("txlist", map[string]interface{}{
		"filter": filter,
		"tag":    tag,
		"page":   page,
	})

	limit := 25
	offset := limit * (page - 1)

	txns, err := u.listTransactions(limit, offset, 16, tag, filter)
	if err != nil {
		log.Warn().Err(err).Str("user", u.Username).Int("page", page).
			Msg("failed to list transactions")
		return
	}

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

	send(ctx, EDIT, &keyboard, t.TXLIST, t.T{
		"Offset":       offset,
		"Limit":        limit,
		"From":         offset + 1,
		"To":           offset + limit,
		"Transactions": txns,
	})
}

func handleLogView(ctx context.Context, opts docopt.Opts) {
	// query failed transactions (only available in the first 24h after the failure)
	u := ctx.Value("initiator").(User)
	hashfirstchars := opts["<hash>"].(string)
	if len(hashfirstchars) < 5 {
		send(ctx, u, t.ERROR, t.T{"Err": "hash too small."})
		return
	}
	go u.track("view log", nil)

	hash := hashfirstchars
	if len(hash) == 64 {
		// continue
	} else if txn, err := u.getTransaction(hashfirstchars); err == nil {
		hash = txn.Hash
	} else {
		hash, err = rds.Get("hash:" + strconv.Itoa(u.Id) + ":" + hashfirstchars).Result()
		if err != nil {
			send(ctx, u, t.TXNOTFOUND, t.T{"HashFirstChars": hashfirstchars},
				ctx.Value("message"))
			return
		}
	}

	send(ctx, u, renderLogInfo(ctx, hash, true))
}

func checkAllOutgoingPayments(ctx context.Context) {
	var hashes []string
	err := pg.Select(&hashes,
		"SELECT payment_hash FROM lightning.transaction WHERE pending AND to_id IS NULL")
	if err == sql.ErrNoRows {
		err = nil
	}
	if err != nil {
		log.Error().Err(err).Msg("failed to get all pending outgoing payment hashes")
		return
	}

	log.Debug().Int("n", len(hashes)).Msg("checking pending outgoing payments")
	for _, hash := range hashes {
		log.Debug().Str("hash", hash).Msg("checking outgoing")
		checkOutgoingPayment(ctx, hash)
	}
}
