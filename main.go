package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/arschles/go-bindata-html-template"
	"github.com/elazarl/go-bindata-assetfs"
	"github.com/fiatjaf/lightningd-gjson-rpc"
	"github.com/fiatjaf/lightningd-gjson-rpc/plugin"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/tidwall/gjson"
	"gopkg.in/redis.v5"
)

type Settings struct {
	ServiceId   string `envconfig:"SERVICE_ID" default:"lntxbot"`
	ServiceURL  string `envconfig:"SERVICE_URL" required:"true"`
	Port        string `envconfig:"PORT" required:"true"`
	BotToken    string `envconfig:"BOT_TOKEN" required:"true"`
	PostgresURL string `envconfig:"DATABASE_URL" required:"true"`
	RedisURL    string `envconfig:"REDIS_URL" required:"true"`

	// account in the database named '@'
	ProxyAccount int `envconfig:"PROXY_ACCOUNT" required:"true"`

	PaywallLinkKey     string `envconfig:"PAYWALLLINK_KEY"`
	LNToRubKey         string `envconfig:"LNTORUB_KEY"`
	BitrefillBasicAuth string `envconfig:"BITREFILL_BASIC_AUTH"`

	InvoiceTimeout       time.Duration `envconfig:"INVOICE_TIMEOUT" default:"24h"`
	PayConfirmTimeout    time.Duration `envconfig:"PAY_CONFIRM_TIMEOUT" default:"5h"`
	GiveAwayTimeout      time.Duration `envconfig:"GIVE_AWAY_TIMEOUT" default:"5h"`
	HiddenMessageTimeout time.Duration `envconfig:"HIDDEN_MESSAGE_TIMEOUT" default:"72h"`

	CoinflipDailyQuota int `envconfig:"COINFLIP_DAILY_QUOTA" default:"5"` // times each user can join a coinflip
	CoinflipAvgDays    int `envconfig:"COINFLIP_AVG_DAYS" default:"7"`    // days we'll consider for the average
	GiveflipDailyQuota int `envconfig:"GIVEFLIP_DAILY_QUOTA" default:"5"`
	GiveflipAvgDays    int `envconfig:"GIVEFLIP_AVG_DAYS" default:"7"`

	TutorialWalletVideoId string `envconfig:"TUTORIAL_WALLET_VIDEO_ID"`
	TutorialBlueVideoId   string `envconfig:"TUTORIAL_BLUE_VIDEO_ID"`

	Banned map[int]bool `envconfig:"BANNED"`

	NodeId string
	Usage  string
}

var err error
var s Settings
var pg *sqlx.DB
var ln *lightning.Client
var rds *redis.Client
var bot *tgbotapi.BotAPI
var log = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stderr})
var tmpl = template.Must(template.New("", Asset).ParseFiles("templates/donation.html"))
var router = mux.NewRouter()
var waitingInvoices = make(map[string][]chan gjson.Result)
var waitingPaymentSuccesses = make(map[string][]chan string)
var bundle t.Bundle

func main() {
	p := plugin.Plugin{
		Name:    "lntxbot",
		Version: "v1.0",
		Options: []plugin.Option{
			{"lntxbot-envfile", "string", "", "Path to the file containing everything"},
		},
		Subscriptions: []plugin.Subscription{
			{
				"invoice_payment",
				func(p *plugin.Plugin, params plugin.Params) {
					label := params.Get("invoice_payment.label").String()
					invspaid, err := ln.Call("listinvoices", label)
					if err != nil {
						log.Error().Err(err).Str("label", label).Msg("failed to query paid invoice")
						return
					}

					inv := invspaid.Get("invoices.0")
					go handleInvoicePaid(
						inv.Get("pay_index").Int(),
						inv.Get("msatoshi_received").Int(),
						inv.Get("description").String(),
						inv.Get("payment_hash").String(),
						inv.Get("label").String(),
					)
					go resolveWaitingInvoice(inv.Get("payment_hash").String(), inv)
				},
			},
			{
				"sendpay_success",
				func(p *plugin.Plugin, params plugin.Params) {
					hash := params.Get("sendpay_success.payment_hash").String()
					preimage := params.Get("sendpay_success.payment_preimage").String()
					log.Print("SENDPAY_SUCCESS ", hash, " ", preimage)
					go resolveWaitingPaymentSuccess(hash, preimage)
				},
			},
		},
		OnInit: server,
	}

	p.Run()
}

func server(p *plugin.Plugin) {
	// globalize the lightning rpc client
	ln = p.Client

	// load values from envfile (hack)
	if envpath, err := p.Args.String("lntxbot-envfile"); err == nil {
		if !filepath.IsAbs(envpath) {
			// expand tlspath from lightning dir
			godotenv.Load(filepath.Join(filepath.Dir(p.Client.Path), envpath))
		} else {
			godotenv.Load(envpath)
		}
	} else {
		log.Fatal().Err(err).Msg("couldn't find envfile, specify lntxbot-envpath")
	}
	err = envconfig.Process("", &s)
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't process envconfig.")
	}

	bundle, err = createLocalizerBundle()
	if err != nil {
		log.Fatal().Err(err).Msg("error initializing localization")
	}

	setupCommands()

	// setup logger
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log = log.With().Timestamp().Logger()

	// seed the random generator
	rand.Seed(time.Now().UnixNano())

	// postgres connection
	pg, err = sqlx.Connect("postgres", s.PostgresURL)
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't connect to postgres")
	}

	// redis connection
	rurl, _ := url.Parse(s.RedisURL)
	pw, _ := rurl.User.Password()
	rds = redis.NewClient(&redis.Options{
		Addr:     rurl.Host,
		Password: pw,
	})
	if err := rds.Ping().Err(); err != nil {
		log.Fatal().Err(err).Str("url", s.RedisURL).
			Msg("failed to connect to redis")
	}

	// create bot
	bot, err = tgbotapi.NewBotAPI(s.BotToken)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	log.Info().Str("username", bot.Self.UserName).Msg("telegram bot authorized")

	// lightningd connection
	lastinvoiceindex, err := rds.Get("lastinvoiceindex").Int64()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get lastinvoiceindex from redis")
		return
	}
	if lastinvoiceindex < 10 {
		res, err := ln.Call("listinvoices")
		if err != nil {
			log.Fatal().Err(err).Msg("failed to get lastinvoiceindex from listinvoices")
			return
		}
		indexes := res.Get("invoices.#.pay_index").Array()
		for _, indexr := range indexes {
			index := indexr.Int()
			if index > lastinvoiceindex {
				lastinvoiceindex = index
			}
		}
	}

	// handle QR code requests from telegram
	router.Path("/qr/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path[3:]
		if strings.HasPrefix(path, "/tmp/") && strings.HasSuffix(path, ".png") {
			http.ServeFile(w, r, path)
		} else {
			http.Error(w, "not found", 404)
		}
	})

	// lndhub-compatible routes
	registerAPIMethods()

	// lnurl routes
	serveLNURL()

	// donation webpage
	registerPages()

	// app-specific initializations
	servePoker()
	servePaywallWebhook()
	serveGiftsWebhook()
	serveBitrefillWebhook()
	go cancelAllLNToRubOrders()
	go initializeBitrefill()
	go bitcloudsCheckingRoutine()

	// random assets
	router.PathPrefix("/static/").Handler(http.FileServer(&assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, AssetInfo: AssetInfo}))

	// start http server
	srv := &http.Server{
		Handler: router,
		Addr:    "0.0.0.0:" + s.Port,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Error().Err(err).Msg("error serving http")
		}
	}()

	// pause here until lightningd works
	s.NodeId = probeLightningd()

	// dispatch kick job for pending users
	startKicking()

	// bot stuff
	lastTelegramUpdate, err := rds.Get("lasttelegramupdate").Int64()
	if err != nil || lastTelegramUpdate < 10 {
		log.Fatal().Err(err).Int64("got", lastTelegramUpdate).Msg("failed to get lasttelegramupdate from redis")
		return
	}

	u := tgbotapi.NewUpdate(int(lastTelegramUpdate + 1))
	u.Timeout = 60
	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Error().Err(err).Msg("telegram getupdates fail")
	}
	for update := range updates {
		lastTelegramUpdate = int64(update.UpdateID)
		go rds.Set("lasttelegramupdate", lastTelegramUpdate, 0)
		func() {
			defer func() {
				if err := recover(); err != nil {
					fmt.Fprint(os.Stderr, string(debug.Stack()))
					sendMessage(418342569, string(debug.Stack()))
				}
			}()
			handle(update)
		}()
	}
}

func probeLightningd() string {
	nodeinfo, err := ln.Call("getinfo")
	if err != nil {
		log.Warn().Err(err).Msg("can't talk to lightningd. retrying.")
		time.Sleep(time.Second * 5)
		return probeLightningd()
	}
	log.Info().
		Str("id", nodeinfo.Get("id").String()).
		Str("alias", nodeinfo.Get("alias").String()).
		Int64("channels", nodeinfo.Get("num_active_channels").Int()).
		Int64("blockheight", nodeinfo.Get("blockheight").Int()).
		Str("version", nodeinfo.Get("version").String()).
		Msg("lightning node connected")

	return nodeinfo.Get("id").String()
}

func createLocalizerBundle() (t.Bundle, error) {
	// bundle stores a set of messages
	bundle = t.NewBundle("en")

	// template functions
	bundle.AddFunc("s", func(iquantity interface{}) string {
		switch quantity := iquantity.(type) {
		case int64:
			if quantity != 1 {
				return "s"
			}
		case int:
			if quantity != 1 {
				return "s"
			}
		case float64:
			if quantity != 1 {
				return "s"
			}
		}
		return ""
	})
	bundle.AddFunc("dollar", func(isat interface{}) string {
		switch sat := isat.(type) {
		case int64:
			return getDollarPrice(sat * 1000)
		case int:
			return getDollarPrice(int64(sat) * 1000)
		case float64:
			return getDollarPrice(int64(sat * 1000))
		default:
			return "~"
		}
	})

	err := bundle.AddLanguage("en", t.EN)
	if err != nil {
		return bundle, err
	}
	err = bundle.AddLanguage("ru", t.RU)
	if err != nil {
		return bundle, err
	}

	// print an annoying message if keys are missing from translations
	for lang, missing := range bundle.Check() {
		log.Debug().Str("lang", lang).Interface("keys", missing).
			Msg("missing translation")
	}

	return bundle, nil
}
