package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/lntxbot/t"
	"github.com/gorilla/mux"
	"gopkg.in/antage/eventsource.v1"
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
		ctx, _, permission, err := loadUserFromAPICall(r)
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

		lnurlEncoded := handleCreateLNURLWithdraw(ctx, docopt.Opts{
			"<satoshis>": params.Satoshis,
		})
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
		_, user, permission, err := loadUserFromAPICall(r)
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
		_, user, permission, err := loadUserFromAPICall(r)
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

	router.Path("/payments/stream").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, user, permission, err := loadUserFromAPICall(r)
		if err != nil {
			errorBadAuth(w)
			return
		}
		if permission < ReadOnlyPermissions {
			errorInsufficientPermissions(w)
			return
		}

		var es eventsource.EventSource
		if ies, ok := userPaymentStream.Get(strconv.Itoa(user.Id)); ok {
			es = ies.(eventsource.EventSource)
		} else {
			es = eventsource.New(
				&eventsource.Settings{
					Timeout:        5 * time.Second,
					CloseOnTimeout: true,
					IdleTimeout:    1 * time.Minute,
				},
				func(r *http.Request) [][]byte {
					return [][]byte{
						[]byte("X-Accel-Buffering: no"),
						[]byte("Cache-Control: no-cache"),
						[]byte("Content-Type: text/event-stream"),
						[]byte("Connection: keep-alive"),
						[]byte("Access-Control-Allow-Origin: *"),
					}
				},
			)
			userPaymentStream.Set(strconv.Itoa(user.Id), es)
			go func() {
				for {
					time.Sleep(25 * time.Second)
					es.SendEventMessage("", "keepalive", "")
				}
			}()
		}

		go func() {
			time.Sleep(100 * time.Millisecond)
			es.SendRetryMessage(3 * time.Second)
		}()

		es.ServeHTTP(w, r)
	})
}

func loadUserFromAPICall(
	r *http.Request,
) (ctx context.Context, user User, permission Permission, err error) {
	ctx = context.WithValue(context.Background(), "origin", "api")

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

	ctx = context.WithValue(ctx, "initiator", user)

	// check password
	if password == user.Password {
		permission = FullPermissions
		return
	}
	hash1 := hashString(user.Password)
	if password == hash1 {
		permission = InvoicePermissions
		return
	}
	hash2 := hashString(hash1)
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

func handleAPI(ctx context.Context, opts docopt.Opts) {
	u := ctx.Value("initiator").(User)
	go u.track("api", nil)

	passwordFull := u.Password
	passwordInvoice := hashString(passwordFull)
	passwordReadOnly := hashString(passwordInvoice)

	tokenFull := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%s", u.Id, passwordFull)))
	tokenInvoice := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%s", u.Id, passwordInvoice)))
	tokenReadOnly := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%s", u.Id, passwordReadOnly)))

	switch {
	case opts["full"].(bool):
		send(ctx, qrURL(tokenFull), tokenFull)
	case opts["invoice"].(bool):
		send(ctx, qrURL(tokenInvoice), tokenInvoice)
	case opts["readonly"].(bool):
		send(ctx, qrURL(tokenReadOnly), tokenReadOnly)
	case opts["url"].(bool):
		send(ctx, qrURL(s.ServiceURL+"/"), s.ServiceURL+"/")
	case opts["refresh"].(bool):
		opts["bluewallet"] = true
		send(ctx, u, t.COMPLETED)
	default:
		send(ctx, u, t.APICREDENTIALS, t.T{
			"Full":       tokenFull,
			"Invoice":    tokenInvoice,
			"ReadOnly":   tokenReadOnly,
			"ServiceURL": s.ServiceURL,
		})
	}
}

func handleLightningATM(ctx context.Context) {
	u := ctx.Value("initiator").(User)
	token := base64.StdEncoding.EncodeToString(
		[]byte(fmt.Sprintf("%d:%s", u.Id, u.Password)))
	text := fmt.Sprintf("%s@%s", token, s.ServiceURL)
	send(ctx, qrURL(text), "<pre>"+text+"</pre>")
}
