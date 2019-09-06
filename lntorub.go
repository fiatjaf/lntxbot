package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gopkg.in/jmcvetta/napping.v3"
)

type LNToRubOrder struct {
	Hash      string  `json:"hash"`
	Sat       int64   `json:"sat"`
	Rub       float64 `json:"rub"`
	Target    string  `json:"target"`
	Type      string  `json:"type"`
	Invoice   string  `json:"invoice"`
	MessageId int     `json:"messageId"`
}

type LNToRubData struct {
	Targets map[string]string             `json:"targets"`
	Orders  map[string][]LNToRubDataOrder `json:"orders"`
}

type LNToRubDataOrder struct {
	Id     string  `json:"id"`
	Sat    int64   `json:"sat"`
	Rub    float64 `json:"rub"`
	Target string  `json:"target"`
	Time   string  `json:"time"`
}

type LNToRubReply struct {
	Ok    bool   `json:"ok"`
	Error string `json:"error"`
}

type LNToRubOrderReply struct {
	Invoice string  `json:"invoice"`
	Sat     int64   `json:"sat"`
	Rub     float64 `json:"rub"`
	Hash    string  `json:"hash"`
	LNToRubReply
}

type LNToRubStatusReply struct {
	Status LNToRubStatus `json:"status"`
	LNToRubReply
}

type LNToRubStatus string

const (
	LNIN LNToRubStatus = "LNIN"
	OKAY               = "OKAY"
	EXPD               = "EXPD"
	QER1               = "QER1"
	QER2               = "QER2"
	CANC               = "CANC"
)

func LNToRubExchangeInit(user User, amount float64, exchangeType, unit, target string, messageId int) (order LNToRubOrder, err error) {
	var reply LNToRubOrderReply
	_, err = napping.Post("https://vds.sw4me.com/lntorub", map[string]interface{}{
		"key":    s.LNToRubKey,
		"method": "exchange",
		"unit":   unit,
		"amount": amount,
		"type":   exchangeType,
		"wallet": target,
	}, &reply, &reply)
	if err != nil {
		log.Warn().Err(err).Msg("lntorub call failed")
		return
	}
	if reply.Error != "" {
		err = errors.New(reply.Error)
		log.Warn().Err(err).Msg("lntorub call failed")
		return
	}

	return LNToRubOrder{
		Hash:      reply.Hash,
		Sat:       reply.Sat,
		Rub:       reply.Rub,
		Target:    target,
		Type:      exchangeType,
		Invoice:   reply.Invoice,
		MessageId: messageId,
	}, nil
}

func LNToRubExchangeFinish(user User, order LNToRubOrder) error {
	// pay the invoice and that's it
	inv, err := ln.Call("decodepay", order.Invoice)
	if err != nil {
		err = errors.New("Failed to decode invoice.")
		return err
	}
	err = user.actuallySendExternalPayment(
		order.MessageId, order.Invoice, inv, inv.Get("msatoshi").Int(),
		fmt.Sprintf("%s.lntorub.%s.%d", s.ServiceId, order.Hash, user.Id), map[string]interface{}{},
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
			paymentHasSucceeded(u, messageId, msatoshi, msatoshi_sent, preimage, "lntorub", hash)

			// save order
			var lntorubdata LNToRubData
			err := u.getAppData("lntorub", &lntorubdata)
			if err != nil {
				log.Warn().Err(err).Int("user", u.Id).Msg("lntorub get data fail")
			} else {
				// create map if it doesn't exist
				if lntorubdata.Orders == nil {
					lntorubdata.Orders = make(map[string][]LNToRubDataOrder)
				}

				var orders []LNToRubDataOrder
				var ok bool
				if orders, ok = lntorubdata.Orders[order.Type]; !ok {
					orders = make([]LNToRubDataOrder, 0)
				}

				orders = append(orders, LNToRubDataOrder{
					Id:     order.Hash,
					Sat:    order.Sat,
					Rub:    order.Rub,
					Target: order.Target,
					Time:   time.Now().Format("2006-01-02T15:04:05"),
				})

				// only store the last 20
				if len(orders) > 20 {
					orders = orders[len(orders)-20:]
				}

				lntorubdata.Orders[order.Type] = orders
				err = u.setAppData("lntorub", lntorubdata)
				if err != nil {
					log.Warn().Err(err).Int("user", u.Id).Interface("data", lntorubdata).
						Msg("lntorub set data fail")
				}
			}
		},
		func(
			u User,
			messageId int,
			hash string,
		) {
			// on failure
			paymentHasFailed(u, messageId, hash)

			// cancel order
			LNToRubExchangeCancel(hash)
		},
	)

	return nil
}

func LNToRubExchangeCancel(orderId string) {
	var cancelreply LNToRubReply
	_, err = napping.Post("https://vds.sw4me.com/lntorub", map[string]interface{}{
		"key":    s.LNToRubKey,
		"method": "cancel",
		"hash":   orderId,
	}, &cancelreply, &cancelreply)
	if err != nil {
		log.Error().Err(err).Msg("lntorub cancel call failed")
		return
	}
	if cancelreply.Error != "" {
		err = errors.New(cancelreply.Error)
		log.Warn().Err(err).Msg("lntorub cancel call failed")
		return
	}

	// delete from redis
	rds.Del("lntorub:" + orderId)
}

func LNToRubQueryStatus(id string) (status LNToRubStatus, err error) {
	var statusreply LNToRubStatusReply
	_, err = napping.Post("https://vds.sw4me.com/lntorub", map[string]interface{}{
		"key":    s.LNToRubKey,
		"method": "status",
		"hash":   id,
	}, &statusreply, &statusreply)
	if err != nil {
		log.Error().Err(err).Msg("lntorub status call failed")
		return
	}
	if statusreply.Error != "" {
		err = errors.New(statusreply.Error)
		log.Warn().Err(err).Msg("lntorub status call failed")
		return
	}

	return statusreply.Status, nil
}

func getDefaultLNToRubTarget(user User, exchangeType string) string {
	var data LNToRubData
	err := user.getAppData("lntorub", &data)
	if err != nil {
		return ""
	}
	target, _ := data.Targets[exchangeType]
	return target
}

func setDefaultLNToRubTarget(user User, exchangeType, target string) error {
	var data LNToRubData
	err := user.getAppData("lntorub", &data)
	if err != nil {
		return err
	}

	if data.Targets == nil {
		data.Targets = make(map[string]string)
	}
	data.Targets[exchangeType] = target

	err = user.setAppData("lntorub", data)
	return err
}

// to be called on app init
func cancelAllLNToRubOrders() {
	// get all preexisting orders and cancel them after 2 minutes.
	// won't affect orders created in the meantime.
	// this is only so we can be sure every unfulfilled order is canceled properly.
	keys := rds.Keys("lntorub:*").Val()
	if len(keys) > 0 {
		log.Debug().Interface("keys", keys).
			Msg("canceling and deleting lntorub orders in 2 minutes")
		time.Sleep(2 * time.Minute)
		for _, key := range keys {
			orderId := strings.Split(key, ":")[1]
			LNToRubExchangeCancel(orderId)
		}
	}
}
