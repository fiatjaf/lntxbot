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
	"image"
	"image/jpeg"
	"io/ioutil"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/go-lnurl"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jmoiron/sqlx"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/nfnt/resize"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/soudy/mathcat"
	"github.com/tidwall/gjson"
)

var bolt11regex = regexp.MustCompile(`.*?((lnbcrt|lntb|lnbc)([0-9]{1,}[a-z0-9]+){1})`)

var menuItems = map[string]*big.Rat{
	"bear":       big.NewRat(5000, 1),
	"popcorn":    big.NewRat(27000, 1),
	"ziplock":    big.NewRat(50000, 1),
	"piparote":   big.NewRat(88000, 1),
	"coffee":     big.NewRat(525000, 1),
	"hamster":    big.NewRat(666000, 1),
	"banana":     big.NewRat(777000, 1),
	"watermelon": big.NewRat(1214000, 1),
	"cow":        big.NewRat(3000000, 1),
	"bull":       big.NewRat(5000000, 1),
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
		if err == nil && msats < 1000 {
			err = fmt.Errorf("amount too small: %dmsat", msats)
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
	amt = strings.ReplaceAll(amt, "🐄", "cow")
	amt = strings.ReplaceAll(amt, "🐻", "bear")
	amt = strings.ReplaceAll(amt, "☕", "coffee")
	amt = strings.ReplaceAll(amt, "🐂", "bull")
	amt = strings.ReplaceAll(amt, "🐹", "hamster")

	// lowercase
	amt = strings.ToLower(amt)

	// prepare mathcat
	p := mathcat.New()

	// add emoji values
	for k, v := range menuItems {
		p.Variables[k] = v
	}

	// add currency values
	for _, currencyCode := range CURRENCIES {
		lower := strings.ToLower(currencyCode)
		if strings.Index(amt, lower) != -1 {
			fiatMsat, err := getMsatsPerFiatUnit(currencyCode)
			if err != nil {
				return 0, err
			}
			fiatRat := new(big.Rat).SetInt64(fiatMsat)
			p.Variables[lower] = fiatRat
		}
	}

	// run mathcat
	r, err := p.Run(amt)
	if err == nil {
		f, _ := r.Float64()
		if f < 1000 {
			return 0, errors.New("'satoshis' param invalid")
		}
		return int64(f), nil
	} else {
		return 0, fmt.Errorf("invalid math expression '%s': %w", amt, err)
	}
}

func getDollarPrice(msat int64) string {
	rate, err := getMsatsPerFiatUnit("USD")
	if err != nil {
		return "~ USD"
	}
	return fmt.Sprintf("%.2f USD", float64(msat)/float64(rate))
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

	return fmt.Sprintf(`<a href="http://ln.fiatjaf.com/%s">%s…%s</a>`,
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

	return fmt.Sprintf(`<a href="http://ln.fiatjaf.com/%s">%s</a>`,
		nodeIdShortened, alias)
}

func channelLink(scid string) string {
	return fmt.Sprintf(`<a href="http://ln.fiatjaf.com/%s">%s</a>`, scid, scid)
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

func checkProxyBalance(txn *sqlx.Tx) error {
	// check proxy balance (should be always zero)
	var proxybalance int64
	err := txn.Get(&proxybalance, `
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

func base64ImageFromURL(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", errors.New("image returned status " + strconv.Itoa(resp.StatusCode))
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to decode image from %s: %w", url, err)
	}

	img = resize.Resize(160, 0, img, resize.NearestNeighbor)
	out := &bytes.Buffer{}
	err = jpeg.Encode(out, img, &jpeg.Options{50})
	if err != nil {
		return "", fmt.Errorf("failed to encode image: %w", err)
	}

	return base64.StdEncoding.EncodeToString(out.Bytes()), nil
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

var nodeAliases = cmap.New()

func getNodeAlias(id string) string {
begin:
	if alias, ok := nodeAliases.Get(id); ok {
		return alias.(string)
	}

	if id == "" {
		return "~"
	}

	resp, err := http.Get("https://ln.fiatjaf.com/nodes?select=alias&pubkey=eq." + id)
	if err != nil {
		return "~"
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "~"
	}

	alias := gjson.ParseBytes(b).Get("0.alias").String()
	if alias == "" {
		alias = "~"
	}

	nodeAliases.Set(id, alias)
	goto begin
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

func savePaymentAttemptLog(hash, bolt11 string) {
	// TODO
}

func getHost(r *http.Request) string {
	if host := r.Header.Get("X-Forwarded-Host"); host != "" {
		return host
	} else {
		return r.Host
	}
}

var NameDesc = regexp.MustCompile(`^([\w_\-.@+]{1,40}):  \w`)

func extractNameFromDesc(invoiceDescription string) string {
	res := NameDesc.FindStringSubmatch(invoiceDescription)
	if len(res) == 2 {
		return res[1]
	}
	return ""
}
