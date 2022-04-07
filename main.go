package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/fiatjaf/go-cliche"
	"github.com/fiatjaf/go-lnurl"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/kelseyhightower/envconfig"
	_ "github.com/lib/pq"
	"github.com/msingleton/amplitude-go"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
	"gopkg.in/redis.v5"
)

type Settings struct {
	ServiceId        string   `envconfig:"SERVICE_ID" default:"lntxbot"`
	ServiceURL       string   `envconfig:"SERVICE_URL" required:"true"`
	Host             string   `envconfig:"HOST" default:"0.0.0.0"`
	Port             string   `envconfig:"PORT" required:"true"`
	TorProxyURL      *url.URL `envconfig:"TOR_PROXY_URL"`
	TelegramBotToken string   `envconfig:"TELEGRAM_BOT_TOKEN" required:"true"`
	PostgresURL      string   `envconfig:"DATABASE_URL" required:"true"`
	RedisURL         string   `envconfig:"REDIS_URL" required:"true"`
	DiscordBotToken  string   `envconfig:"DISCORD_BOT_TOKEN" required:"false"`
	ClicheJARPath    string   `envconfig:"CLICHE_JAR_PATH" required:"true"`
	ClicheDataDir    string   `envconfig:"CLICHE_DATADIR" required:"true"`

	// account in the database named '@'
	ProxyAccount int `envconfig:"PROXY_ACCOUNT" required:"true"`
	AdminAccount int `envconfig:"ADMIN_ACCOUNT"`

	LNPayKey           string `envconfig:"LNPAY_KEY"`
	AmplitudeKey       string `envconfig:"AMPLITUDE_KEY"`
	BitrefillBasicAuth string `envconfig:"BITREFILL_BASIC_AUTH"`

	InvoiceTimeout       time.Duration `envconfig:"INVOICE_TIMEOUT" default:"480h"`
	PayConfirmTimeout    time.Duration `envconfig:"PAY_CONFIRM_TIMEOUT" default:"10m"`
	GiveAwayTimeout      time.Duration `envconfig:"GIVE_AWAY_TIMEOUT" default:"5h"`
	HiddenMessageTimeout time.Duration `envconfig:"HIDDEN_MESSAGE_TIMEOUT" default:"72h"`

	CoinflipDailyQuota int `envconfig:"COINFLIP_DAILY_QUOTA" default:"5"` // times each user can join a coinflip
	CoinflipAvgDays    int `envconfig:"COINFLIP_AVG_DAYS" default:"7"`    // days we'll consider for the average
	GiveflipDailyQuota int `envconfig:"GIVEFLIP_DAILY_QUOTA" default:"5"`
	GiveflipAvgDays    int `envconfig:"GIVEFLIP_AVG_DAYS" default:"7"`
	GiveawayDailyQuota int `envconfig:"GIVEAWAY_DAILY_QUOTA" default:"5"`
	GiveawayAvgDays    int `envconfig:"GIVEAWAY_AVG_DAYS" default:"7"`

	Banned map[int]bool `envconfig:"BANNED"`

	Usage string
}

var s Settings
var pg *sqlx.DB
var ln *cliche.Control
var rds *redis.Client
var bot *tgbotapi.BotAPI
var discord *discordgo.Session
var amp *amplitude.Client
var log = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: PluginLogger{}})
var router = mux.NewRouter()
var waitingPaymentSuccesses = cmap.New() //  make(map[string][]chan string)
var bundle t.Bundle
var isInternal func(payee string) bool = func(payee string) bool { return false }

//go:embed templates
var templates embed.FS
var tmpl = template.Must(template.ParseFS(templates, "templates/*"))

//go:embed static
var static embed.FS

func main() {
	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't process envconfig.")
	}

	// increase default lnurl client timeout because people are using tor unfortunately
	lnurl.Client = &http.Client{Timeout: 25 * time.Second}
	lnurl.TorClient = &http.Client{
		Timeout: 50 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(s.TorProxyURL),
		},
	}

	// setup logger
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log = log.With().Timestamp().Logger()

	// http client
	http.DefaultClient.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		return fmt.Errorf("target '%s' has returned a redirect", r.URL)
	}

	// translations and templates
	bundle, err = createLocalizerBundle()
	if err != nil {
		log.Fatal().Err(err).Msg("error initializing localization")
	}

	// seed the random generator
	rand.Seed(time.Now().UnixNano())

	// setup cliche
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
		isInternal = func(payee string) bool {
			for _, channel := range nodeinfo.Channels {
				if channel.Peer.OurPubkey == payee {
					return true
				}
			}
			return false
		}
	}
	go handleClicheEvents()

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

	// setup commands
	setupCommands()

	// create telegram bot
	bot, err = tgbotapi.NewBotAPI(s.TelegramBotToken)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	log.Info().Str("username", bot.Self.UserName).Msg("telegram bot authorized")

	// setup telegram webhook
	go func() {
		time.Sleep(1 * time.Second)
		// set webhook
		_, err = bot.SetWebhook(tgbotapi.NewWebhook(s.ServiceURL + "/" + bot.Token))
		if err != nil {
			log.Fatal().Err(err).Msg("failed to set webhook")
		}
		_, err := bot.GetWebhookInfo()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to get webhook info")
		}
	}()

	// discord bot session
	if s.DiscordBotToken != "" {
		discord, err = discordgo.New("Bot " + s.DiscordBotToken)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create discord session")
		}

		addDiscordHandlers()

		err = discord.Open()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to establish discord connection")
		}
		log.Info().Msg("discord connection initialized")
		defer discord.Close()
	}

	// routines
	routineCtx := context.WithValue(context.Background(), "origin", "routine")
	go startKicking()
	go sats4adsCleanupRoutine()
	go lnurlBalanceCheckRoutine()
	go checkAllOutgoingPayments(routineCtx)
	go checkAllIncomingPayments(routineCtx)

	// routes
	//
	// telegram webhooks
	router.Path("/" + bot.Token).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes, _ := ioutil.ReadAll(r.Body)
		var update tgbotapi.Update
		json.Unmarshal(bytes, &update)
		handle(update)
	})

	// lndhub-compatible routes
	registerAPIMethods()

	// register webserver routes
	serveQRCodes()
	serveTempAssets()
	serveLNURL()
	serveLNURLBalanceNotify()
	servePages()
	router.Path("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://t.me/lntxbot", http.StatusTemporaryRedirect)
	})

	// random assets
	router.PathPrefix("/static/").Handler(http.FileServer(http.FS(static)))

	// start http server
	srv := &http.Server{
		Handler:      cors.Default().Handler(router),
		Addr:         s.Host + ":" + s.Port,
		WriteTimeout: 300 * time.Second,
		ReadTimeout:  300 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Error().Err(err).Msg("error serving http")
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
	bundle.AddFunc("menuItem", func(sats interface{}, rawItem string, showSats bool) string {
		var satShow string
		switch s := sats.(type) {
		case int:
			satShow = strconv.Itoa(s) + " sat"
		case int64:
			satShow = strconv.FormatInt(s, 10) + " sat"
		case float64:
			satShow = fmt.Sprintf("%.3g sat", s)
		}

		if _, ok := menuItems[rawItem]; ok {
			if showSats {
				return rawItem + " (" + satShow + ")"
			} else {
				return rawItem
			}
		}

		return satShow
	})
	bundle.AddFunc("messageLink", telegramMessageLink)

	err := bundle.AddLanguage("en", t.EN)
	if err != nil {
		return bundle, err
	}
	err = bundle.AddLanguage("ru", t.RU)
	if err != nil {
		return bundle, err
	}
	err = bundle.AddLanguage("de", t.DE)
	if err != nil {
		return bundle, err
	}
	err = bundle.AddLanguage("es", t.ES)
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
