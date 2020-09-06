module github.com/fiatjaf/lntxbot

go 1.14

require (
	github.com/PuerkitoBio/goquery v1.5.1
	github.com/arschles/assert v2.0.0+incompatible // indirect
	github.com/arschles/go-bindata-html-template v0.0.0-20170123182818-839a6918b9ff
	github.com/btcsuite/btcd v0.20.1-beta.0.20200515232429-9f0179fd2c46
	github.com/docopt/docopt-go v0.0.0-20180111231733-ee0de3bc6815
	github.com/elazarl/go-bindata-assetfs v1.0.0
	github.com/fiatjaf/eventsource v0.0.0-20200623030538-9845829a8ba8
	github.com/fiatjaf/go-lnurl v1.1.0
	github.com/fiatjaf/lightningd-gjson-rpc v1.1.0
	github.com/fiatjaf/ln-decodepay v1.1.0
	github.com/go-telegram-bot-api/telegram-bot-api v4.6.4+incompatible
	github.com/gorilla/mux v1.7.4
	github.com/jmcvetta/randutil v0.0.0-20150817122601-2bb1b664bcff // indirect
	github.com/jmoiron/sqlx v1.2.0
	github.com/joho/godotenv v1.3.0
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/kr/pretty v0.1.0
	github.com/lib/pq v1.7.0
	github.com/lithammer/fuzzysearch v1.1.0
	github.com/lucsky/cuid v1.0.2
	github.com/msingleton/amplitude-go v0.0.0-20200312121213-b7c11448c30e
	github.com/orcaman/concurrent-map v0.0.0-20190826125027-8c72a8bb44f6
	github.com/rs/zerolog v1.19.0
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	github.com/tidwall/gjson v1.6.1
	github.com/tuotoo/qrcode v0.0.0-20190222102259-ac9c44189bf2
	github.com/willf/bitset v1.1.10 // indirect
	gopkg.in/jmcvetta/napping.v3 v3.2.0
	gopkg.in/redis.v5 v5.2.9
)

replace github.com/fiatjaf/go-lnurl => /home/fiatjaf/comp/go-lnurl

replace github.com/fiatjaf/lightningd-gjson-rpc => /home/fiatjaf/comp/lightningd-gjson-rpc
