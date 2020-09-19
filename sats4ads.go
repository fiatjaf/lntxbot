package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/rs/zerolog"
)

const SATS4ADSUNACTIVITYDATEFORMAT = "20060102"

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

func getSats4AdsRate(user User) (rate int, err error) {
	err = pg.Get(&rate, `
SELECT (appdata->'sats4ads'->>'rate')::integer
FROM telegram.account
WHERE id = $1
    `, user.Id)
	return
}

func getSats4AdsRates() (rates []Sats4AdsRateGroup, err error) {
	err = pg.Select(&rates, `
WITH enabled_listeners AS (
  SELECT (appdata->'sats4ads'->>'rate')::integer AS rate
  FROM telegram.account
  WHERE appdata->'sats4ads'->'on' = 'true'::jsonb
), rategroups AS (
  SELECT generate_series ^ 3 AS uptorate FROM generate_series(1, 10)
)

SELECT uptorate, (SELECT count(*) FROM enabled_listeners WHERE rate <= uptorate) AS nusers
FROM rategroups
    `)
	return
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

		// identifier for the received payment
		// will be pending until the user clicks the "Viewed" button
		targethash := calculateHash(
			fmt.Sprintf("%d:%s:%d", contentMessage.MessageID, sourcehash, target.Id),
		)
		data := "x=s4a-v-" + targethash[:10]

		// build ad message based on the message that was replied to
		ad, nchars, thisCostMsat, thisCostSatoshis := buildSats4AdsMessage(
			logger,
			contentMessage, target, row.Rate,
			tgbotapi.InlineKeyboardMarkup{
				InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
					{
						tgbotapi.InlineKeyboardButton{
							Text:         "Viewed",
							CallbackData: &data,
						},
					},
				},
			},
		)

		if ad == nil {
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
			// logger.Info().Err(err).Msg("message wasn't sent. skipping.")
			err = nil
			continue
		}

		// commit payment (pending for receiver)
		errMsg, err = user.sendThroughProxy(
			sourcehash,
			targethash,
			contentMessage.MessageID,
			message.MessageID,
			target,
			thisCostMsat,
			fmt.Sprintf("ad dispatched to %d", messagesSent+1),
			fmt.Sprintf("%d characters ad (%s) at %d msat/char", nchars, sourcehash, row.Rate),
			true, // pending
			"sats4ads",
		)
		if err != nil {
			logger.Error().Err(err).Msg("error saving proxied payment. abort all.")
			return
		}

		// we will store this for 7 days so we can use this information on a task
		// if someone fail to see an ad for more than 3 days they will be excluded
		rds.SetNX(redisKeyUnviewedAd(
			target.Id),
			time.Now().Format(SATS4ADSUNACTIVITYDATEFORMAT),
			time.Hour*24*7,
		)

		messagesSent += 1
		costSatoshis += thisCostSatoshis
	}

	roundedCostSatoshis = int(costSatoshis)
	return
}

func buildSats4AdsMessage(
	logger zerolog.Logger,
	contentMessage *tgbotapi.Message,
	target User,
	rate int,
	keyboard interface{},
) (ad tgbotapi.Chattable, nchars int, thisCostMsat int, thisCostSatoshis float64) {
	thisCostMsat = 1000 // fixed 1sat fee for each message

	baseChat := tgbotapi.BaseChat{
		ChatID:      target.ChatId,
		ReplyMarkup: keyboard,
	}

	switch {
	case contentMessage.Text != "":
		nchars = len(contentMessage.Text)
		thisCostMsat += rate * nchars
		thisCostSatoshis = float64(thisCostMsat) / 1000
		footer := "\n\n" + translateTemplate(t.SATS4ADSADFOOTER, target.Locale, t.T{
			"Sats": thisCostSatoshis,
		})

		ad = tgbotapi.MessageConfig{
			BaseChat:              baseChat,
			Text:                  contentMessage.Text + footer,
			DisableWebPagePreview: false,
		}
	case contentMessage.Animation != nil:
		nchars = 300 + len(contentMessage.Caption)
		thisCostMsat += rate * nchars
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
		thisCostMsat += rate * nchars
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
		thisCostMsat += rate * nchars
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
		thisCostMsat += rate * nchars
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
		thisCostMsat += rate * nchars
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
	}

	return
}

func confirmAdViewed(user User, hashfirst10chars string) {
	_, err := pg.Exec(`
UPDATE lightning.transaction
SET pending = false
WHERE to_id = $1 AND payment_hash LIKE $2 || '%'
    `, user.Id, hashfirst10chars)
	if err != nil {
		log.Warn().Err(err).Str("hash", hashfirst10chars).Int("user", user.Id).
			Msg("failed to mark sats4ads tx as not pending")
	}

	// user viewed (any) ad, so prevent unsubscribing him
	rds.Del(redisKeyUnviewedAd(user.Id))
}

func cleanupUnviewedAds() {
	// for every person who has received an ad over 3 days ago and haven't seen it
	// we will cancel that payment (which is pending) and remove that person from
	// the sats4ads list
	txn, err := pg.Beginx()
	if err != nil {
		return
	}
	defer txn.Rollback()

	var deletedReceiverIds []int
	err = txn.Select(&deletedReceiverIds, `
WITH adsreceivedtxs AS (
  SELECT to_id, amount, payment_hash, proxied_with FROM lightning.transaction
  WHERE tag = 'sats4ads' AND time < (now() - interval '3 days') AND pending
), groupedbyproxy AS (
  SELECT proxied_with, sum(amount) AS amount FROM adsreceivedtxs
  GROUP BY proxied_with
), sourceupdates AS (
  UPDATE lightning.transaction AS s
  SET amount = s.amount - t.amount
  FROM groupedbyproxy AS t
  WHERE t.proxied_with = s.payment_hash
), deletes AS (
  DELETE FROM lightning.transaction
  WHERE payment_hash IN (SELECT payment_hash FROM adsreceivedtxs)
)
SELECT DISTINCT to_id FROM adsreceivedtxs
    `)
	if err != nil {
		log.Warn().Err(err).Msg("failed to delete sats4ads pending tx")
		return
	}

	// check proxy balance (should be always zero)
	if err := checkProxyBalance(txn); err != nil {
		log.Error().Err(err).Msg("proxy balance check on cleanupUnviewedAds")
		return
	}

	err = txn.Commit()
	if err != nil {
		return
	}

	// for each deleted we check redis for sats4ads viewer inactivity and unsubscribe
	threedaysago := time.Now().AddDate(0, 0, -3)
	for _, receiverId := range deletedReceiverIds {
		key := redisKeyUnviewedAd(receiverId)
		if val, err := rds.Get(key).Result(); err == nil {
			if rec, err := time.Parse(SATS4ADSUNACTIVITYDATEFORMAT, val); err == nil {
				if rec.Before(threedaysago) {
					if receiver, err := loadUser(receiverId, 0); err == nil {
						err = turnSats4AdsOff(receiver)
						if err != nil {
							log.Warn().Int("user", receiverId).
								Msg("failed to turn off sats4ads for inactive user")
							continue
						}

						receiver.notify(t.SATS4ADSTOGGLE, t.T{"On": false})
						rds.Del(key)
					}
				}
			}
		}
	}
}

func sats4adsCleanupRoutine() {
	for {
		cleanupUnviewedAds()
		time.Sleep(time.Hour * 6)
	}
}

func redisKeyUnviewedAd(userId int) string {
	return fmt.Sprintf("sats4ads:unviewed:%d", userId)
}
