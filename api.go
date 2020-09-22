package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/lntxbot/t"
	"github.com/gorilla/mux"
)

type Permission int

const (
	FullPermissions     Permission = 10
	InvoicePermissions             = 7
	ReadOnlyPermissions            = 3
)

// for now the API is a superset of bluewallet/lndhub APIs, most basic methods are there
// maybe later we'll have a better API

func registerAPIMethods() {
	registerBluewalletMethods()

	router.Path("/generatelnurlwithdraw").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, permission, err := loadUserFromAPICall(r)
		if err != nil {
			errorBadAuth(w)
			return
		}
		if permission < FullPermissions {
			errorInsufficientPermissions(w)
			return
		}

		var params struct {
			Satoshis string `json:"satoshis"`
		}
		err = json.NewDecoder(r.Body).Decode(&params)
		if err != nil {
			errorInvalidParams(w)
			return
		}
		sats, err := strconv.Atoi(params.Satoshis)
		if err != nil {
			errorInvalidParams(w)
			return
		}

		lnurlEncoded := handleCreateLNURLWithdraw(user, docopt.Opts{
			"<satoshis>": sats,
		}, -rand.Int())
		if lnurlEncoded == "" {
			errorInvalidParams(w)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			LNURL string `json:"lnurl"`
		}{lnurlEncoded})
	})

	router.Path("/invoicestatus/{hash}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, permission, err := loadUserFromAPICall(r)
		if err != nil {
			errorBadAuth(w)
			return
		}
		if permission < ReadOnlyPermissions {
			errorInsufficientPermissions(w)
			return
		}

		hash := mux.Vars(r)["hash"]
		if len(hash) != 64 {
			errorInvalidParams(w)
			return
		}

		txn, err := user.getTransaction(hash)
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"hash":     txn.Hash,
				"preimage": txn.Preimage.String,
				"amount":   txn.Amount,
			})
			return
		}

		// transaction not found, so let's wait a while for it
		if r.URL.Query().Get("wait") == "false" {
			errorTimeout(w)
			return
		}

		select {
		case inv := <-waitInvoice(hash):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"hash":     inv.PaymentHash,
				"preimage": inv.Preimage,
				"amount":   inv.MSatoshi,
			})
			return
		case <-time.After(180 * time.Second):
			errorTimeout(w)
			return
		}
	})

	router.Path("/paymentstatus/{hash}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, permission, err := loadUserFromAPICall(r)
		if err != nil {
			errorBadAuth(w)
			return
		}
		if permission < ReadOnlyPermissions {
			errorInsufficientPermissions(w)
			return
		}

		hash := mux.Vars(r)["hash"]
		if len(hash) != 64 {
			errorInvalidParams(w)
			return
		}

		status := "unknown"
		txn, err := user.getTransaction(hash)
		if err != nil {
			if err == sql.ErrNoRows {
				status = "failed"
			} else {
				errorInvalidParams(w)
				return
			}
		}

		if txn.Status == "PENDING" {
			status = "pending"
		} else if txn.Status == "SENT" {
			status = "complete"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"hash":   txn.Hash,
			"status": status,
		})
		return
	})
}

func loadUserFromAPICall(r *http.Request) (user User, permission Permission, err error) {
	// decode user id and password from auth token
	splt := strings.Split(strings.TrimSpace(r.Header.Get("Authorization")), " ")
	token := splt[len(splt)-1]
	res, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return
	}
	parts := strings.Split(string(res), ":")
	userId, err := strconv.Atoi(parts[0])
	if err != nil {
		return
	}

	// check user temporarily banned
	if _, ok := s.Banned[userId]; ok {
		log.Debug().Int("id", userId).Msg("got api request from banned user")
		return
	}

	password := parts[1]

	// load user
	user, err = loadUser(userId)
	if err != nil {
		return
	}

	// check password
	if password == user.Password {
		permission = FullPermissions
		return
	}
	hash1 := calculateHash(user.Password)
	if password == hash1 {
		permission = InvoicePermissions
		return
	}
	hash2 := calculateHash(hash1)
	if password == hash2 {
		permission = ReadOnlyPermissions
		return
	}

	err = errors.New("invalid password")
	return
}

func errorTimeout(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{
      "error": true,
      "code": 5,
      "message": "timeout"
    }`))
}

func errorInvalidParams(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{
      "error": true,
      "code": 8,
      "message": "invalid params"
    }`))
}

func errorBadAuth(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{
      "error": true,
      "code": 1,
      "message": "bad auth"
    }`))
}

func errorInsufficientPermissions(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{
      "error": true,
      "code": 2,
      "message": "insufficient permissions"
    }`))
}

func errorPaymentFailed(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{
      "error": true,
      "code": 10,
      "message": "` + err.Error() + `"
    }`))
}

func errorInternal(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{
      "error": true,
      "code": 7,
      "message": "Internal failure"
    }`))
}

func handleAPI(u User, opts docopt.Opts) {
	go u.track("api", nil)

	passwordFull := u.Password
	passwordInvoice := calculateHash(passwordFull)
	passwordReadOnly := calculateHash(passwordInvoice)

	tokenFull := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%s", u.Id, passwordFull)))
	tokenInvoice := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%s", u.Id, passwordInvoice)))
	tokenReadOnly := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%s", u.Id, passwordReadOnly)))

	switch {
	case opts["full"].(bool):
		u.sendMessageWithPicture(qrURL(tokenFull), tokenFull)
	case opts["invoice"].(bool):
		u.sendMessageWithPicture(qrURL(tokenInvoice), tokenInvoice)
	case opts["readonly"].(bool):
		u.sendMessageWithPicture(qrURL(tokenReadOnly), tokenReadOnly)
	case opts["url"].(bool):
		u.sendMessageWithPicture(qrURL(s.ServiceURL+"/"), s.ServiceURL+"/")
	case opts["refresh"].(bool):
		opts["bluewallet"] = true
		u.notify(t.COMPLETED, nil)
	default:
		u.notify(t.APICREDENTIALS, t.T{
			"Full":       tokenFull,
			"Invoice":    tokenInvoice,
			"ReadOnly":   tokenReadOnly,
			"ServiceURL": s.ServiceURL,
		})
	}
}

func handleLightningATM(u User) {
	token := base64.StdEncoding.EncodeToString(
		[]byte(fmt.Sprintf("%d:%s", u.Id, u.Password)))
	text := fmt.Sprintf("%s@%s", token, s.ServiceURL)
	u.sendMessageWithPicture(qrURL(text), "<pre>"+text+"</pre>")
}
