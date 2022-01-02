package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/docopt/docopt-go"
	decodepay "github.com/fiatjaf/ln-decodepay"
	"github.com/fiatjaf/lntxbot/t"
)

func registerBluewalletMethods() {
	router.Path("/getinfo").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errorBadAuth(w)
	})

	router.Path("/auth").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var params struct {
			Login        string `json:"login"`
			Password     string `json:"password"`
			RefreshToken string `json:"refresh_token"`
		}
		err := json.NewDecoder(r.Body).Decode(&params)
		if err != nil {
			errorInvalidParams(w)
			return
		}
		log.Debug().
			Str("login", params.Login).Str("password", params.Password).
			Str("token", params.RefreshToken).Msg("bluewallet /auth")

		var token string
		if params.Password == "" {
			token = params.RefreshToken
		} else {
			token = base64.StdEncoding.EncodeToString(
				[]byte(params.Login + ":" + params.Password))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			RefreshToken string `json:"refresh_token"`
			AccessToken  string `json:"access_token"`
		}{token, token})
	})

	router.Path("/addinvoice").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, user, permission, err := loadUserFromAPICall(r)
		if err != nil {
			errorBadAuth(w)
			return
		}
		if permission < InvoicePermissions {
			errorInsufficientPermissions(w)
			return
		}

		var params struct {
			Amount          string `json:"amt"`
			Memo            string `json:"memo"`
			DescriptionHash string `json:"description_hash"`
		}
		err = json.NewDecoder(r.Body).Decode(&params)
		if err != nil {
			errorInvalidParams(w)
			return
		}
		satoshi, err := strconv.ParseInt(params.Amount, 10, 64)
		if err != nil {
			errorInvalidParams(w)
			return
		}

		log.Debug().Str("amount", params.Amount).
			Str("memo", params.Memo).Stringer("user", &user).
			Msg("bluewallet addinvoice")

		bolt11, hash, err := user.makeInvoice(ctx, &MakeInvoiceArgs{
			IgnoreInvoiceSizeLimit: true,
			Msatoshi:               1000 * satoshi,
			Description:            params.Memo,
			DescriptionHash:        params.DescriptionHash,
			BlueWallet:             true,
		})
		if err != nil {
			errorInternal(w)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			PayReq         string `json:"pay_req"`
			PaymentRequest string `json:"payment_request"`
			AddIndex       string `json:"add_index"`
			RHash          Buffer `json:"r_hash"`
			Hash           string `json:"payment_hash"`
		}{bolt11, bolt11, "1000", Buffer(hash), hash})
	})

	router.Path("/payinvoice").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, user, permission, err := loadUserFromAPICall(r)
		if err != nil {
			errorBadAuth(w)
			return
		}
		if permission < FullPermissions {
			errorInsufficientPermissions(w)
			return
		}

		var params struct {
			Invoice string      `json:"invoice"`
			Amount  interface{} `json:"amount"`
		}
		err = json.NewDecoder(r.Body).Decode(&params)
		if err != nil {
			errorInvalidParams(w)
			return
		}

		var amount int64
		switch val := params.Amount.(type) {
		case string:
			amount, _ = strconv.ParseInt(val, 10, 64)
		case int:
			amount = int64(val)
		case int64:
			amount = val
		case float64:
			amount = int64(val)
		}

		log.Debug().Str("bolt11", params.Invoice).Stringer("user", &user).
			Msg("bluewallet /payinvoice")

		decoded, _ := decodeInvoiceAsLndHub(params.Invoice)
		var preimage string

		go func() {
			select {
			case preimage = <-waitPaymentSuccess(decoded.PaymentHash):
			case <-time.After(150 * time.Second):
			}
		}()

		_, err = user.payInvoice(ctx, params.Invoice, 1000*amount)
		if err != nil {
			errorPaymentFailed(w, err)
			return
		}

		select {
		case preimage = <-waitPaymentSuccess(decoded.PaymentHash):
		case <-time.After(5 * time.Second):
		}

		tx, _ := user.getTransaction(decoded.PaymentHash)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LndHubPaymentResult{
			PaymentError:    "",
			PaymentPreimage: preimage,
			PaymentRoute:    make(map[string]interface{}),
			PaymentHash:     tx.Hash,
			Decoded:         decoded,
			FeeMsat:         int64(tx.Fees * 1000),
			Type:            "paid_invoice",
			Fee:             tx.Fees,
			Value:           tx.Amount,
			Timestamp:       tx.Time.UTC().Unix(),
			Memo:            tx.Description + " " + tx.PeerActionDescription(),
		})
	})

	router.Path("/balance").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, user, permission, err := loadUserFromAPICall(r)
		if err != nil {
			errorBadAuth(w)
			return
		}
		if permission < ReadOnlyPermissions {
			errorInsufficientPermissions(w)
			return
		}

		info, err := user.getInfo()
		if err != nil {
			errorInternal(w)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]map[string]int64{
			"BTC": {
				"AvailableBalance": int64(info.Balance),
			},
		})
	})

	router.Path("/gettxs").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, user, permission, err := loadUserFromAPICall(r)
		if err != nil {
			errorBadAuth(w)
			return
		}
		if permission < ReadOnlyPermissions {
			errorInsufficientPermissions(w)
			return
		}

		limit, offset := getLimitAndOffset(r)
		txs, err := user.listTransactions(limit, offset, 120, "", Out)
		if err != nil {
			errorInternal(w)
			return
		}

		payments := make([]LndHubPaymentResult, len(txs))
		for i, tx := range txs {
			preimage := tx.Preimage.String
			if preimage == "" {
				preimage = "0000000000000000000000000000000000000000000000000000000000000000"
			}

			payments[i] = LndHubPaymentResult{
				PaymentError:    "",
				PaymentPreimage: preimage,
				PaymentRoute:    make(map[string]interface{}),
				PaymentHash:     tx.Hash,
				Decoded:         LndHubDecoded{},
				FeeMsat:         int64(tx.Fees * 1000),
				Type:            "paid_invoice",
				Fee:             tx.Fees,
				Value:           tx.Amount,
				Timestamp:       tx.Time.UTC().Unix(),
				Memo:            tx.Description + " " + tx.PeerActionDescription(),
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payments)
	})

	router.Path("/getpending").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]interface{}{})
	})

	router.Path("/getuserinvoices").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, user, permission, err := loadUserFromAPICall(r)
		if err != nil {
			errorBadAuth(w)
			return
		}
		if permission < ReadOnlyPermissions {
			errorInsufficientPermissions(w)
			return
		}

		limit, offset := getLimitAndOffset(r)
		txns, err := user.listTransactions(limit, offset, 120, "", In)
		if err != nil {
			errorInternal(w)
			return
		}

		type Inv struct {
			RHash          Buffer  `json:"r_hash"`
			PaymentRequest string  `json:"payment_request"`
			PayReq         string  `json:"pay_req"`
			AddIndex       string  `json:"add_index"`
			Description    string  `json:"description"`
			PaymentHash    string  `json:"payment_hash"`
			IsPaid         bool    `json:"ispaid"`
			Amount         float64 `json:"amt"`
			ExpireTime     float64 `json:"expire_time"`
			Timestamp      int64   `json:"timestamp"`
			Type           string  `json:"type"`
		}

		invs := make([]Inv, len(txns))
		for i, tx := range txns {
			invs[i] = Inv{
				Buffer(tx.Hash),
				"",
				"",
				"1000",
				tx.PeerActionDescription() + tx.Description,
				tx.Hash,
				true,
				tx.Amount,
				float64(s.InvoiceTimeout.Seconds()),
				tx.Time.UTC().Unix(),
				"user_invoice",
			}
		}

		iinv, err := rds.Get("justcreatedbluewalletinvoice:" + strconv.Itoa(user.Id)).Result()
		if err == nil {
			var inv map[string]interface{}
			json.Unmarshal([]byte(iinv), &inv)
			invs = append(invs, Inv{
				Buffer(inv["hash"].(string)),
				inv["bolt11"].(string),
				inv["bolt11"].(string),
				"1000",
				inv["desc"].(string),
				inv["hash"].(string),
				false,
				inv["amount"].(float64),
				inv["expiry"].(float64),
				time.Now().UTC().Unix(),
				"user_invoice",
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(invs)
	})

	router.Path("/decodeinvoice").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bolt11 := r.URL.Query().Get("invoice")

		decoded, err := decodeInvoiceAsLndHub(bolt11)
		if err != nil {
			errorInternal(w)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(decoded)
	})
}

type Buffer string

func (b Buffer) MarshalJSON() ([]byte, error) {
	arrayBytes, err := hex.DecodeString(string(b))
	if err != nil {
		return nil, err
	}
	arr := make([]int, len(arrayBytes))
	for i, b := range arrayBytes {
		arr[i] = int(b)
	}
	return json.Marshal(map[string]interface{}{
		"type": "Buffer",
		"data": arr,
	})
}

type LndHubPaymentResult struct {
	PaymentError    string                 `json:"payment_error"`
	PaymentPreimage string                 `json:"payment_preimage"`
	PaymentRoute    map[string]interface{} `json:"route"`
	PaymentHash     string                 `json:"payment_hash"`
	Decoded         LndHubDecoded          `json:"decoded"`
	FeeMsat         int64                  `json:"fee_msat"`
	Type            string                 `json:"type"`
	Fee             float64                `json:"fee"`
	Value           float64                `json:"value"`
	Timestamp       int64                  `json:"timestamp"`
	Memo            string                 `json:"memo"`
}

type LndHubDecoded struct {
	Destination     string      `json:"destination"`
	PaymentHash     string      `json:"payment_hash"`
	NumSatoshis     string      `json:"num_satoshis"`
	Timestamp       string      `json:"timestamp"`
	Expiry          string      `json:"expiry"`
	Description     string      `json:"description"`
	DescriptionHash string      `json:"description_hash"`
	FallbackAddr    string      `json:"fallback_addr"`
	CLTVExpiry      string      `json:"cltv_expiry"`
	RouteHints      interface{} `json:"route_hints"`
}

func decodeInvoiceAsLndHub(bolt11 string) (LndHubDecoded, error) {
	inv, err := decodepay.Decodepay(bolt11)
	if err != nil {
		return LndHubDecoded{}, err
	}

	return LndHubDecoded{
		Destination:     inv.Payee,
		PaymentHash:     inv.PaymentHash,
		NumSatoshis:     strconv.Itoa(int(float64(inv.MSatoshi) / 1000.0)),
		Timestamp:       strconv.Itoa(inv.CreatedAt),
		Expiry:          strconv.Itoa(inv.Expiry),
		Description:     inv.Description,
		DescriptionHash: inv.DescriptionHash,
		FallbackAddr:    "",
		CLTVExpiry:      strconv.Itoa(inv.MinFinalCLTVExpiry),
		RouteHints:      inv.Route,
	}, nil
}

func getLimitAndOffset(r *http.Request) (limit int, offset int) {
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}

	offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))

	return
}

func handleBlueWallet(ctx context.Context, opts docopt.Opts) {
	u := ctx.Value("initiator").(User)

	go u.track("bluewallet", map[string]interface{}{
		"refresh": opts["refresh"].(bool),
	})

	var err error
	password := u.Password
	if opts["refresh"].(bool) {
		password, err = u.updatePassword()
		if err != nil {
			log.Warn().Err(err).Stringer("user", &u).Msg("error updating password")
			send(ctx, t.APIPASSWORDUPDATEERROR, t.T{"Err": err.Error()})
			return
		}
		send(ctx, t.COMPLETED)
	}
	blueURL := fmt.Sprintf("lndhub://%d:%s@%s", u.Id, password, s.ServiceURL)
	send(ctx, qrURL(blueURL), "<code>"+blueURL+"</code>")
}
