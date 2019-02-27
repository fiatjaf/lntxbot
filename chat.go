package main

/*
   ALL GROUP CHAT TELEGRAM IDS ARE NEGATIVE
*/

type GroupChat struct {
	TelegramId int  `db:"telegram_id"`
	Spammy     bool `db:"spammy"`
}

var spammy_cache = map[int64]bool{}

func toggleSpammy(telegramId int64) (spammy bool, err error) {
	err = pg.Get(&spammy, `
      INSERT INTO telegram.chat AS c (telegram_id, spammy) VALUES ($1, true)
      ON CONFLICT (telegram_id)
        DO UPDATE SET spammy = NOT c.spammy
        RETURNING spammy
    `, -telegramId)

	spammy_cache[-telegramId] = spammy

	return
}

func isSpammy(telegramId int64) (spammy bool) {
	if spammy, ok := spammy_cache[-telegramId]; ok {
		return spammy
	}

	err := pg.Get(&spammy, `
      SELECT spammy FROM telegram.chat WHERE telegram_id = $1
    `, -telegramId)
	if err != nil {
		return false
	}

	spammy_cache[-telegramId] = spammy

	return
}
