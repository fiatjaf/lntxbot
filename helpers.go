package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io/ioutil"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/go-lnurl"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/nfnt/resize"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/soudy/mathcat"
	"github.com/tidwall/gjson"
)

var bolt11regex = regexp.MustCompile(`.*?((lnbcrt|lntb|lnbc)([0-9]{1,}[a-z0-9]+){1})`)

var menuItems = map[string]*big.Rat{
	"msat":  big.NewRat(1, 1),
	"msats": big.NewRat(1, 1),
	"sat":   big.NewRat(1000, 1),
	"sats":  big.NewRat(1000, 1),
	"btc":   big.NewRat(100000000000, 1),

	"bear":       big.NewRat(5000, 1),
	"popcorn":    big.NewRat(27000, 1),
	"ziplock":    big.NewRat(50000, 1),
	"piparote":   big.NewRat(88000, 1),
	"coffee":     big.NewRat(525000, 1),
	"beer":       big.NewRat(525000, 1),
	"ramen":      big.NewRat(888000, 1),
	"hamster":    big.NewRat(666000, 1),
	"banana":     big.NewRat(777000, 1),
	"watermelon": big.NewRat(1214000, 1),
	"cow":        big.NewRat(3000000, 1),
	"bull":       big.NewRat(5000000, 1),
	"crown":      big.NewRat(10000000, 1),
}

func parseSatoshis(opts docopt.Opts) (msats int64, err error) {
	amt, ok := opts["<satoshis>"].(string)
	if !ok {
		return 0, errors.New("'satoshis' param missing")
	}

	msats, err = parseAmountString(amt)
	if err != nil {
		return 0, err
	}

	return msats, nil
}

func parseAmountString(amt string) (msats int64, err error) {
	defer func() {
		if r := recover(); r != nil {
			msats = 0
		}

		if err == nil && msats < 1000 {
			err = fmt.Errorf("amount too small: %dmsat", msats)
		}
	}()

	// is a number
	sats, err := strconv.ParseFloat(amt, 64)
	if err == nil {
		return int64(sats * 1000), nil
	}

	// it's an expression
	return calculate(amt)
}

func calculate(expr string) (int64, error) {
	// replace emojis
	expr = strings.ReplaceAll(expr, "🍌", "banana")
	expr = strings.ReplaceAll(expr, "🍉", "watermelon")
	expr = strings.ReplaceAll(expr, "🍿", "popcorn")
	expr = strings.ReplaceAll(expr, "🐄", "cow")
	expr = strings.ReplaceAll(expr, "🐻", "bear")
	expr = strings.ReplaceAll(expr, "☕", "coffee")
	expr = strings.ReplaceAll(expr, "🍺", "beer")
	expr = strings.ReplaceAll(expr, "🍜", "ramen")
	expr = strings.ReplaceAll(expr, "🐂", "bull")
	expr = strings.ReplaceAll(expr, "🐹", "hamster")
	expr = strings.ReplaceAll(expr, "👑", "crown")

	// lowercase
	expr = strings.ToLower(expr)

	// prepare mathcat
	p := mathcat.New()

	// add emoji values
	for k, v := range menuItems {
		p.Variables[k] = v
	}

	// add currency values
	for _, currencyCode := range CURRENCIES {
		lower := strings.ToLower(currencyCode)
		if strings.Index(expr, lower) != -1 {
			fiatMsat, err := getMsatsPerFiatUnit(currencyCode)
			if err != nil {
				return 0, err
			}
			fiatRat := new(big.Rat).SetInt64(fiatMsat)
			p.Variables[lower] = fiatRat
		}
	}
	// run mathcat
	r, err := p.Run(expr)
	if err == nil {
		f, _ := r.Float64()
		return int64(f), nil
	} else {
		return 0, fmt.Errorf("invalid math expression '%s': %w", expr, err)
	}
}

func getDollarPrice(msat int64) string {
	rate, err := getMsatsPerFiatUnit("USD")
	if err != nil {
		return "~ USD"
	}
	return fmt.Sprintf("%.2f USD", float64(msat)/float64(rate))
}

func searchForInvoice(ctx context.Context) (bolt11, lnurltext, address string, ok bool) {
	var message interface{}
	if imessage := ctx.Value("message"); imessage != nil {
		message = imessage
	} else {
		return "", "", "", false
	}

	var text string

	switch m := message.(type) {
	case *tgbotapi.Message:
		text = m.Text
		if text == "" {
			text = m.Caption
		}
	}

	if bolt11, ok = getBolt11(text); ok {
		return
	}

	if lnurltext, ok = lnurl.FindLNURLInText(text); ok {
		return
	}

	if name, domain, okW := lnurl.ParseInternetIdentifier(text); okW {
		address = name + "@" + domain
		ok = okW
		return
	}

	// receiving a picture, try to decode the qr code
	if m, tk := message.(*tgbotapi.Message); tk && m.Photo != nil && len(*m.Photo) > 0 {
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

		user := ctx.Value("initiator").(*User)
		log.Debug().Str("data", text).Stringer("user", user).
			Msg("got qr code data")
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

	return fmt.Sprintf(`<a href="https://amboss.space/node/%s">%s…%s</a>`,
		nodeId, nodeId[:4], nodeId[len(nodeId)-4:])
}

func nodeAliasLink(nodeId string) string {
	if nodeId == "" {
		return "{}"
	}

	alias := getNodeAlias(nodeId)
	if alias == "" {
		alias = fmt.Sprintf("%s…%s", nodeId[:4], nodeId[len(nodeId)-4:])
	} else if len(alias) > 16 {
		alias = alias[:15] + "…"
	}

	return fmt.Sprintf(`<a href="https://amboss.space/%s">%s</a>`, nodeId, alias)
}

func channelLink(scid string) string {
	spl := strings.Split(scid, "x")
	if len(spl) != 3 {
		return scid
	}
	block, err1 := strconv.ParseInt(spl[0], 10, 64)
	tx, err2 := strconv.ParseInt(spl[1], 10, 64)
	out, err3 := strconv.ParseInt(spl[2], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return scid
	}

	scidDecimal := ((block & 0xffffff) << 40) | ((tx & 0xffffff) << 16) | (out & 0xffff)
	return fmt.Sprintf(`<a href="https://1ml.com/channel/%d">%s</a>`, scidDecimal, scid)
}

var (
	scidRe = regexp.MustCompile(`\d+x\d+x\d+`)
	nodeRe = regexp.MustCompile(`[0-9a-f]{66}`)
)

func makeLinks(e string) string {
	for _, match := range scidRe.FindAllString(e, -1) {
		e = strings.ReplaceAll(e, match, channelLink(match))
	}
	for _, match := range nodeRe.FindAllString(e, -1) {
		e = strings.ReplaceAll(e, match, nodeAliasLink(match))
	}

	return e
}

func randomHex() (string, error) {
	data := make([]byte, 32)
	_, err := rand.Read(data)
	if err != nil {
		return "", fmt.Errorf("can't create random bytes: %w", err)
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
	} else {
		if itarget := ctx.Value("initiator"); itarget != nil {
			if target, ok := itarget.(*User); ok {
				locale = target.Locale
			}
		}
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

func imageBytesFromURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, errors.New("image returned status " + strconv.Itoa(resp.StatusCode))
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image from %s: %w", url, err)
	}

	img = resize.Resize(160, 0, img, resize.NearestNeighbor)
	out := &bytes.Buffer{}
	err = jpeg.Encode(out, img, &jpeg.Options{Quality: 50})
	if err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	return out.Bytes(), nil
}

type BalanceGetter interface {
	Get(interface{}, string, ...interface{}) error
}

func getBalance(txn BalanceGetter, userId int) int64 {
	var balance int64
	err := txn.Get(&balance, "SELECT balance::numeric(13) FROM lightning.balance WHERE account_id = $1", userId)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Warn().Err(err).Int("account", userId).Msg("failed to fetch balance")
		}
		return 0
	}
	return balance
}

func checkProxyBalance(txn BalanceGetter) error {
	// check proxy balance (should be always zero)
	var proxybalance int64
	err := txn.Get(&proxybalance, "SELECT balance::numeric(13) FROM lightning.balance WHERE account_id = $1", s.ProxyAccount)
	if err != nil {
		return err
	} else if proxybalance != 0 {
		return fmt.Errorf("proxy balance isn't 0, but %d", proxybalance)
	} else {
		return nil
	}
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

	resp, err := http.Get("https://mempool.space/api/v1/lightning/search?searchText=" + id)
	if err != nil {
		return "~"
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "~"
	}

	alias := gjson.ParseBytes(b).Get("nodes.0.alias").String()
	if alias == "" {
		alias = "~"
	}

	nodeAliases.Set(id, alias)
	goto begin
}

func savePaymentAttemptLog(hash, bolt11 string) {
	// TODO
}

func getHost() string {
	return strings.Split(s.ServiceURL, "://")[1]
}

var NameDesc = regexp.MustCompile(`^([\w_\-.@+]{1,40}):  \S`)

func extractNameFromDesc(invoiceDescription string) string {
	res := NameDesc.FindStringSubmatch(invoiceDescription)
	if len(res) == 2 {
		return res[1]
	}
	return ""
}

func senderNameFromPayerData(payer lnurl.PayerDataValues) string {
	if payer.LightningAddress != "" {
		return payer.LightningAddress
	}
	if payer.Email != "" {
		return payer.Email
	}
	if payer.FreeName != "" {
		return payer.FreeName
	}
	if payer.KeyAuth != nil {
		return payer.KeyAuth.Key
	}
	if payer.PubKey != "" {
		return payer.PubKey
	}
	return ""
}

var waitingGeneric = cmap.New() // make(map[string][]chan interface{})

func waitGeneric(key string) (inv <-chan interface{}) {
	wait := make(chan interface{})
	waitingGeneric.Upsert(key, wait,
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

func dispatchGeneric(key string, val interface{}) {
	if chans, ok := waitingGeneric.Get(key); ok {
		for _, ch := range chans.([]interface{}) {
			select {
			case ch.(chan interface{}) <- val:
			default:
			}
		}
		waitingGeneric.Remove(key)
	}
}
