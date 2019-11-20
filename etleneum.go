package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/tidwall/gjson"
	"gopkg.in/jmcvetta/napping.v3"
)

type EtleneumResponse struct {
	Ok    bool            `json:"ok"`
	Error string          `json:"error"`
	Value json.RawMessage `json:"value"`
}

func getContractState(contractId string) (state gjson.Result, err error) {
	var reply EtleneumResponse
	_, err = napping.Get("https://etleneum.com/~/contract/"+contractId+"/state", nil, &reply, &reply)
	if err != nil {
		err = errors.New("etleneum.com invalid response: " + err.Error())
		return
	}
	if !reply.Ok {
		err = errors.New("etleneum.com call failed: " + reply.Error)
		return
	}

	return gjson.ParseBytes([]byte(reply.Value)), nil
}

func prepareCall(
	contractId string,
	method string, payload interface{}, sats int,
) (callId, bolt11 string, err error) {
	var reply EtleneumResponse
	_, err = napping.Post(
		"https://etleneum.com/~/contract/"+contractId+"/call",
		&payload, &reply, &reply,
	)
	if err != nil {
		err = errors.New("etleneum.com invalid response: " + err.Error())
		return
	}
	if !reply.Ok {
		err = errors.New("etleneum.com call failed: " + reply.Error)
		return
	}

	decoded := gjson.ParseBytes([]byte(reply.Value))
	bolt11 = decoded.Get("invoice").String()
	callId = decoded.Get("id").String()

	return
}

func payAndDispatchCall(
	user User, messageId int,
	callId, bolt11 string,
	appname string, successKey t.Key,
) (ret gjson.Result, err error) {
	inv, err := ln.Call("decodepay", bolt11)
	if err != nil {
		err = errors.New("Failed to decode invoice.")
		return
	}

	err = user.actuallySendExternalPayment(
		messageId, bolt11, inv, inv.Get("msatoshi").Int(),
		fmt.Sprintf("%s.etleneum.%s.%d", s.ServiceId, callId, user.Id),
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
			paymentHasSucceeded(u, messageId, msatoshi, msatoshi_sent, preimage, appname, hash)

			// finish call on etleneum
			var reply EtleneumResponse
			_, err := napping.Post("https://etleneum.com/~/call/"+callId, nil, &reply, &reply)
			if err != nil {
				err = errors.New("etleneum.com invalid response: " + err.Error())
				return
			}
			if !reply.Ok {
				err = errors.New("etleneum.com call failed: " + reply.Error)
				return
			}

			// grab returned value from call
			ret = gjson.ParseBytes([]byte(reply.Value))

			u.notifyAsReply(successKey, nil, messageId)
		},
		func(
			u User,
			messageId int,
			hash string,
		) {
			// on failure
			paymentHasFailed(u, messageId, hash)

			u.notifyAsReply(t.ETLENEUMFAILEDTOPAY, nil, messageId)
		},
	)

	return
}

func patchCall(callId string, payload interface{}) (err error) {
	var reply EtleneumResponse
	_, err = napping.Patch("https://etleneum.com/~/call/"+callId, &payload, &reply, &reply)
	if err != nil {
		err = errors.New("etleneum.com invalid response: " + err.Error())
		return
	}
	if !reply.Ok {
		err = errors.New("etleneum.com call failed: " + reply.Error)
		return
	}

	return
}
