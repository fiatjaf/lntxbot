package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/fiatjaf/go-cliche"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/kballard/go-shellquote"
)

func setupCliche() {
	ln = &cliche.Control{
		JARPath: s.ClicheJARPath,
		DataDir: s.ClicheDataDir,
	}
	log.Info().Msg("starting cliche")
	if err := ln.Start(); err != nil {
		log.Fatal().Err(err).Msg("failed to start cliche")
	}
	if nodeinfo, err := ln.GetInfo(); err != nil {
		log.Fatal().Err(err).Msg("can't talk to cliche")
	} else {
		log.Info().
			Int("blockHeight", nodeinfo.BlockHeight).
			Int("channels", len(nodeinfo.Channels)).
			Msg("cliche connected")
	}
}

func handleClicheEvents() {
	ctx := context.WithValue(context.Background(), "origin", "cliche")

	go func() {
		for event := range ln.IncomingPayments {
			go paymentReceived(ctx, event.PaymentHash, event.Msatoshi)
		}
	}()

	go func() {
		for event := range ln.PaymentSuccesses {
			go paymentHasSucceeded(
				ctx,
				event.Msatoshi,
				event.FeeMsatoshi,
				event.Preimage,
				"",
				event.PaymentHash,
			)
		}
	}()

	go func() {
		for event := range ln.PaymentFailures {
			go paymentHasFailed(ctx, event.PaymentHash, event.Failure)
		}
	}()
}

func handleClicheCommand(
	ctx context.Context,
	message *tgbotapi.Message,
	messageText string,
) {
	u := ctx.Value("initiator").(*User)

	spl := strings.SplitN(messageText[8:], " ", 2)
	method := spl[0]
	params := make(map[string]interface{})

	if len(spl) == 2 {
		argv, err := shellquote.Split(spl[1])
		if err != nil {
			send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
			return
		}

		if len(argv)%2 != 0 {
			send(ctx, u, t.ERROR, t.T{"Err": "invalid number of args"})
			return
		}

		for i := range argv {
			if i%2 != 0 || !strings.HasPrefix(argv[i], "--") {
				continue
			}

			key := argv[i][2:]
			val := argv[i+1]

			var value interface{}
			if err := json.Unmarshal([]byte(val), &value); err != nil {
				value = val
			}

			params[key] = value
		}
	}

	resp, err := ln.Call(method, params)
	if err != nil {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
		return
	}

	var jresp interface{}
	pretty := resp
	if err := json.Unmarshal(resp, &jresp); err == nil {
		pretty, _ = json.MarshalIndent(jresp, "", "  ")
	}

	send(ctx, u, "<pre><code class=\"language-json\">\n"+string(pretty)+"\n</code></pre>")
}

func clicheCheckingRoutine() {
	ctx := context.Background()

	for {
		time.Sleep(5 * time.Minute)

		select {
		case err := <-clichePing():
			if err != nil {
				log.Error().Err(err).Msg("cliche ping returned error")
				break
			} else {
				log.Debug().Msg("cliche is fine")
				continue
			}
		case <-time.After(3 * time.Minute):
			log.Error().Msg("cliche is not responding after 3 minutes")
			break
		}

		// message admin
		if admin, err := loadUser(s.AdminAccount); err == nil {
			send(ctx, admin, "cliche has failed, bot turning off")
		}

		// exit cleanly here so systemd won't restart the service
		os.Exit(0)
	}
}

func clichePing() chan error {
	ch := make(chan error)
	go func() {
		_, err := ln.Call("ping", map[string]interface{}{})
		ch <- err
	}()
	return ch
}
