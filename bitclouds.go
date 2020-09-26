package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"gopkg.in/jmcvetta/napping.v3"
)

const BITCLOUDSHOURPRICESATS = 66

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
	UserPort  interface{} `json:"user_port"`
	Sparko    string      `json:"sparko"`
	RPCPort   interface{} `json:"rpc_port"`
	RPCUser   string      `json:"rpc_usr"`
	RPCPwd    string      `json:"rpc_pwd"`

	Pwd string `json:"pwd"`
}

func createBitcloudImage(ctx context.Context, image string) (err error) {
	user := ctx.Value("initiator").(User)

	var create struct {
		Host       string `json:"host"`
		PayToStart string `json:"paytostart"`
	}
	resp, err := napping.Get("https://bitclouds.sh/create/"+image, nil, &create, nil)
	if err != nil || resp.Status() >= 300 {
		log.Warn().Err(err).Msg("failed to create bitclouds images")
		return errors.New("failed to create bitclouds.sh host")
	}

	inv, err := decodeInvoice(create.PayToStart)
	if err != nil {
		return errors.New("Failed to decode invoice.")
	}
	err = user.actuallySendExternalPayment(
		ctx, create.PayToStart, inv, inv.MSatoshi,
		func(
			ctx context.Context,
			u User,
			msatoshi float64,
			msatoshi_sent float64,
			preimage string,
			tag string,
			hash string,
		) {
			// on success
			paymentHasSucceeded(ctx, u, msatoshi, msatoshi_sent, preimage, "bitclouds", hash)

			// acknowledge the vps creation
			go func() {
				for i := 0; i < 25; i++ {
					status, err := getBitcloudStatus(create.Host)
					if err != nil {
						send(ctx, u, t.ERROR, ctx.Value("message"),
							t.T{"App": "bitclouds", "Err": err.Error()})
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

						send(ctx, u, t.BITCLOUDSCREATED, t.T{
							"Image":       image,
							"Host":        create.Host,
							"EscapedHost": escapeBitcloudsHost(create.Host),
							"Status":      status,
						}, ctx.Value("message"))
						return
					} else {
						// keep polling
						time.Sleep(15 * time.Second)
					}
				}

				send(ctx, u, t.BITCLOUDSSTOPPEDWAITING, t.T{
					"Host":        create.Host,
					"EscapedHost": escapeBitcloudsHost(create.Host),
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

	if status.Sparko != "" {
		status.Sparko = status.Sparko[:len(status.Sparko)-4]
	}

	if status.Pwd != "" && status.SSHPwd == "" {
		status.SSHPwd = status.Pwd
	}

	if status.SSHPort == nil {
		status.SSHPort = "22"
	}

	return
}

func topupBitcloud(ctx context.Context, host string, sats int) error {
	user := ctx.Value("initiator").(User)

	// the amount to actually pay will be dependent on the
	// fixed sat/hour price (because if we overpay we don't get
	// fractions of hours in the balance)
	sats = sats - (sats % BITCLOUDSHOURPRICESATS)

	var topup struct {
		Invoice string `json:"invoice"`
	}
	resp, err := napping.Get("https://bitclouds.sh/topup/"+host+"/"+strconv.Itoa(sats), nil, &topup, nil)
	if err != nil || resp.Status() >= 300 {
		log.Warn().Err(err).Msg("failed to get bitclouds topup invoice")
		return errors.New("failed to get invoice to topup")
	}

	inv, err := decodeInvoice(topup.Invoice)
	if err != nil {
		return errors.New("Failed to decode invoice.")
	}
	return user.actuallySendExternalPayment(
		ctx, topup.Invoice, inv, inv.MSatoshi,
		func(
			ctx context.Context,
			u User,
			msatoshi float64,
			msatoshi_sent float64,
			preimage string,
			tag string,
			hash string,
		) {
			// on success
			paymentHasSucceeded(ctx, u, msatoshi, msatoshi_sent, preimage, "bitclouds", hash)
		},
		paymentHasFailed,
	)
}

func showBitcloudStatus(ctx context.Context, host string) {
	status, err := getBitcloudStatus(host)
	if err != nil {
		send(ctx, ctx.Value("initiator"), t.ERROR,
			t.T{"App": "bitclouds", "Err": err.Error()})
	}

	send(ctx, ctx.Value("initiator"), t.BITCLOUDSSTATUS, t.T{
		"Host":        host,
		"EscapedHost": escapeBitcloudsHost(host),
		"Status":      status,
	})
}

func bitcloudsImagesKeyboard() (keyboard *tgbotapi.InlineKeyboardMarkup, err error) {
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
	buttons := make([][]tgbotapi.InlineKeyboardButton, nimages/2+nimages%2)
	for i, image := range images {
		buttons[i/2] = append(buttons[i/2],
			tgbotapi.NewInlineKeyboardButtonData(
				image,
				fmt.Sprintf("x=bitclouds-create-%s", image),
			),
		)
	}

	return &tgbotapi.InlineKeyboardMarkup{buttons}, nil
}

func bitcloudsHostsKeyboard(user User, data string) (noHosts bool, singleHost string, keyboard *tgbotapi.InlineKeyboardMarkup, err error) {
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

	buttons := make([][]tgbotapi.InlineKeyboardButton, nhosts/2+nhosts%2)
	for i, host := range hosts {
		buttons[i/2] = append(buttons[i/2],
			tgbotapi.NewInlineKeyboardButtonData(
				host,
				fmt.Sprintf("x=bitclouds-%s-%s", data, escapeBitcloudsHost(host)),
			),
		)
	}

	keyboard = &tgbotapi.InlineKeyboardMarkup{buttons}
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

func bitcloudsCheckingRoutine() {
	ctx := context.WithValue(
		context.Background(),
		"origin", "background",
	)

	for {
		time.Sleep(1 * time.Hour)

		var users []User
		err := pg.Select(&users, `
SELECT `+USERFIELDS+`, jsonb_object_keys(appdata->'bitclouds') AS extra
FROM account
    `)
		if err != nil {
			log.Error().Err(err).Msg("failed to query hosts on bitclouds checking routine")
			return
		}

		for _, user := range users {
			host := user.Extra
			status, err := getBitcloudStatus(host)
			if err != nil {
				log.Error().Err(err).Msg("getting status on bitclouds checking routine")
				continue
			}

			switch {
			case status.HoursLeft <= 0 || status.IP == "":
				// it's gone
				_, err := pg.Exec(`
UPDATE account
SET appdata =
  jsonb_set(appdata, '{bitclouds}',
    jsonb_strip_nulls(
      jsonb_set(appdata->'bitclouds', ARRAY[$2], 'null')
    )
  )
WHERE id = $1
            `, user.Id, host)
				if err != nil {
					log.Error().Err(err).Msg("deleting expired bitclouds host")
				}
			case status.HoursLeft < 24*14 && status.HoursLeft > 24*14-6 && !rds.Exists("bitclouds:2w:"+host).Val():
				send(ctx, user, t.BITCLOUDSREMINDER, t.T{
					"Alarm":        false,
					"Host":         host,
					"EscapedHost":  escapeBitcloudsHost(host),
					"TimeToExpire": "2 weeks",
					"Sats":         BITCLOUDSHOURPRICESATS * 24 * 7,
				})
				rds.Set("bitclouds:2w:"+host, "1", time.Hour*48)
			case status.HoursLeft < 24*7 && status.HoursLeft > 24*7-6 && !rds.Exists("bitclouds:1w:"+host).Val():
				send(ctx, user, t.BITCLOUDSREMINDER, t.T{
					"Alarm":        false,
					"Host":         host,
					"EscapedHost":  escapeBitcloudsHost(host),
					"TimeToExpire": "1 week",
					"Sats":         BITCLOUDSHOURPRICESATS * 24 * 7,
				})
				rds.Set("bitclouds:1w:"+host, "1", time.Hour*48)
			case status.HoursLeft < 24*3 && status.HoursLeft > 24*3-6 && !rds.Exists("bitclouds:3d:"+host).Val():
				send(ctx, user, t.BITCLOUDSREMINDER, t.T{
					"Alarm":        false,
					"Host":         host,
					"EscapedHost":  escapeBitcloudsHost(host),
					"TimeToExpire": "3 days",
					"Sats":         BITCLOUDSHOURPRICESATS * 24 * 7,
				})
				rds.Set("bitclouds:3d:"+host, "1", time.Hour*24)
			case status.HoursLeft < 25:
				send(ctx, user, t.BITCLOUDSREMINDER, t.T{
					"Alarm":        true,
					"Host":         host,
					"EscapedHost":  escapeBitcloudsHost(host),
					"TimeToExpire": fmt.Sprintf("%dh", status.HoursLeft),
					"Sats":         BITCLOUDSHOURPRICESATS * 24 * 7,
				})
			}
		}
	}
}

func escapeBitcloudsHost(host string) string {
	return strings.Replace(host, "-", "", -1)
}

var bitcloudsUnescaper = regexp.MustCompile(`^([a-zA-Z]+)(\d+)$`)

func unescapeBitcloudsHost(ehost string) string {
	return bitcloudsUnescaper.ReplaceAllString(ehost, "$1-$2")
}
