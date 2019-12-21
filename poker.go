package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/lucsky/cuid"
	"gopkg.in/jmcvetta/napping.v3"
)

type PokerFirestoreDocument struct {
	Name   string                         `json:"name"`
	Fields map[string]PokerFirestoreValue `json:"fields"`
}

type PokerFirestoreDocumentList struct {
	Documents []PokerFirestoreDocument `json:"documents"`
}

type PokerFirestoreValue struct {
	String  string `json:"stringValue,omitempty"`
	Integer string `json:"integerValue,omitempty"`
}

func pokerDeposit(user User, sats int, messageId int) (err error) {
	// create invoice
	var invcreate PokerFirestoreDocument
	resp, err := napping.Post("https://firestore.googleapis.com/v1/projects/ln-pkr/databases/(default)/documents/invoices", PokerFirestoreDocument{
		Fields: map[string]PokerFirestoreValue{
			"amount":    {String: strconv.Itoa(sats)},
			"accountId": {String: getPokerId(user)},
			"state":     {String: "requested"},
		},
	}, &invcreate, nil)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = errors.New("error calling lightning-poker.com backend")
		return
	}

	invid := strings.Split(invcreate.Name, "/")[6]

	// get bolt11 from invoice id (after waiting a while for the invoice to get created)
	time.Sleep(2 * time.Second)
	var invdata PokerFirestoreDocument
	resp, err = napping.Get("https://firestore.googleapis.com/v1/projects/ln-pkr/databases/(default)/documents/invoices/"+invid, nil, &invdata, nil)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = errors.New("error calling lightning-poker.com backend")
		return
	}

	bolt11 := invdata.Fields["payment_request"].String

	// actually pay
	inv, err := ln.Call("decodepay", bolt11)
	if err != nil {
		return errors.New("Failed to decode invoice.")
	}
	err = user.actuallySendExternalPayment(
		messageId, bolt11, inv, inv.Get("msatoshi").Int(),
		fmt.Sprintf("%s.poker.%s.%d", s.ServiceId, cuid.Slug(), user.Id),
		func(
			u User,
			messageId int,
			msatoshi float64,
			msatoshi_sent float64,
			preimage string,
			tag string,
			hash string,
		) {
			// on success
			paymentHasSucceeded(u, messageId, msatoshi, msatoshi_sent, preimage, "poker", hash)
		},
		func(
			u User,
			messageId int,
			hash string,
		) {
			// on failure
			paymentHasFailed(u, messageId, hash)
		},
	)
	if err != nil {
		return
	}

	return
}

func getPokerBalance(user User) (int, error) {
	var balance PokerFirestoreDocument
	resp, err := napping.Get("https://firestore.googleapis.com/v1/projects/ln-pkr/databases/(default)/documents/accounts/"+getPokerId(user), nil, &balance, nil)
	if err != nil {
		return 0, err
	}
	if resp.Status() >= 300 {
		err = errors.New("error calling lightning-poker.com backend")
		return 0, err
	}

	val, _ := strconv.Atoi(balance.Fields["balance"].Integer)
	return val, nil
}

func withdrawPoker(user User, sats int, messageId int) (err error) {
	bolt11, _, _, err := user.makeInvoice(sats, "withdraw from lightning-poker.com",
		"", nil, messageId, "poker", true)
	if err != nil {
		return
	}

	resp, err := napping.Post("https://firestore.googleapis.com/v1/projects/ln-pkr/databases/(default)/documents/payments", PokerFirestoreDocument{
		Fields: map[string]PokerFirestoreValue{
			"payment_request": {String: bolt11},
			"accountId":       {String: getPokerId(user)},
			"state":           {String: "requested"},
		},
	}, nil, nil)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = errors.New("error calling lightning-poker.com backend")
		return
	}

	return
}

func getActivePokerTables() (nplayers int, ntables int, err error) {
	var tables PokerFirestoreDocumentList
	resp, err := napping.Get("https://firestore.googleapis.com/v1/projects/ln-pkr/databases/(default)/documents/tables", nil, &tables, nil)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = errors.New("error calling lightning-poker.com backend")
		return
	}

	for _, table := range tables.Documents {
		count, _ := strconv.Atoi(table.Fields["playing"].Integer)
		if count > 0 {
			ntables += 1
			nplayers += count
		}
	}

	return
}

func getCurrentPokerPlayers() (playerHashes []string, totalChips int, err error) {
	var players PokerFirestoreDocumentList
	resp, err := napping.Get("https://firestore.googleapis.com/v1/projects/ln-pkr/databases/(default)/documents/players", nil, &players, nil)
	if err != nil {
		return
	}
	if resp.Status() >= 300 {
		err = errors.New("error calling lightning-poker.com backend")
		return
	}

	playerHashes = make([]string, len(players.Documents))
	for i, player := range players.Documents {
		chips, _ := strconv.Atoi(player.Fields["chips"].Integer)
		totalChips += chips

		playerHashes[i] = player.Fields["accountHash"].String
	}

	return
}

func getPokerId(user User) string {
	return s.ServiceId + ":" + calculateHash(fmt.Sprintf("%s.poker.%d", s.BotToken, user.Id))[:14]
}

func getPokerAccountHash(user User) string {
	return calculateHash(getPokerId(user) + "this-is-salt-jfkd934343")[:10]
}

func getPokerURL(user User) string {
	return fmt.Sprintf("%s/static/poker/?account=%s&user=%d", s.ServiceURL, getPokerId(user), user.Id)
}

func loadUserFromPokerCall(r *http.Request) (user User, pokerFriends []User, err error) {
	token := strings.TrimSpace(r.Header.Get("X-Bot-Poker-Token"))
	res, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		err = errors.New("invalid token")
		return
	}

	parts := strings.SplitN(string(res), "~", 2)
	if len(parts) != 2 {
		err = errors.New("invalid token")
		return
	}

	userId, err := strconv.Atoi(parts[0])
	if err != nil {
		err = errors.New("invalid user")
		return
	}
	pokerId := parts[1]

	// load user
	user, err = loadUser(userId, 0)
	if err != nil {
		err = errors.New("couldn't load user")
		return
	}

	// check poker id
	if getPokerId(user) != pokerId {
		err = errors.New("wrong pokerId")
		return
	}

	// load poker friends (people this person have interacted with and that have already played poker)
	pg.Select(&pokerFriends, `
SELECT `+USERFIELDS+`
FROM telegram.account
WHERE id IN (
  SELECT to_id AS friend
  FROM lightning.transaction
  WHERE from_id = $1 AND to_id != from_id AND amount > 100000 AND time > now() - '30d'::interval
  INNER JOIN lightning.transaction AS tx
    ON tx.from_id = friend
  WHERE tx.remote_node = '03ad156742a9a9d0e82e0022f264d6857addfd534955d5e97de4a695bf8dd12af0'
    AND tx.time > (now() - make_interval(days := 7))
) AND username IS NOT NULL
    `, user.Id)

	return
}

func servePoker() {
	// this is called by the poker app to deposit funds as soon as the user tries to sit on a table
	// but doesn't have enough money for the buy-in.
	router.Path("/app/poker/deposit").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sats, err := strconv.Atoi(r.FormValue("satoshis"))
		if err != nil {
			http.Error(w, "invalid amount", 400)
			return
		}

		user, _, err := loadUserFromPokerCall(r)
		if err != nil {
			http.Error(w, err.Error(), 401)
			return
		}

		// actually send the deposit
		err = pokerDeposit(user, sats, 0)
		if err != nil {
			http.Error(w, "deposit went wrong", 505)
		}

		fmt.Fprintf(w, "ok")
	})

	router.Path("/app/poker/playing").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pokerfriends, err := loadUserFromPokerCall(r)
		if err != nil {
			http.Error(w, err.Error(), 401)
			return
		}

		rds.HSet("poker-players", getPokerAccountHash(user), user.Username)
		rds.Expire("poker-players", time.Hour*24)

		// also remove player from the watchers list because he is already playing
		rds.Del(fmt.Sprintf("poker-watcher:%d", user.Id))

		// notify poker friends
		for _, friend := range pokerfriends {
			friend.notify(t.POKERNOTIFYFRIEND, t.T{"FriendName": user.AtName()})
		}

		// notify all watchers
		notifyPokerWatchers()
	})

	router.Path("/app/poker/online").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rds.HGetAll("poker-players").Val())
	})
}

func notifyPokerWatchers() {
	playerHashes, chips, err := getCurrentPokerPlayers()
	if err != nil {
		log.Warn().Err(err).Msg("failed to get poker data before notifying people")
		return
	}

	watcherKeys := rds.Keys("poker-watcher:*").Val()
	watchers := make([]User, len(watcherKeys))
	nwatchers := len(watcherKeys)
	for i, watcherKey := range watcherKeys {
		userId, err := strconv.Atoi(strings.Split(watcherKey, ":")[1])
		if err != nil {
			continue
		}
		watcher, err := loadUser(userId, 0)
		if err != nil {
			continue
		}

		watchers[i] = watcher
	}

	for _, watcher := range watchers {
		nplayers := len(playerHashes)

		// watcher is not playing, so he was counted in nwatchers
		if nwatchers == 1 && nplayers == 0 {
			// this means there's only this watcher around, so don't notify
			continue
		}

		watcher.notify(t.POKERNOTIFY, t.T{
			"Sats":    chips,
			"Playing": nplayers,
			"Waiting": nwatchers,
		})
	}
}

func subscribePoker(user User, howlong time.Duration, active bool) {
	if active {
		// "active" means the person has called /app_poker_available which means they really
		// want to play now. all other poker-related commands will subscribe them too,
		// but will not trigger notifications to other players who were already subscribed.
		// an "active" action will.
		notifyPokerWatchers()
	}

	// now we just add them to the list so they'll be notified later if someone wants to play
	go func() {
		time.Sleep(2 * time.Second)
		rds.Set(fmt.Sprintf("poker-watcher:%d", user.Id), "-", howlong)
	}()
}
