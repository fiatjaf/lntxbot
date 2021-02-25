package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/go-lnurl"
	lightning "github.com/fiatjaf/lightningd-gjson-rpc"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jmoiron/sqlx"
	"github.com/lithammer/fuzzysearch/fuzzy"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/soudy/mathcat"
	"github.com/tidwall/gjson"
)

var bolt11regex = regexp.MustCompile(`.*?((lnbcrt|lntb|lnbc)([0-9]{1,}[a-z0-9]+){1})`)

var dollarPrice = struct {
	lastUpdate time.Time
	rate       float64
}{time.Now(), 0}

var menuItems = map[string]*big.Rat{
	"popcorn":    big.NewRat(27, 1),
	"ziplock":    big.NewRat(50, 1),
	"piparote":   big.NewRat(88, 1),
	"banana":     big.NewRat(777, 1),
	"watermelon": big.NewRat(1214, 1),
}

func parseSatoshis(opts docopt.Opts) (msats int64, err error) {
	amt, ok := opts["<satoshis>"].(string)
	if !ok {
		return 0, errors.New("'satoshis' param missing")
	}

	return parseAmountString(amt)
}

func parseAmountString(amt string) (msats int64, err error) {
	defer func() {
		if msats < 1000 {
			err = errors.New("too small")
		}
	}()

	// is a number
	sats, err := strconv.ParseFloat(amt, 64)
	if err == nil {
		return int64(sats * 1000), nil
	}

	// replace emojis
	amt = strings.ReplaceAll(amt, "🍌", "banana")
	amt = strings.ReplaceAll(amt, "🍉", "watermelon")
	amt = strings.ReplaceAll(amt, "🍿", "popcorn")

	// usd
	usdMsat, err := getDollarRate()
	if err != nil {
		return 0, errors.New("couldn't get exchange rate")
	}

	// is an expression
	p := mathcat.New()
	for k, v := range menuItems {
		p.Variables[k] = v
	}
	usdRat := new(big.Rat).SetFloat64(usdMsat)
	usdRat.Mul(usdRat, big.NewRat(1, 1000))
	p.Variables["USD"] = usdRat
	p.Variables["usd"] = usdRat
	r, err := p.Run(amt)
	if err == nil {
		f, _ := r.Float64()
		if f < 100000 {
			return int64(f * 1000), nil
		}
	} else {
		log.Debug().Err(err).Str("expr", amt).Msg("invalid amt math expression")
	}

	// is a fiat currency
	if strings.HasSuffix(strings.ToUpper(amt), "USD") {
		dollars, err := strconv.ParseFloat(amt[0:len(amt)-3], 64)
		if err == nil {
			msats := int64(dollars * usdMsat)
			return msats, nil
		}
	}

	return 0, errors.New("'satoshis' param invalid")
}

func searchForInvoice(ctx context.Context) (bolt11, lnurltext string, ok bool) {
	var message interface{}
	if imessage := ctx.Value("message"); imessage != nil {
		message = imessage
	} else {
		return "", "", false
	}

	var text string

	switch m := message.(type) {
	case *tgbotapi.Message:
		text = m.Text
		if text == "" {
			text = m.Caption
		}
	case *discordgo.Message:
		text = m.Content
	}

	if bolt11, ok = getBolt11(text); ok {
		return
	}

	if lnurltext, ok = lnurl.FindLNURLInText(text); ok {
		return
	}

	// receiving a picture, try to decode the qr code
	if m, tk := message.(tgbotapi.Message); tk && m.Photo != nil && len(*m.Photo) > 0 {
		log.Debug().Msg("got photo, looking for qr code.")

		photos := *m.Photo
		photo := photos[len(photos)-1]

		photourl, err := bot.GetFileDirectURL(photo.FileID)
		if err != nil {
			log.Warn().Err(err).Str("fileid", photo.FileID).
				Msg("failed to get photo URL.")
			send(ctx, t.QRCODEFAIL, t.T{"Err": err.Error()}, m.MessageID)
			return
		}

		text, err := decodeQR(photourl)
		if err != nil {
			send(ctx, t.QRCODEFAIL, t.T{"Err": err.Error()}, m.MessageID)
			return
		}

		log.Debug().Str("data", text).Msg("got qr code data")
		send(ctx, text)

		if bolt11, ok = getBolt11(text); ok {
			return
		}

		if lnurltext, ok = lnurl.FindLNURLInText(text); ok {
			return
		}
	}

	return
}

func getBolt11(text string) (bolt11 string, ok bool) {
	text = strings.ToLower(text)
	results := bolt11regex.FindStringSubmatch(text)

	if len(results) == 0 {
		return
	}

	return results[1], true
}

func nodeLink(nodeId string) string {
	if nodeId == "" {
		return "{}"
	}

	if getNodeAlias(nodeId) == "~" {
		// if there's no node alias we assume this isn't a public node
		// and thus show its id fully and no links
		return `<code>` + nodeId + `</code>`
	}

	return fmt.Sprintf(`<a href="http://ln.bigsun.xyz/%s">%s…%s</a>`,
		nodeId, nodeId[:4], nodeId[len(nodeId)-4:])
}

func nodeAliasLink(nodeId string) string {
	if nodeId == "" {
		return "{}"
	}

	nodeIdShortened := nodeId[:10]
	alias := getNodeAlias(nodeId)
	if alias == "" {
		alias = fmt.Sprintf("%s…%s", nodeId[:4], nodeId[len(nodeId)-4:])
		nodeIdShortened = nodeId
	} else if len(alias) > 16 {
		alias = alias[:15] + "…"
	}

	return fmt.Sprintf(`<a href="http://ln.bigsun.xyz/%s">%s</a>`,
		nodeIdShortened, alias)
}

func channelLink(scid string) string {
	return fmt.Sprintf(`<a href="http://ln.bigsun.xyz/%s">%s</a>`, scid, scid)
}

var scidRe = regexp.MustCompile(`\d+x\d+x\d+`)
var nodeRe = regexp.MustCompile(`[0-9a-f]{66}`)

func makeLinks(e string) string {
	for _, match := range scidRe.FindAllString(e, -1) {
		e = strings.ReplaceAll(e, match, channelLink(match))
	}
	for _, match := range nodeRe.FindAllString(e, -1) {
		e = strings.ReplaceAll(e, match, nodeAliasLink(match))
	}

	return e
}

func getDollarPrice(msats int64) string {
	rate, err := getDollarRate()
	if err != nil {
		return "~ USD"
	}
	return fmt.Sprintf("%.2f USD", float64(msats)/rate)
}

func getDollarRate() (rate float64, err error) {
begin:
	if dollarPrice.rate > 0 && dollarPrice.lastUpdate.After(time.Now().Add(-time.Hour)) {
		// it's fine
		return dollarPrice.rate, nil
	}

	resp, err := http.Get("https://www.bitstamp.net/api/v2/ticker/btcusd")
	if err != nil || resp.StatusCode >= 300 {
		return
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	srate := gjson.GetBytes(b, "last").String()
	btcrate, err := strconv.ParseFloat(srate, 64)
	if err != nil {
		return
	}

	// we want the msat -> dollar rate, not dollar -> btc
	dollarPrice.rate = 1 / (btcrate / 100000000000)
	dollarPrice.lastUpdate = time.Now()
	goto begin
}

func randomPreimage() (string, error) {
	data := make([]byte, 32)
	_, err := rand.Read(data)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func hashString(format string, a ...interface{}) string {
	str := fmt.Sprintf(format, a...)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(str)))
}

func findSimilar(source string, targets []string) (result []string) {
	var (
		first  []string
		second []string
		third  []string
		fourth []string
	)

	for _, target := range targets {
		if fuzzy.Match(source, target) {
			first = append(first, target)
			continue
		}

		score := fuzzy.LevenshteinDistance(source, target)
		if score < 1 {
			second = append(result, target)
			continue
		}
		if score < 2 {
			third = append(result, target)
			continue
		}
		if score < 3 {
			fourth = append(result, target)
			continue
		}
	}

	res := first
	res = append(first, second...)
	res = append(res, third...)
	res = append(res, fourth...)

	return res
}

func roman(number int) string {
	conversions := []struct {
		value int
		digit string
	}{
		{1000, "M"},
		{900, "CM"},
		{500, "D"},
		{400, "CD"},
		{100, "C"},
		{90, "XC"},
		{50, "L"},
		{40, "XL"},
		{10, "X"},
		{9, "IX"},
		{5, "V"},
		{4, "IV"},
		{1, "I"},
	}

	roman := ""
	for _, conversion := range conversions {
		for number >= conversion.value {
			roman += conversion.digit
			number -= conversion.value
		}
	}
	return roman
}

func translate(ctx context.Context, key t.Key) string {
	return translateTemplate(ctx, key, nil)
}

func translateTemplate(ctx context.Context, key t.Key, data t.T) string {
	iloc := ctx.Value("locale")
	var locale string = "en"
	if iloc != nil {
		locale, _ = iloc.(string)
	}

	msg, err := bundle.Render(locale, key, data)

	if err != nil {
		log.Error().Err(err).Str("locale", locale).Str("key", string(key)).
			Msg("translation failed")
	}

	return msg
}

func escapeHTML(m string) string {
	return strings.Replace(
		strings.Replace(
			strings.Replace(
				strings.Replace(
					m,
					"&", "&amp;", -1),
				"<", "&lt;", -1),
			">", "&gt;", -1),
		"\"", "&quot;", -1)
}

func stringIsIn(needle string, haystack []string) bool {
	for _, str := range haystack {
		if str == needle {
			return true
		}
	}
	return false
}

func getVariadicFieldOrReplyToContent(ctx context.Context, opts docopt.Opts, optsField string) string {
	if text, ok := opts[optsField]; ok {
		return strings.Join(text.([]string), " ")
	}

	if imessage := ctx.Value("message"); imessage != nil {
		if message, ok := imessage.(*tgbotapi.Message); ok {
			if message.ReplyToMessage != nil {
				return message.ReplyToMessage.Text
			}
		}
	}

	return ""
}

func waitPaymentSuccess(hash string) (preimage <-chan string) {
	wait := make(chan string)
	waitingPaymentSuccesses.Upsert(hash, wait,
		func(exists bool, arr interface{}, v interface{}) interface{} {
			if exists {
				return append(arr.([]interface{}), v)
			} else {
				return []interface{}{v}
			}
		},
	)
	return wait
}

func resolveWaitingPaymentSuccess(hash string, preimage string) {
	if chans, ok := waitingPaymentSuccesses.Get(hash); ok {
		for _, ch := range chans.([]interface{}) {
			select {
			case ch.(chan string) <- preimage:
			default:
			}
		}
		waitingPaymentSuccesses.Remove(hash)
	}
}

func checkProxyBalance(txn *sqlx.Tx) error {
	// check proxy balance (should be always zero)
	var proxybalance int64
	err = txn.Get(&proxybalance, `
SELECT (coalesce(sum(amount), 0) - coalesce(sum(fees), 0))::numeric(13) AS balance
FROM lightning.account_txn
WHERE account_id = $1
    `, s.ProxyAccount)
	if err != nil {
		return err
	} else if proxybalance != 0 {
		return errors.New("proxy balance isn't 0")
	} else {
		return nil
	}
}

func base64FileFromURL(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", errors.New("image returned status " + strconv.Itoa(resp.StatusCode))
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), nil
}

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

func zipdata(filename string, content []byte) (zipped []byte, err error) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	f, err := w.Create(filename)
	if err != nil {
		return
	}
	_, err = f.Write(content)
	if err != nil {
		return
	}

	err = w.Close()
	if err != nil {
		return
	}

	return buf.Bytes(), nil
}
