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

	"github.com/donovanhide/eventsource"
	"gopkg.in/jmcvetta/napping.v3"
)

const (
	ALIAS_CONTRACT        string = "c7c491sw04"
	ALIAS_DEFAULT_ACCOUNT string = "ay81i7dw7"
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

type EtleneumContract struct {
	Id     string `json:"id"`
	Name   string `json:"name"`
	Funds  int64  `json:"funds"`
	NCalls int    `json:"ncalls"`
	Readme string `json:"readme"`
	Code   string `json:"code"`
}

func listEtleneumContracts(user User) (contracts []EtleneumContract, aliases map[string]string, err error) {
	// list contracts
	var reply EtleneumResponse
	_, err = napping.Get("https://etleneum.com/~/contracts", nil, &reply, &reply)
	if err != nil {
		err = errors.New("etleneum.com invalid response: " + reply.Error)
		return
	}
	if !reply.Ok {
		err = errors.New("etleneum.com call failed: " + reply.Error)
		return
	}
	err = json.Unmarshal(reply.Value, &contracts)
	if err != nil {
		err = fmt.Errorf("failed to decode contracts JSON: %s", err.Error())
		return
	}

	// get aliases
	var userdata EtleneumAppData
	user.getAppData("etleneum", &userdata)
	accountId := userdata.Account
	if accountId == "" {
		accountId = "bogus"
	}

	state, _ := getEtleneumContractState(ALIAS_CONTRACT,
		` .`+ALIAS_DEFAULT_ACCOUNT+` * (.`+accountId+` // {})
        | to_entries
        | map(._ = .value | .value = .key | .key = ._)
        | from_entries
        `)
	err = json.Unmarshal(state, &aliases)
	if err != nil {
		err = fmt.Errorf("failed to decode aliases JSON: %s", err.Error())
		return
	}

	return
}

func aliasToEtleneumContractId(user User, aliasOrId string) (id string) {
	var userdata EtleneumAppData
	user.getAppData("etleneum", &userdata)
	accountId := userdata.Account
	if accountId == "" {
		accountId = "bogus"
	}

	state, _ := getEtleneumContractState(ALIAS_CONTRACT,
		` .`+ALIAS_DEFAULT_ACCOUNT+` * (.`+accountId+` // {})
        | .["`+aliasOrId+`"] // "`+aliasOrId+`"
        `)
	json.Unmarshal(state, &id)

	if id == "" {
		id = aliasOrId
	}

	return
}

func etleneumLogin(user User) (account, secret string, balance float64, withdraw string, err error) {
	es, err := eventsource.Subscribe("https://etleneum.com/~~~/session", "")
	if err != nil {
		return
	}

	go func() {
		for err := range es.Errors {
			log.Debug().Err(err).Msg("eventsource error")
		}
	}()

	go func() {
		time.Sleep(10 * time.Second)
		es.Close()
	}()

	for ev := range es.Events {
		var data map[string]interface{}
		json.Unmarshal([]byte(ev.Data()), &data)

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

	if account == "" {
		err = errors.New("etleneum.com authorization timed out")
		return
	}

	go user.setAppData("etleneum", EtleneumAppData{
		account,
		secret,
	})

	return
}

func getEtleneumContractMetadata(contractId string) (ct EtleneumContract, err error) {
	var reply EtleneumResponse
	_, err = napping.Get("https://etleneum.com/~/contracts?id="+contractId, nil, &reply, &reply)
	if err != nil {
		err = errors.New("etleneum.com invalid response: " + reply.Error)
		return
	}
	if !reply.Ok {
		err = errors.New("etleneum.com call failed: " + reply.Error)
		return
	}

	var arr []EtleneumContract
	err = json.Unmarshal(reply.Value, &arr)
	if err != nil {
		return
	}
	if len(arr) != 1 {
		err = errors.New("not found")
		return
	}

	return arr[0], err
}

func getEtleneumContractState(contractId, jqfilter string) (state json.RawMessage, err error) {
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
		err = errors.New("etleneum.com invalid response: " + reply.Error)
		return
	}
	if !reply.Ok {
		err = errors.New("etleneum.com call failed: " + reply.Error)
		return
	}

	return reply.Value, err
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
		account, _, _, _, err := etleneumLogin(user)
		if err != nil {
			return "", err
		}

		return account, nil
	}
}
