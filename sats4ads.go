package main

import "git.alhur.es/fiatjaf/lntxbot/t"

type Sats4AdsData struct {
	On    bool `json:"on"`
	Price int  `json:"price"` // in msatoshi per character
}

type Sats4AdsPriceGroup struct {
	Price  int `db:"price"` // in msatoshi per character
	NUsers int `db:"nusers"`
}

func turnSats4AdsOn(user User, price int) error {
	var data Sats4AdsData
	err := user.getAppData("sats4ads", &data)
	if err != nil {
		return err
	}

	data.On = true
	data.Price = price
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

func broadcastSats4Ads(user User, budgetSatoshis int, content string) (messagesSent int, costSatoshis int, err error) {
	nchars := len(content)

	rows, err := pg.Queryx(`
SELECT id, (appdata->'sats4ads'->>'price')::int AS price
FROM telegram.account
WHERE appdata->'sats4ads'->'on' = 'true'::jsonb
  AND id != $1
ORDER BY appdata->'sats4ads'->>'price' ASC, random()
    `, user.Id)
	if err != nil {
		return
	}

	// send messages and pay receivers one by one
	for rows.Next() {
		var row struct {
			Id    int `db:"id"`
			Price int `db:"price"`
		}

		err = rows.StructScan(&row)
		if err != nil {
			return
		}

		thisCostMsat := row.Price * nchars
		thisCostSatoshis := float64(thisCostMsat) / 1000

		if costSatoshis+int(thisCostSatoshis) > budgetSatoshis {
			// budget ended
			return
		}

		// ok, we still have money to spend.
		target, err := loadUser(row.Id, 0)
		if err != nil {
			continue
		}

		message := sendMessage(target.ChatId, content+"\n\n"+translateTemplate(t.SATS4ADSADFOOTER, target.Locale, t.T{
			"Sats": thisCostSatoshis,
		}))
		if message.MessageID == 0 {
			// message wasn't sent
			continue
		}

		_, err = user.sendInternally(message.MessageID, target, false, thisCostMsat, "", "sats4ads")
		if err != nil {
			log.Warn().Err(err).Str("user", user.Username).Str("target", target.Username).Int("amount", thisCostMsat).
				Msg("failed to pay sats4ads")
			continue
		}

		messagesSent += 1
		costSatoshis += int(thisCostSatoshis)
	}

	err = nil
	return
}

func getSats4AdsPrices(user User) (prices []Sats4AdsPriceGroup, err error) {
	err = pg.Select(&prices, `
SELECT * FROM (
  SELECT
    (appdata->'sats4ads'->>'price')::integer AS price,
    count(*) AS nusers
  FROM telegram.account
  WHERE appdata->'sats4ads'->'on' = 'true'::jsonb
    AND id != $1
  GROUP BY (appdata->'sats4ads'->>'price')::integer
)x ORDER BY nusers
    `, user.Id)
	return
}
