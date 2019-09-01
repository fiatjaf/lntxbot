package main

import (
	"errors"
	"fmt"
	"time"

	"gopkg.in/jmcvetta/napping.v3"
)

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
	Ok    string `json:"ok"`
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

func LNToRubExchange(user User, amount float64, typ, unit, target string, messageId int) (id string, err error) {
	var reply LNToRubOrderReply
	_, err = napping.Post("https://vds.sw4me.com/lntorub", map[string]interface{}{
		"key":    s.LNToRubKey,
		"method": "exchange",
		"unit":   unit,
		"amount": amount,
		"type":   typ,
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

	id = reply.Hash

	// pay the invoice
	inv, err := ln.Call("decodepay", reply.Invoice)
	if err != nil {
		err = errors.New("Failed to decode invoice.")
		return
	}
	err = user.actuallySendExternalPayment(
		messageId, reply.Invoice, inv, inv.Get("msatoshi").Int(),
		fmt.Sprintf("%s.lntorub.%s.%d", s.ServiceId, reply.Hash, user.Id), map[string]interface{}{},
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
				var orders []LNToRubDataOrder
				var ok bool
				if orders, ok = lntorubdata.Orders[typ]; !ok {
					orders = make([]LNToRubDataOrder, 0)
				}

				orders = append(orders, LNToRubDataOrder{
					Id:     reply.Hash,
					Sat:    reply.Sat,
					Rub:    reply.Rub,
					Target: target,
					Time:   time.Now().Format("2006-01-02T15:04:05"),
				})

				// only store the last 20
				if len(orders) > 20 {
					orders = orders[len(orders)-20:]
				}

				lntorubdata.Orders[typ] = orders
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
			var cancelreply LNToRubReply
			_, err = napping.Post("https://vds.sw4me.com/lntorub", map[string]interface{}{
				"key":    s.LNToRubKey,
				"method": "cancel",
				"hash":   reply.Hash,
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
		},
	)

	return
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

func getDefaultLNToRubTarget(user User, typ string) string {
	var data LNToRubData
	err := user.getAppData("lntorub", &data)
	if err != nil {
		return ""
	}
	target, _ := data.Targets[typ]
	return target
}

func setDefaultLNToRubTarget(user User, typ, target string) error {
	var data LNToRubData
	err := user.getAppData("lntorub", &data)
	if err != nil {
		return err
	}

	if data.Targets == nil {
		data.Targets = make(map[string]string)
	}
	data.Targets[typ] = target

	err = user.setAppData("lntorub", data)
	return err
}
