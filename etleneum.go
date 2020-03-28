package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/go-rfc/sse"
	"gopkg.in/jmcvetta/napping.v3"
)

type EtleneumAppData struct {
	Account string `json:"account"`
	Secret  string `json:"secret"`
}

type EtleneumResponse struct {
	Ok    bool            `json:"ok"`
	Error string          `json:"error"`
	Value json.RawMessage `json:"value"`
}

func etleneumLogin(user User) (account, secret string, balance float64, withdraw string) {
	es, _ := sse.NewEventSource("https://etleneum.com/~~~/session")

	go func() {
		time.Sleep(10 * time.Second)
		es.Close()
	}()

	for ev := range es.MessageEvents() {
		log.Print(ev.Name)
		log.Print(ev.Data)
		var data map[string]interface{}
		json.Unmarshal([]byte(ev.Data), &data)

		if _, ok := data["auth"]; ok {
			handleLNURL(user, data["auth"].(string), handleLNURLOpts{
				loginSilently: true,
			})
			withdraw = data["withdraw"].(string)
		}
		if _, ok := data["account"]; ok {
			account = data["account"].(string)
			secret = data["secret"].(string)
			balance = data["balance"].(float64) / 1000
			es.Close()
			break
		}
	}

	go user.setAppData("etleneum", EtleneumAppData{
		account,
		secret,
	})

	return
}

func getEtleneumContractState(contractId, jqfilter string) (state string, err error) {
	var reply EtleneumResponse
	if jqfilter == "" {
		_, err = napping.Get("https://etleneum.com/~/contract/"+contractId+"/state", nil, &reply, &reply)
	} else {
		_, err = napping.Send(&napping.Request{
			Url:        "https://etleneum.com/~/contract/" + contractId + "/state",
			Method:     "POST",
			Payload:    bytes.NewBufferString(jqfilter),
			RawPayload: true,
			Result:     &reply,
			Error:      &reply,
		})
	}
	if err != nil {
		err = errors.New("etleneum.com invalid response: " + err.Error())
		return
	}
	if !reply.Ok {
		err = errors.New("etleneum.com call failed: " + reply.Error)
		return
	}

	d, err := json.MarshalIndent(reply.Value, "", "  ")
	return string(d), err
}

func buildEtleneumCallLNURL(
	user *User,
	contractId string,
	method string,
	args []string,
	sats *int,
) (string, error) {
	qs := url.Values{}

	for _, kv := range args {
		spl := strings.Split(kv, "=")
		if len(spl) != 2 {
			return "", fmt.Errorf("%s is not a valid key-value pair.", kv)
		}

		v := strings.TrimSpace(spl[1])

		// if kv is like "user=@fiatjaf" we will translate "@fiatjaf" into the
		// actual etleneum account for @fiatjaf
		if strings.HasPrefix(v, "@") && strings.Index(v, " ") == -1 {
			v, err = translateToEtleneumAccount(v)
			if err != nil {
				return "", err
			}
		}

		qs.Set(strings.TrimSpace(spl[0]), v)
	}

	if user != nil {
		var userdata EtleneumAppData
		err := user.getAppData("etleneum", &userdata)
		if err != nil {
			return "", err
		}

		mac := etleneumHmacCall(userdata.Secret, contractId, method, qs, sats)
		qs.Set("_account", userdata.Account)
		qs.Set("_hmac", mac)
	}

	amount := ""
	if sats != nil {
		amount = fmt.Sprintf("/%d", *sats*1000)
	}

	return fmt.Sprintf("https://etleneum.com/lnurl/contract/%s/call/%s%s?%s",
		contractId, method, amount, qs.Encode()), nil
}

func etleneumHmacCall(secret, ctid, method string, args url.Values, sats *int) string {
	msatoshi := 0
	if sats != nil {
		msatoshi = *sats * 1000
	}

	res := fmt.Sprintf("%s:%s:%d,", ctid, method, msatoshi)

	// sort payload keys
	keys := make([]string, len(args))
	i := 0
	for k, _ := range args {
		keys[i] = k
		i++
	}
	sort.Strings(keys)

	// add key-values
	for _, k := range keys {
		v := args.Get(k)
		res += fmt.Sprintf("%s=%v", k, v)
		res += ","
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(res))
	return hex.EncodeToString(mac.Sum(nil))
}

func translateToEtleneumAccount(username string) (accountId string, err error) {
	user, err := ensureUsername(username[1:])
	if err != nil {
		return
	}

	var userdata EtleneumAppData
	err = user.getAppData("etleneum", &userdata)
	if err != nil {
		return
	}

	if userdata.Account != "" {
		return userdata.Account, nil
	} else {
		// create etleneum account for this user now and who cares
		account, _, _, _ := etleneumLogin(user)
		return account, nil
	}
}
