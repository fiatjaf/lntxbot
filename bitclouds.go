package main

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"gopkg.in/jmcvetta/napping.v3"
)

type BitcloudsData map[string]BitcloudInstanceData // {<host>: {policy: ""}, <host>: ...}

type BitcloudInstanceData struct {
	Policy string `json:"policy"` // remind, letdie, autorefill
}

type BitcloudStatus struct {
	Status    string      `json:"status"`
	HoursLeft int         `json:"hours_left"`
	IP        string      `json:"ip"`
	SSHPort   interface{} `json:"ssh_port"`
	SSHUser   string      `json:"ssh_usr"`
	SSHPwd    string      `json:"ssh_pwd"`
	AppPort   interface{} `json:"app_port"`
	Sparko    string      `json:"sparko"`
	UserPort  interface{} `json:"user_port"`
	RPCPort   interface{} `json:"rpc_port"`
	RPCUser   string      `json:"rpc_usr"`
	RPCPwd    string      `json:"rpc_pwd"`

	Pwd string `json:"pwd"`
}

func createBitcloudImage(user User, image string) (err error) {
	var create struct {
		Host       string `json:"host"`
		PayToStart string `json:"paytostart"`
	}
	resp, err := napping.Get("https://bitclouds.sh/create/"+image, nil, &create, nil)
	if err != nil || resp.Status() >= 300 {
		log.Warn().Err(err).Msg("failed to create bitclouds images")
		return errors.New("failed to create bitclouds.sh host")
	}

	inv, err := ln.Call("decodepay", create.PayToStart)
	if err != nil {
		return errors.New("Failed to decode invoice.")
	}
	err = user.actuallySendExternalPayment(
		0, create.PayToStart, inv, inv.Get("msatoshi").Int(),
		fmt.Sprintf("%s.bitclouds.%s.%d", s.ServiceId, create.Host, user.Id), map[string]interface{}{},
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
			paymentHasSucceeded(u, messageId, msatoshi, msatoshi_sent, preimage, "bitclouds", hash)

			// acknowledge the vps creation
			go func() {
				for i := 0; i < 25; i++ {
					status, err := getBitcloudStatus(create.Host)
					if err != nil {
						u.notifyAsReply(t.ERROR, t.T{"App": "bitclouds", "Err": err.Error()}, messageId)
						return
					}

					if status.Status == "subscribed" {
						// creation successful
						var data BitcloudsData
						err := user.getAppData("bitclouds", &data)
						if err != nil {
							log.Error().Err(err).Str("user", user.Username).Msg("error loading bitclouds data")
							return
						}
						data[create.Host] = BitcloudInstanceData{Policy: "remind"}
						err = user.setAppData("bitclouds", data)
						if err != nil {
							log.Error().Err(err).Str("user", user.Username).Msg("error saving bitclouds data")
						}

						u.notifyAsReply(t.BITCLOUDSCREATED, t.T{
							"Image":  image,
							"Host":   create.Host,
							"Status": status,
						}, messageId)
						return
					} else {
						// keep polling
						time.Sleep(15 * time.Second)
					}
				}

				u.notify(t.BITCLOUDSSTOPPEDWAITING, t.T{
					"Host": create.Host,
				})
			}()
		},
		paymentHasFailed,
	)
	return
}

func getBitcloudStatus(host string) (status BitcloudStatus, err error) {
	resp, err := napping.Get("https://bitclouds.sh/status/"+host, nil, &status, nil)
	if err != nil || resp.Status() >= 300 {
		log.Warn().Err(err).Str("host", host).Msg("failed to get bitcloud status")
		if err == nil {
			err = errors.New("failed to get " + host + " status")
		}
		return
	}

	if status.SSHUser == "" {
		status.SSHUser = "root"
	}

	if status.Pwd != "" && status.SSHPwd == "" {
		status.SSHPwd = status.Pwd
	}

	return
}

func topupBitcloud(user User, host string, sats int) error {
	var topup struct {
		Invoice string `json:"invoice"`
	}
	resp, err := napping.Get("https://bitclouds.sh/topup/"+host+"/"+strconv.Itoa(sats), nil, &topup, nil)
	if err != nil || resp.Status() >= 300 {
		log.Warn().Err(err).Msg("failed to get bitclouds topup invoice")
		return errors.New("failed to get invoice to topup")
	}

	inv, err := ln.Call("decodepay", topup.Invoice)
	if err != nil {
		return errors.New("Failed to decode invoice.")
	}
	return user.actuallySendExternalPayment(
		0, topup.Invoice, inv, inv.Get("msatoshi").Int(),
		fmt.Sprintf("%s.bitclouds.%s.%d.%d", s.ServiceId, host, time.Now().Unix(), user.Id), map[string]interface{}{},
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
			paymentHasSucceeded(u, messageId, msatoshi, msatoshi_sent, preimage, "bitclouds", hash)
		},
		paymentHasFailed,
	)
}

func showBitcloudStatus(user User, host string) {
	status, err := getBitcloudStatus(host)
	if err != nil {
		user.notify(t.ERROR, t.T{"App": "bitclouds", "Err": err.Error()})
	}

	user.notify(t.BITCLOUDSSTATUS, t.T{
		"Host":   host,
		"Status": status,
	})
}

func bitcloudsImagesKeyboard() (inlinekeyboard [][]tgbotapi.InlineKeyboardButton, err error) {
	var imagesresp struct {
		Images []string `json:"images"`
	}

	resp, err := napping.Get("https://bitclouds.sh/images", nil, &imagesresp, nil)
	if err != nil || resp.Status() >= 300 {
		log.Warn().Err(err).Msg("failed to get bitclouds images")
		return nil, errors.New("failed to list bitclouds.sh images")
	}

	images := imagesresp.Images
	nimages := len(images)
	inlinekeyboard = make([][]tgbotapi.InlineKeyboardButton, nimages/2+nimages%2)
	for i, image := range images {
		inlinekeyboard[i/2] = append(inlinekeyboard[i/2],
			tgbotapi.NewInlineKeyboardButtonData(
				image,
				fmt.Sprintf("x=bitclouds-create-%s", image),
			),
		)
	}

	return
}

func bitcloudsHostsKeyboard(user User, data string) (noHosts bool, singleHost string, inlinekeyboard [][]tgbotapi.InlineKeyboardButton, err error) {
	hosts, err := listBitclouds(user)
	if err != nil {
		return
	}

	nhosts := len(hosts)
	if nhosts == 0 {
		noHosts = true
		return
	} else if nhosts == 1 {
		singleHost = hosts[0]
		return
	}

	inlinekeyboard = make([][]tgbotapi.InlineKeyboardButton, nhosts/2+nhosts%2)
	for i, host := range hosts {
		inlinekeyboard[i/2] = append(inlinekeyboard[i/2],
			tgbotapi.NewInlineKeyboardButtonData(
				host,
				fmt.Sprintf("x=bitclouds-%s-%s", data, host),
			),
		)
	}

	return
}

func listBitclouds(user User) (hosts []string, err error) {
	var data BitcloudsData
	err = user.getAppData("bitclouds", &data)
	if err != nil {
		log.Error().Err(err).Str("user", user.Username).Msg("error loading bitclouds data")
		return
	}

	for host, _ := range data {
		hosts = append(hosts, host)
	}

	return
}
