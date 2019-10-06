package main

import (
	"errors"
	"fmt"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

type Sats4AdsData struct {
	On     bool `json:"on"`
	Rate   int  `json:"rate"` // in msatoshi per character
	Banned bool `json:"banned,omitempty"`
}

type Sats4AdsRateGroup struct {
	NUsers   int `db:"nusers"`
	UpToRate int `db:"uptorate"` // in msatoshi per character
}

func turnSats4AdsOn(user User, rate int) error {
	var data Sats4AdsData
	err := user.getAppData("sats4ads", &data)
	if err != nil {
		return err
	}

	if data.Banned {
		return errors.New("user banned")
	}

	data.On = true
	data.Rate = rate
	return user.setAppData("sats4ads", data)
}

func turnSats4AdsOff(user User) error {
	var data Sats4AdsData
	err := user.getAppData("sats4ads", &data)
	if err != nil {
		return err
	}

	data.On = false
	return user.setAppData("sats4ads", data)
}

func broadcastSats4Ads(
	user User,
	budgetSatoshis int,
	contentMessage *tgbotapi.Message,
	maxrate int,
	offset int,
) (messagesSent int, roundedCostSatoshis int, errMsg string, err error) {
	costSatoshis := 0.0

	if maxrate == 0 {
		maxrate = 500
	}

	// decide on a unique hash for the source payment (so payments can be aggregated
	// like Payer-3->Proxy, then Proxy-1->TargetA, Proxy-2->TargetB, Proxy-3->TargetC)
	random, err := randomPreimage()
	if err != nil {
		return
	}
	sourcehash := calculateHash(random)

	logger := log.With().Str("sourcehash", sourcehash).Int("budget", budgetSatoshis).Int("max", maxrate).Logger()

	rows, err := pg.Queryx(`
SELECT id, (appdata->'sats4ads'->>'rate')::int AS rate
FROM telegram.account
WHERE appdata->'sats4ads'->'on' = 'true'::jsonb
  AND id != $1
  AND (appdata->'sats4ads'->>'rate')::integer <= $2
ORDER BY appdata->'sats4ads'->'rate' ASC, random()
OFFSET $3
    `, user.Id, maxrate, offset)
	if err != nil {
		return
	}

	// send messages and queue receivers to be paid
	for rows.Next() {
		var row struct {
			Id   int `db:"id"`
			Rate int `db:"rate"`
		}

		err = rows.StructScan(&row)
		if err != nil {
			return
		}

		// fetch the target user
		var target User
		target, err = loadUser(row.Id, 0)
		if err != nil || target.ChatId == 0 {
			continue
		}

		// build ad message based on the message that was replied to
		var nchars int
		var ad tgbotapi.Chattable
		var thisCostMsat int = 1000 // fixed 1sat fee for each message
		var thisCostSatoshis float64
		baseChat := tgbotapi.BaseChat{ChatID: target.ChatId}

		switch {
		case contentMessage.Text != "":
			nchars = len(contentMessage.Text)
			thisCostMsat += row.Rate * nchars
			thisCostSatoshis = float64(thisCostMsat) / 1000
			footer := "\n\n" + translateTemplate(t.SATS4ADSADFOOTER, target.Locale, t.T{
				"Sats": thisCostSatoshis,
			})

			ad = tgbotapi.MessageConfig{
				BaseChat: baseChat,
				Text:     contentMessage.Text + footer,
				DisableWebPagePreview: true,
			}
		case contentMessage.Animation != nil:
			nchars = 300 + len(contentMessage.Caption)
			thisCostMsat += row.Rate * nchars
			thisCostSatoshis = float64(thisCostMsat) / 1000
			footer := "\n\n" + translateTemplate(t.SATS4ADSADFOOTER, target.Locale, t.T{
				"Sats": thisCostSatoshis,
			})

			ad = tgbotapi.AnimationConfig{
				Caption: contentMessage.Caption + footer,
				BaseFile: tgbotapi.BaseFile{
					BaseChat:    baseChat,
					FileID:      contentMessage.Animation.FileID,
					UseExisting: true,
				},
			}
		case contentMessage.Photo != nil:
			nchars = 300 + len(contentMessage.Caption)
			thisCostMsat += row.Rate * nchars
			thisCostSatoshis = float64(thisCostMsat) / 1000
			footer := "\n\n" + translateTemplate(t.SATS4ADSADFOOTER, target.Locale, t.T{
				"Sats": thisCostSatoshis,
			})
			photos := *contentMessage.Photo

			ad = tgbotapi.PhotoConfig{
				Caption: contentMessage.Caption + footer,
				BaseFile: tgbotapi.BaseFile{
					BaseChat:    baseChat,
					FileID:      photos[0].FileID,
					UseExisting: true,
				},
			}
		case contentMessage.Video != nil:
			nchars = 300 + len(contentMessage.Caption)
			thisCostMsat += row.Rate * nchars
			thisCostSatoshis = float64(thisCostMsat) / 1000
			footer := "\n\n" + translateTemplate(t.SATS4ADSADFOOTER, target.Locale, t.T{
				"Sats": thisCostSatoshis,
			})

			ad = tgbotapi.VideoConfig{
				Caption: contentMessage.Caption + footer,
				BaseFile: tgbotapi.BaseFile{
					BaseChat:    baseChat,
					FileID:      contentMessage.Video.FileID,
					UseExisting: true,
				},
			}
		case contentMessage.Document != nil:
			nchars = 200 + len(contentMessage.Caption)
			thisCostMsat += row.Rate * nchars
			thisCostSatoshis = float64(thisCostMsat) / 1000
			footer := "\n\n" + translateTemplate(t.SATS4ADSADFOOTER, target.Locale, t.T{
				"Sats": thisCostSatoshis,
			})

			ad = tgbotapi.DocumentConfig{
				Caption: contentMessage.Caption + footer,
				BaseFile: tgbotapi.BaseFile{
					BaseChat:    baseChat,
					FileID:      contentMessage.Document.FileID,
					UseExisting: true,
				},
			}
		case contentMessage.Audio != nil:
			nchars = 150 + len(contentMessage.Caption)
			thisCostMsat += row.Rate * nchars
			thisCostSatoshis = float64(thisCostMsat) / 1000
			footer := "\n\n" + translateTemplate(t.SATS4ADSADFOOTER, target.Locale, t.T{
				"Sats": thisCostSatoshis,
			})

			ad = tgbotapi.AudioConfig{
				Caption: contentMessage.Caption + footer,
				BaseFile: tgbotapi.BaseFile{
					BaseChat:    baseChat,
					FileID:      contentMessage.Audio.FileID,
					UseExisting: true,
				},
			}
		default:
			logger.Info().Msg("invalid message used as ad content")
			break
		}

		if int(costSatoshis+thisCostSatoshis) > budgetSatoshis {
			// budget ended, stop queueing messages
			logger.Info().Float64("spent", costSatoshis).Float64("next", thisCostSatoshis).Msg("budget ended")
			break
		}

		var message tgbotapi.Message
		message, err = bot.Send(ad)
		if err != nil {
			// message wasn't sent
			logger.Info().Err(err).Msg("message wasn't sent. skipping.")
			continue
		}

		// commit payment
		var random string
		random, err = randomPreimage()
		if err != nil {
			return
		}
		errMsg, err = user.sendThroughProxy(
			sourcehash,
			calculateHash(random),
			contentMessage.MessageID,
			message.MessageID,
			target,
			thisCostMsat,
			fmt.Sprintf("ad dispatched to %d", messagesSent+1),
			fmt.Sprintf("%d characters ad (%s) at %d msat/char", nchars, sourcehash, row.Rate),
			"sats4ads",
		)
		if err != nil {
			logger.Error().Err(err).Msg("error saving proxied payment. abort all.")
			return
		}

		messagesSent += 1
		costSatoshis += thisCostSatoshis

		logger.Debug().Float64("cost", thisCostSatoshis).Int("rate", row.Rate).
			Float64("total", costSatoshis).Int("n", messagesSent).
			Msg("ad broadcasted")
	}

	roundedCostSatoshis = int(costSatoshis)
	return
}

func getSats4AdsRates(user User) (rates []Sats4AdsRateGroup, err error) {
	err = pg.Select(&rates, `
WITH enabled_listeners AS (
  SELECT (appdata->'sats4ads'->>'rate')::integer AS rate
  FROM telegram.account
  WHERE appdata->'sats4ads'->'on' = 'true'::jsonb
    AND id != $1
), rategroups AS (
  SELECT generate_series ^ 3 AS uptorate FROM generate_series(1, 10)
)

SELECT uptorate, (SELECT count(*) FROM enabled_listeners WHERE rate <= uptorate) AS nusers
FROM rategroups
    `, user.Id)
	return
}
