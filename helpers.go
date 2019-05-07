package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fiatjaf/lightningd-gjson-rpc"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/kballard/go-shellquote"
	"github.com/renstrom/fuzzysearch/fuzzy"
	"github.com/skip2/go-qrcode"
	"github.com/tidwall/gjson"
	"gopkg.in/jmcvetta/napping.v3"
)

const INVOICE_UNDEFINED_AMOUNT = -273

var dollarPrice = struct {
	lastUpdate time.Time
	rate       float64
}{time.Now(), 0}
var nodeAliases = make(map[string]string)

func makeLabel(userId int, messageId interface{}, preimage string) string {
	return fmt.Sprintf("%s.%d.%v.%s", s.ServiceId, userId, messageId, preimage)
}

func parseLabel(label string) (messageId, userId int, preimage string, ok bool) {
	ok = false
	parts := strings.Split(label, ".")
	if len(parts) == 4 {
		userId, err = strconv.Atoi(parts[1])
		if err == nil {
			ok = true
		}
		messageId, err = strconv.Atoi(parts[2])
		if err == nil {
			ok = true
		}
		preimage = parts[3]
	}
	return
}

func qrImagePath(label string) string {
	return filepath.Join(os.TempDir(), s.ServiceId+".invoice."+label+".png")
}

func searchForInvoice(message tgbotapi.Message) (bolt11 string, ok bool) {
	text := message.Text
	if text == "" {
		text = message.Caption
	}

	if bolt11, ok = getBolt11(text); ok {
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
			return
		}

		p := &url.Values{}
		p.Set("fileurl", photourl)
		var r []struct {
			Type   string `json:"type"`
			Symbol []struct {
				Data  string `json:"data"`
				Error string `json:"error"`
			} `json:"symbol"`
		}
		_, err = napping.Get("https://api.qrserver.com/v1/read-qr-code/", p, &r, nil)
		if err != nil {
			log.Warn().Err(err).Str("url", photourl).Msg("failed to call qrserver")
			return
		}
		if len(r) == 0 || len(r[0].Symbol) == 0 {
			log.Warn().Str("url", photourl).Msg("invalid response from  qrserver")
			return
		}
		if r[0].Symbol[0].Error != "" {
			log.Debug().Str("err", r[0].Symbol[0].Error).
				Str("url", photourl).Msg("qrserver failed to decode")
			return
		}

		text = r[0].Symbol[0].Data
		log.Debug().Str("data", text).Msg("got qr code data")
		return getBolt11(text)
	}

	return
}

func getBolt11(text string) (bolt11 string, ok bool) {
	text = strings.ToLower(text)

	argv, err := shellquote.Split(text)
	if err != nil {
		return
	}

	for _, arg := range argv {
		if strings.HasPrefix(arg, "lightning:") {
			arg = arg[10:]
		}

		if strings.HasPrefix(arg, "lnbc") {
			return arg, true
		}
	}

	return
}

func decodeInvoice(invoice string) (inv gjson.Result, nodeAlias, usd string, err error) {
	inv, err = ln.Call("decodepay", invoice)
	if err != nil {
		return
	}
	if inv.Get("code").Int() != 0 {
		err = errors.New(inv.Get("message").String())
		return
	}

	nodeAlias = getNodeAlias(inv.Get("payee").String())
	usd = getDollarPrice(inv.Get("msatoshi").Int())

	return
}

func getNodeAlias(id string) string {
begin:
	if alias, ok := nodeAliases[id]; ok {
		return alias
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

	nodeAliases[id] = alias
	goto begin
}

func getDollarPrice(msats int64) string {
	rate, err := getDollarRate()
	if err != nil {
		return "~ USD"
	}
	return fmt.Sprintf("%.3f USD", float64(msats)/rate)
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

func makeInvoice(
	u User,
	sats int,
	desc string,
	messageId interface{},
	preimage string,
) (bolt11 string, qrpath string, err error) {
	log.Debug().Str("user", u.Username).Str("desc", desc).Int("sats", sats).
		Msg("generating invoice")

	if preimage == "" {
		preimage, err = randomPreimage()
		if err != nil {
			return
		}
	}

	label := makeLabel(u.Id, messageId, preimage)

	var msatoshi interface{}
	if sats == INVOICE_UNDEFINED_AMOUNT {
		msatoshi = "any"
	} else {
		msatoshi = sats * 1000
	}

	// make invoice
	res, err := ln.CallWithCustomTimeout(time.Second*40, "invoice", map[string]interface{}{
		"msatoshi":    msatoshi,
		"label":       label,
		"description": desc + " [" + s.ServiceId + "/" + u.AtName() + "]",
		"expiry":      int(s.InvoiceTimeout / time.Second),
		"preimage":    preimage,
	})
	if err != nil {
		return
	}
	bolt11 = res.Get("bolt11").String()

	// generate qr code
	err = qrcode.WriteFile(strings.ToUpper(bolt11), qrcode.Medium, 256, qrImagePath(label))
	if err != nil {
		log.Warn().Err(err).Str("invoice", bolt11).
			Msg("failed to generate qr.")
		err = nil
	} else {
		qrpath = qrImagePath(label)
	}

	return
}

func messageFromError(err error, prefix string) string {
	var msg string
	switch terr := err.(type) {
	case lightning.ErrorTimeout:
		msg = fmt.Sprintf("Operation has timed out after %d seconds.", terr.Seconds)
	case lightning.ErrorCommand:
		msg = terr.Message
	case lightning.ErrorConnect, lightning.ErrorConnectionBroken:
		msg = "Problem connecting to our node. Please try again in a minute."
	case lightning.ErrorJSONDecode:
		msg = "Error reading response from lightningd."
	default:
		msg = err.Error()
	}
	return prefix + ": " + msg
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
		return
	}

	// check entities for user type
	for _, entity := range *message.Entities {
		if entity.Type == "text_mention" && entity.User != nil {
			// user without username
			uid = entity.User.ID
			display = strings.TrimSpace(entity.User.FirstName + " " + entity.User.LastName)
			user, err = ensureTelegramId(uid)
			u = &user
			return
		}
		if entity.Type == "mention" {
			// user with username
			uname := username[1:]
			display = uname
			user, err = ensureUsername(uname)
			u = &user
			return
		}
	}

	// if the user identifier passed was neither @someone (mention) nor a text_mention
	// (for users without usernames but still painted blue and autocompleted by telegram)
	// and we have a uid that means it's the case where just a numeric id was given and nothing
	// more.
	if uid != 0 {
		user, err = ensureTelegramId(uid)
		display = user.AtName()
		u = &user
		return
	}

	return
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

func nodeLink(nodeId string) string {
	return fmt.Sprintf(`<a href="https://lightning.chaintools.io/node/%s">%s…%s</a>`,
		nodeId, nodeId[:4], nodeId[len(nodeId)-4:])
}
