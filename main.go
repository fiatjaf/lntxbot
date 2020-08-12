package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	template "github.com/arschles/go-bindata-html-template"
	assetfs "github.com/elazarl/go-bindata-assetfs"
	lightning "github.com/fiatjaf/lightningd-gjson-rpc"
	"github.com/fiatjaf/lightningd-gjson-rpc/plugin"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	_ "github.com/lib/pq"
	"github.com/msingleton/amplitude-go"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/rs/zerolog"
	"gopkg.in/redis.v5"
)

type Settings struct {
	ServiceId   string `envconfig:"SERVICE_ID" default:"lntxbot"`
	ServiceURL  string `envconfig:"SERVICE_URL" required:"true"`
	Host        string `envconfig:"HOST" default:"0.0.0.0"`
	Port        string `envconfig:"PORT" required:"true"`
	BotToken    string `envconfig:"BOT_TOKEN" required:"true"`
	PostgresURL string `envconfig:"DATABASE_URL" required:"true"`
	RedisURL    string `envconfig:"REDIS_URL" required:"true"`

	// account in the database named '@'
	ProxyAccount int `envconfig:"PROXY_ACCOUNT" required:"true"`

	LNPayKey           string `envconfig:"LNPAY_KEY"`
	AmplitudeKey       string `envconfig:"AMPLITUDE_KEY"`
	BitrefillBasicAuth string `envconfig:"BITREFILL_BASIC_AUTH"`

	InvoiceTimeout       time.Duration `envconfig:"INVOICE_TIMEOUT" default:"480h"`
	PayConfirmTimeout    time.Duration `envconfig:"PAY_CONFIRM_TIMEOUT" default:"10m`
	GiveAwayTimeout      time.Duration `envconfig:"GIVE_AWAY_TIMEOUT" default:"5h"`
	HiddenMessageTimeout time.Duration `envconfig:"HIDDEN_MESSAGE_TIMEOUT" default:"72h"`

	CoinflipDailyQuota int `envconfig:"COINFLIP_DAILY_QUOTA" default:"5"` // times each user can join a coinflip
	CoinflipAvgDays    int `envconfig:"COINFLIP_AVG_DAYS" default:"7"`    // days we'll consider for the average
	GiveflipDailyQuota int `envconfig:"GIVEFLIP_DAILY_QUOTA" default:"5"`
	GiveflipAvgDays    int `envconfig:"GIVEFLIP_AVG_DAYS" default:"7"`
	GiveawayDailyQuota int `envconfig:"GIVEAWAY_DAILY_QUOTA" default:"5"`
	GiveawayAvgDays    int `envconfig:"GIVEAWAY_AVG_DAYS" default:"7"`

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
var amp *amplitude.Client
var log = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stderr})
var tmpl = template.Must(template.New("", Asset).ParseFiles("templates/donation.html"))
var router = mux.NewRouter()
var waitingInvoices = cmap.New()         // make(map[string][]chan gjson.Result)
var waitingPaymentSuccesses = cmap.New() //  make(map[string][]chan string)
var bundle t.Bundle

func main() {
	p := plugin.Plugin{
		Name:    "lntxbot",
		Version: "v1.0",
		Dynamic: true,
		Subscriptions: []plugin.Subscription{
			{
				"invoice_payment",
				func(p *plugin.Plugin, params plugin.Params) {
					label := params.Get("invoice_payment.label").String()
					invspaid, err := ln.Call("listinvoices", label)
					if err != nil {
						log.Error().Err(err).Str("label", label).
							Msg("failed to query paid invoice")
						return
					}

					inv := invspaid.Get("invoices.0")
					go handleInvoicePaid(
						inv.Get("pay_index").Int(),
						inv.Get("msatoshi_received").Int(),
						inv.Get("description").String(),
						inv.Get("payment_hash").String(),
						inv.Get("payment_preimage").String(),
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
	envpath := "lntxbot.env"
	if !filepath.IsAbs(envpath) {
		// expand tlspath from lightning dir
		godotenv.Load(filepath.Join(filepath.Dir(p.Client.Path), envpath))
	} else {
		godotenv.Load(envpath)
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

	// amplitude client
	if s.AmplitudeKey != "" {
		amp = amplitude.New(s.AmplitudeKey)
	}

	// create bot
	bot, err = tgbotapi.NewBotAPI(s.BotToken)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	log.Info().Str("username", bot.Self.UserName).Msg("telegram bot authorized")

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

	// register webserver routes
	serveLNURL()
	servePages()
	serveGiftsWebhook()
	serveBitrefillWebhook()
	router.Path("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://t.me/lntxbot", http.StatusTemporaryRedirect)
	})

	// routines
	go startKicking()
	go sats4adsCleanupRoutine()
	go initializeBitrefill()
	go bitcloudsCheckingRoutine()
	go startListeningToEtleneumContracts()

	// random assets
	router.PathPrefix("/static/").Handler(http.FileServer(&assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, AssetInfo: AssetInfo}))

	// start http server
	srv := &http.Server{
		Handler:      router,
		Addr:         s.Host + ":" + s.Port,
		WriteTimeout: 300 * time.Second,
		ReadTimeout:  300 * time.Second,
	}
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Error().Err(err).Msg("error serving http")
		}
	}()

	// pause here until lightningd works
	s.NodeId = probeLightningd()

	// bot stuff
	lastTelegramUpdate, err := rds.Get("lasttelegramupdate").Int64()
	if err != nil || lastTelegramUpdate < 10 {
		log.Error().Err(err).Int64("got", lastTelegramUpdate).
			Msg("failed to get lasttelegramupdate from redis")
		lastTelegramUpdate = -1
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
		handle(update)
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
	bundle.AddFunc("msatToSat", func(imsat interface{}) float64 {
		switch msat := imsat.(type) {
		case int64:
			return float64(msat) / 1000
		case int:
			return float64(msat) / 1000
		case float64:
			return msat / 1000
		default:
			return 0
		}
	})
	bundle.AddFunc("escapehtml", escapeHTML)
	bundle.AddFunc("nodeLink", nodeLink)
	bundle.AddFunc("nodeAlias", getNodeAlias)
	bundle.AddFunc("channelLink", channelLink)
	bundle.AddFunc("nodeAliasLink", nodeAliasLink)
	bundle.AddFunc("makeLinks", makeLinks)
	bundle.AddFunc("json", func(v interface{}) string {
		j, _ := json.MarshalIndent(v, "", "  ")
		return string(j)
	})
	bundle.AddFunc("time", func(t time.Time) string {
		return t.Format("2 Jan 2006 at 3:04PM")
	})
	bundle.AddFunc("timeSmall", func(t time.Time) string {
		return t.Format("2 Jan 15:04")
	})
	bundle.AddFunc("paddedSatoshis", func(amount float64) string {
		if amount > 99999 {
			return fmt.Sprintf("%7.15g", amount)
		}
		if amount < -9999 {
			return fmt.Sprintf("%7.15g", amount)
		}
		return fmt.Sprintf("%7.15g", amount)
	})
	bundle.AddFunc("lower", strings.ToLower)
	bundle.AddFunc("roman", roman)
	bundle.AddFunc("letter", func(i int) string { return string([]rune{rune(i) + 97}) })
	bundle.AddFunc("add", func(a, b int) int { return a + b })
	bundle.AddFunc("menuItem", func(sats int, rawItem string, showSats bool) string {
		if _, ok := menuItems[rawItem]; ok {
			if showSats {
				return rawItem + " (" + strconv.Itoa(sats) + " sat)"
			} else {
				return rawItem
			}
		}
		return strconv.Itoa(sats) + " sat"
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
