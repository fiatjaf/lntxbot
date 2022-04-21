package main

func RunAggregateOldTransactions() {
	//	twoMonthsAgo := time.Now().AddDate(0, -2, 0)
	//	yearBegin := time.Date(twoMonthsAgo.Year(), 1, 0, 0, 0, 0, 0, time.FixedZone("UTC", 0))
	//
	//	var users []int
	//	if err := pg.Select(&users, "SELECT id FROM accounts"); err != nil {
	//		log.Fatal().Err(err).Msg("failed to get users")
	//	}
	//
	//	txn, err := pg.BeginTxx(ctx, &sql.TxOptions{ Isolation: sql.LevelSerializable })()
	//	if err != nil {
	//		log.Fatal().Err(err).Msg("failed to start transaction")
	//	}
	//
	//	for _, user := range users {
	//		log.Printf("user %d", user)
	//
	//		var agg struct {
	//			Balance int64 `db:"amount"`
	//			Fees    int64 `db:"fees"`
	//		}
	//		if err := txn.Get(&agg, `
	//        SELECT
	//          coalesce(sum(amount), 0) AS amount,
	//          coalesce(sum(fees), 0) AS fees
	//        FROM lightning.account_txn
	//        WHERE account_id = $1
	//          AND pending = false
	//        `, user); err != nil {
	//			log.Fatal().Err(err).Msg("failed to query user")
	//		}
	//
	//	}
}
