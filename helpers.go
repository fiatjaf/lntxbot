package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	mrand "math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/go-lnurl"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jmoiron/sqlx"
	"github.com/renstrom/fuzzysearch/fuzzy"
	"github.com/tidwall/gjson"
)

var bolt11regex = regexp.MustCompile(`.*?((lnbcrt|lntb|lnbc)([0-9]{1,}[a-z0-9]+){1})`)

var dollarPrice = struct {
	lastUpdate time.Time
	rate       float64
}{time.Now(), 0}

func makeLabel(userId int, messageId interface{}, tag string) string {
	if messageId == nil {
		// this is a component of the label, so must be unique
		// if not given we use a random number
		messageId = mrand.Intn(1000)
	}

	return fmt.Sprintf("%s.%d.%v.%s", s.ServiceId, userId, messageId, tag)
}

func parseLabel(label string) (messageId, userId int, tag string, ok bool) {
	ok = false
	parts := strings.Split(label, ".")
	if len(parts) > 2 {
		userId, err = strconv.Atoi(parts[1])
		if err == nil {
			ok = true
		}
		messageId, err = strconv.Atoi(parts[2])
		if err == nil {
			ok = true
		}
	}

	if len(parts) > 3 {
		tag = parts[3]
	}

	return
}

func chatOwnerFromTicketLabel(label string) (owner User, err error) {
	parts := strings.Split(label, ":")
	chatId, err := strconv.Atoi(parts[2])
	if err != nil {
		log.Error().Err(err).Str("label", label).Msg("failed to parse ticket invoice")
		return
	}

	owner, err = getChatOwner(int64(chatId))
	if err != nil {
		log.Error().Err(err).Str("label", label).Msg("failed to get chat owner in ticket invoice handling")
		return
	}

	return
}

func searchForInvoice(u User, message tgbotapi.Message) (bolt11, lnurltext string, ok bool) {
	text := message.Text
	if text == "" {
		text = message.Caption
	}

	if bolt11, ok = getBolt11(text); ok {
		return
	}

	if lnurltext, ok = lnurl.FindLNURLInText(text); ok {
		return
	}

	// receiving a picture, try to decode the qr code
	if message.Photo != nil && len(*message.Photo) > 0 {
		log.Debug().Msg("got photo, looking for qr code.")

		photos := *message.Photo
		photo := photos[len(photos)-1]

		photourl, err := bot.GetFileDirectURL(photo.FileID)
		if err != nil {
			log.Warn().Err(err).Str("fileid", photo.FileID).
				Msg("failed to get photo URL.")
			u.notifyAsReply(t.QRCODEFAIL, t.T{"Err": err.Error()}, message.MessageID)
			return
		}

		text, err := decodeQR(photourl)
		if err != nil {
			u.notifyAsReply(t.QRCODEFAIL, t.T{"Err": err.Error()}, message.MessageID)
			return
		}

		log.Debug().Str("data", text).Msg("got qr code data")
		sendMessage(u.ChatId, text)

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

	return fmt.Sprintf(`<a href="http://ln.bigsun.xyz/%s">%s…%s</a>`,
		nodeId[:10], nodeId[:4], nodeId[len(nodeId)-4:])
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

	return fmt.Sprintf(`<a href="http://ln.bigsun.xyz/%s">%s</a>`,
		nodeId[:10], alias)
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
	hex := []rune("0123456789abcdef")
	b := make([]rune, 64)
	for i := range b {
		r, err := rand.Int(rand.Reader, big.NewInt(16))
		if err != nil {
			return "", err
		}
		b[i] = hex[r.Int64()]
	}
	return string(b), nil
}

func calculateHash(data string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(data)))
}

func parseUsername(message *tgbotapi.Message, value interface{}) (u *User, display string, err error) {
	var username string
	var user User
	var uid int

	switch val := value.(type) {
	case []string:
		if len(val) > 0 {
			username = strings.Join(val, " ")
		}
	case string:
		username = val
	case int:
		uid = val
	}

	if intval, err := strconv.Atoi(username); err == nil {
		uid = intval
	}

	if username != "" {
		username = strings.ToLower(username)
	}

	if username == "" && uid == 0 {
		return nil, "", errors.New("no user")
	}

	// check entities for user type
	if message.Entities != nil {
		for _, entity := range *message.Entities {
			if entity.Type == "text_mention" && entity.User != nil {
				// user without username
				uid = entity.User.ID
				display = strings.TrimSpace(entity.User.FirstName + " " + entity.User.LastName)
				user, err = ensureTelegramId(uid)
				if err != nil {
					return nil, "", err
				}

				return &user, display, nil
			}
			if entity.Type == "mention" {
				// user with username
				uname := username[1:]
				display = "@" + uname
				user, err = ensureUsername(uname)
				if err != nil {
					return nil, "", err
				}

				return &user, display, nil
			}
		}
	}

	// if the user identifier passed was neither @someone (mention) nor a text_mention
	// (for users without usernames but still painted blue and autocompleted by telegram)
	// and we have a uid that means it's the case where just a numeric id was given and nothing
	// more.
	if uid != 0 {
		user, err = ensureTelegramId(uid)
		if err != nil {
			return nil, "", err
		}

		return &user, user.AtName(), nil
	}

	return nil, "", errors.New("no user")
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

func translate(key t.Key, locale string) string {
	return translateTemplate(key, locale, nil)
}

func translateTemplate(key t.Key, locale string, data t.T) string {
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

func getVariadicFieldOrReplyToContent(opts docopt.Opts, message *tgbotapi.Message, optsField string) string {
	if imessage, ok := opts[optsField]; ok {
		return strings.Join(imessage.([]string), " ")
	} else if message.ReplyToMessage != nil {
		return message.ReplyToMessage.Text
	} else {
		return ""
	}
}

func waitInvoice(hash string) (inv <-chan gjson.Result) {
	wait := make(chan gjson.Result)
	waitingInvoices.Upsert(hash, wait,
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

func resolveWaitingInvoice(hash string, inv gjson.Result) {
	if chans, ok := waitingInvoices.Get(hash); ok {
		for _, ch := range chans.([]interface{}) {
			select {
			case ch.(chan gjson.Result) <- inv:
			default:
			}
		}
		waitingInvoices.Remove(hash)
	}
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
