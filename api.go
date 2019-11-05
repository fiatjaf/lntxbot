package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
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

	http.HandleFunc("/generatelnurlwithdraw", func(w http.ResponseWriter, r *http.Request) {
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

		lnurlEncoded := handleLNURLPay(user, sats, -rand.Int())
		if lnurlEncoded == "" {
			errorInvalidParams(w)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			LNURL string `json:"lnurl"`
		}{lnurlEncoded})
	})
}

func loadUserFromAPICall(r *http.Request) (user User, permission Permission, err error) {
	// decode user id and password from auth token
	token := strings.Split(strings.TrimSpace(r.Header.Get("Authorization")), " ")[1]
	res, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return
	}
	parts := strings.Split(string(res), ":")
	userId, err := strconv.Atoi(parts[0])
	if err != nil {
		return
	}
	password := parts[1]

	// load user
	user, err = loadUser(userId, 0)
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
