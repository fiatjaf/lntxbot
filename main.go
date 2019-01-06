package main

import (
	"net/http"
	"os"

	"github.com/fiatjaf/lntxbot-telegram/lightning"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jmoiron/sqlx"
	"github.com/kelseyhightower/envconfig"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
)

type Settings struct {
	ServiceURL  string `envconfig:"SERVICE_URL" required:"true"`
	Port        string `envconfig:"PORT" required:"true"`
	BotToken    string `envconfig:"BOT_TOKEN" required:"true"`
	PostgresURL string `envconfig:"DATABASE_URL" required:"true"`
	SocketPath  string `envconfig:"SOCKET_PATH" required:"true"`
}

var err error
var s Settings
var pg *sqlx.DB
var ln *lightning.Client
var log = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stderr})

func main() {
	err = envconfig.Process("", &s)
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't process envconfig.")
	}

	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log = log.With().Timestamp().Logger()

	// postgres connection
	pg, err = sqlx.Connect("postgres", s.PostgresURL)
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't connect to postgres")
	}

	// lightningd connection
	ln, err = lightning.Connect(s.SocketPath)
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't connect to lightning-rpc")
	}
	ln.Listen(func(err error) {
		log.Error().Err(err).Msg("error reading lightning-rpc")
	})

	// bot stuff
	bot, err := tgbotapi.NewBotAPI(s.BotToken)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	_, err = bot.SetWebhook(tgbotapi.NewWebhook(s.ServiceURL + "/" + bot.Token))
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	info, err := bot.GetWebhookInfo()
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	if info.LastErrorDate != 0 {
		log.Printf("Telegram callback failed: %s", info.LastErrorMessage)
	}
	updates := bot.ListenForWebhook("/" + bot.Token)
	go http.ListenAndServe("0.0.0.0:"+s.Port, nil)

	// initial call
	nodeinfo := <-ln.Call("getinfo", lightning.Params{})
	log.Info().
		Str("id", nodeinfo.Get("id").String()).
		Str("alias", nodeinfo.Get("alias").String()).
		Int64("channels", nodeinfo.Get("num_active_channels").Int()).
		Int64("blockheight", nodeinfo.Get("blockheight").Int()).
		Str("version", nodeinfo.Get("version").String()).
		Msg("lightning node connected")

	for update := range updates {
		log.Printf("%+v\n", update)
		handle(update)
	}
}
