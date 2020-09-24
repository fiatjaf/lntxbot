package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/lucsky/cuid"
)

// hide and reveal
type HiddenMessage struct {
	Preview   string `json:"preview"`
	Content   string `json:"content"`
	Times     int    `json:"times"`
	Crowdfund int    `json:"crowdfund"`
	Public    bool   `json:"public"`
	Satoshis  int    `json:"satoshis"`
}

func (h HiddenMessage) revealed() string {
	return strings.TrimSpace(h.Preview) + "\n~\n" + strings.TrimSpace(h.Content)
}

func getHiddenId(message *tgbotapi.Message) string {
	return calculateHash(fmt.Sprintf("%d%d", message.MessageID, message.Chat.ID))[:7]
}

func findHiddenKey(hiddenid string) (key string, ok bool) {
	found := rds.Keys("hidden:*:" + hiddenid).Val()
	if len(found) == 0 {
		return "", false
	}

	return found[0], true
}

func getHiddenMessage(redisKey, locale string) (sourceuser int, id string, hiddenmessage HiddenMessage, err error) {
	data, err := rds.Get(redisKey).Bytes()
	if err != nil {
		return
	}

	err = json.Unmarshal(data, &hiddenmessage)
	if err != nil {
		return
	}

	keyparts := strings.Split(redisKey, ":")
	id = keyparts[2]
	sourceuser, err = strconv.Atoi(keyparts[1])
	if err != nil {
		return
	}

	if hiddenmessage.Preview == "" {
		hiddenmessage.Preview = translateTemplate(ctx, t.HIDDENDEFAULTPREVIEW, t.T{"Sats": hiddenmessage.Satoshis})
	}

	return
}

func revealKeyboard(fullRedisKey string, hiddenmessage HiddenMessage, havepaid int, locale string) *tgbotapi.InlineKeyboardMarkup {
	return &tgbotapi.InlineKeyboardMarkup{
		[][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData(
					fmt.Sprintf(translateTemplate(ctx, t.HIDDENREVEALBUTTON, t.T{
						"Sats":      hiddenmessage.Satoshis,
						"Public":    hiddenmessage.Public,
						"Crowdfund": hiddenmessage.Crowdfund,
						"Times":     hiddenmessage.Times,
						"HavePaid":  havepaid,
					})),
					fmt.Sprintf("reveal=%s", fullRedisKey),
				),
			},
		},
	}
}

func settleReveal(sats int, hiddenid string, toId int, fromIds []int) (receiver User, err error) {
	txn, err := pg.Beginx()
	if err != nil {
		return
	}
	defer txn.Rollback()

	receiver, _ = loadUser(toId)
	giverNames := make([]string, 0, len(fromIds))

	msats := sats * 1000

	random, err := randomPreimage()
	if err != nil {
		return
	}
	receiverHash := calculateHash(random) // for the proxied transaction
	for _, fromId := range fromIds {
		if fromId == toId {
			continue
		}

		// A->proxy->B (for many A, one B)
		_, err = txn.Exec(`
INSERT INTO lightning.transaction (from_id, to_id, amount, tag)
VALUES ($1, $2, $3, 'reveal')
    `, fromId, s.ProxyAccount, msats)
		if err != nil {
			return
		}
		_, err = txn.Exec(`
INSERT INTO lightning.transaction AS t (payment_hash, from_id, to_id, amount, tag)
VALUES ($1, $2, $3, $4, 'reveal')
ON CONFLICT (payment_hash) DO UPDATE SET amount = t.amount + $4
    `, receiverHash, s.ProxyAccount, toId, msats)
		if err != nil {
			return
		}

		// check sender balance
		balance := getBalance(txn, fromId)
		if balance < 0 {
			err = errors.New("insufficient balance")
			return
		}

		// check proxy balance (should be always zero)
		proxybalance := getBalance(txn, s.ProxyAccount)
		if proxybalance != 0 {
			log.Error().Err(err).Int64("balance", proxybalance).
				Msg("proxy balance isn't 0")
			err = errors.New("proxy balance isn't 0")
			return
		}

		giver, _ := loadUser(fromId)
		giverNames = append(giverNames, giver.AtName())

		send(ctx, giver, t.HIDDENREVEALMSG, t.T{
			"Sats": sats,
			"Id":   hiddenid,
		})
	}

	err = txn.Commit()
	if err != nil {
		return
	}

	send(ctx, receiver, t.HIDDENSOURCEMSG, t.T{
		"Sats":      sats * len(fromIds),
		"Revealers": strings.Join(giverNames, " "),
		"Id":        hiddenid,
	})
	return
}

// giveaway
func giveawayKeyboard(giverId, sats int, locale string) *tgbotapi.InlineKeyboardMarkup {
	giveawayid := cuid.Slug()
	buttonData := fmt.Sprintf("give=%d-%d-%s", giverId, sats, giveawayid)

	rds.Set("giveaway:"+giveawayid, buttonData, s.GiveAwayTimeout)

	return &tgbotapi.InlineKeyboardMarkup{
		[][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData(
					translate(ctx, t.CANCEL),
					fmt.Sprintf("cancel=%d", giverId),
				),
				tgbotapi.NewInlineKeyboardButtonData(
					translate(ctx, t.GIVEAWAYCLAIM),
					buttonData,
				),
			},
		},
	}
}

func canJoinGiveaway(joinerId int) bool {
	var ngiveawaysjoined int
	err := pg.Get(&ngiveawaysjoined, `
SELECT count(*)
FROM lightning.account_txn
WHERE account_id = $1
  AND tag = 'giveaway'
  AND time > 'now'::timestamp - make_interval(days := $2)
    `, joinerId, s.GiveawayAvgDays)

	if err != nil {
		log.Warn().Err(err).Int("joiner", joinerId).Msg("failed to check ngiveaways in last 24h")
		return false
	}

	// since we are not taking into account all giveaways that may be opened right now
	// we'll consider a big time period so the user participation is averaged over time
	// for example, if he joins 15 giveaways today but the quota is 5 it will be ok
	// but then he will be unable to join any for the next 3 day.
	periodQuota := s.GiveawayDailyQuota * s.GiveawayAvgDays

	return ngiveawaysjoined < periodQuota
}

// giveflip
func giveflipKeyboard(giveflipid string, giverId, nparticipants, sats int, locale string) *tgbotapi.InlineKeyboardMarkup {
	return &tgbotapi.InlineKeyboardMarkup{
		[][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData(
					translate(ctx, t.CANCEL),
					fmt.Sprintf("cancel=%d", giverId),
				),
				tgbotapi.NewInlineKeyboardButtonData(
					translate(ctx, t.GIVEFLIPJOIN),
					fmt.Sprintf("gifl=%d-%d-%d-%s", giverId, nparticipants, sats, giveflipid),
				),
			},
		},
	}
}

func canCreateGiveflip(initiatorId int) bool {
	didagivefliprecently, err := rds.Exists(fmt.Sprintf("recentgiveflip:%d", initiatorId)).Result()
	if err != nil {
		log.Warn().Err(err).Int("initiator", initiatorId).Msg("failed to check recentgiveflip:")
		return false
	}
	if didagivefliprecently {
		return false
	}

	return true
}

func canJoinGiveflip(joinerId int) bool {
	var ngiveflipsjoined int
	err := pg.Get(&ngiveflipsjoined, `
SELECT count(*)
FROM lightning.account_txn
WHERE account_id = $1
  AND tag = 'giveflip'
  AND time > 'now'::timestamp - make_interval(days := $2)
    `, joinerId, s.GiveflipAvgDays)

	if err != nil {
		log.Warn().Err(err).Int("joiner", joinerId).Msg("failed to check ngiveflips in last 24h")
		return false
	}

	// since we are not taking into account all giveflips that may be opened right now
	// we'll consider a big time period so the user participation is averaged over time
	// for example, if he joins 15 giveflips today but the quota is 5 it will be ok
	// but then he will be unable to join any for the next 3 day.
	periodQuota := s.GiveflipDailyQuota * s.GiveflipAvgDays

	return ngiveflipsjoined < periodQuota
}

// coinflip
func coinflipKeyboard(
	coinflipid string,
	initiatorId int,
	nparticipants,
	sats int,
	locale string,
) *tgbotapi.InlineKeyboardMarkup {
	if coinflipid == "" {
		coinflipid = cuid.Slug()
	}

	if initiatorId != 0 {
		rds.SAdd("coinflip:"+coinflipid, initiatorId)
	}

	rds.Expire("coinflip:"+coinflipid, s.GiveAwayTimeout)

	return &tgbotapi.InlineKeyboardMarkup{
		[][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData(
					translate(ctx, t.COINFLIPJOIN),
					fmt.Sprintf("flip=%d-%d-%s", nparticipants, sats, coinflipid),
				),
			},
		},
	}
}

func canCreateCoinflip(initiatorId int) bool {
	didacoinfliprecently, err := rds.Exists(fmt.Sprintf("recentcoinflip:%d", initiatorId)).Result()
	if err != nil {
		log.Warn().Err(err).Int("initiator", initiatorId).Msg("failed to check recentcoinflip:")
		return false
	}
	if didacoinfliprecently {
		return false
	}

	return true
}

func canJoinCoinflip(joinerId int) bool {
	var ncoinflipsjoined int
	err := pg.Get(&ncoinflipsjoined, `
SELECT count(*)
FROM lightning.account_txn
WHERE account_id = $1
  AND tag = 'coinflip'
  AND time > 'now'::timestamp - make_interval(days := $2)
    `, joinerId, s.CoinflipAvgDays)

	if err != nil {
		log.Warn().Err(err).Int("joiner", joinerId).Msg("failed to check ncoinflips in last 24h")
		return false
	}

	// since we are not taking into account all coinflips that may be opened right now
	// we'll consider a big time period so the user participation is averaged over time
	// for example, if he joins 15 coinflips today but the quota is 5 it will be ok
	// but then he will be unable to join any for the next 3 day.
	periodQuota := s.CoinflipDailyQuota * s.CoinflipAvgDays

	return ncoinflipsjoined < periodQuota
}

func settleCoinflip(sats int, toId int, fromIds []int) (receiver User, err error) {
	txn, err := pg.Beginx()
	if err != nil {
		return
	}
	defer txn.Rollback()

	receiver, _ = loadUser(toId)
	giverNames := make([]string, 0, len(fromIds))

	msats := int64(sats) * 1000

	// receiver must also have the necessary sats in his balance at the time
	receiverBalance := getBalance(txn, toId)
	if receiverBalance < msats {
		err = errors.New("Receiver has insufficient balance.")
		return
	}

	random, err := randomPreimage()
	if err != nil {
		return
	}
	receiverHash := calculateHash(random) // for the proxied transaction

	// then we create a transfer from each of the other participants
	for _, fromId := range fromIds {
		if fromId == toId {
			continue
		}

		// A->proxy->B (for many A, one B)
		_, err = txn.Exec(`
INSERT INTO lightning.transaction (from_id, to_id, amount, tag)
VALUES ($1, $2, $3, 'coinflip')
    `, fromId, s.ProxyAccount, msats)
		if err != nil {
			return
		}

		_, err = txn.Exec(`
INSERT INTO lightning.transaction AS t (payment_hash, from_id, to_id, amount, tag)
VALUES ($1, $2, $3, $4, 'coinflip')
ON CONFLICT (payment_hash) DO UPDATE SET amount = t.amount + $4
    `, receiverHash, s.ProxyAccount, toId, msats)
		if err != nil {
			return
		}

		// check sender balance
		balance := getBalance(txn, fromId)
		if balance < 0 {
			err = errors.New("insufficient balance")
			return
		}

		// check proxy balance (should be always zero)
		proxybalance := getBalance(txn, s.ProxyAccount)
		if proxybalance != 0 {
			log.Error().Err(err).Int64("balance", proxybalance).
				Msg("proxy balance isn't 0")
			err = errors.New("proxy balance isn't 0")
			return
		}

		giver, _ := loadUser(fromId)
		giverNames = append(giverNames, giver.AtName())

		send(ctx, giver, t.COINFLIPGIVERMSG, t.T{
			"IndividualSats": sats,
			"Receiver":       receiver.AtName(),
		})
	}

	err = txn.Commit()
	if err != nil {
		return
	}

	send(ctx, receiver, t.COINFLIPWINNERMSG, t.T{
		"TotalSats": sats * len(fromIds),
		"Senders":   strings.Join(giverNames, " "),
	})

	return
}

// fundraise
func fundraiseKeyboard(
	fundraiseid string,
	initiatorId int,
	receiverId int,
	nparticipants int,
	sats int,
	locale string,
) *tgbotapi.InlineKeyboardMarkup {
	if fundraiseid == "" {
		fundraiseid = cuid.Slug()
	}

	if initiatorId != 0 {
		rds.SAdd("fundraise:"+fundraiseid, initiatorId)
	}

	rds.Expire("fundraise:"+fundraiseid, s.GiveAwayTimeout)

	return &tgbotapi.InlineKeyboardMarkup{
		[][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData(
					translate(ctx, t.FUNDRAISEJOIN),
					fmt.Sprintf("raise=%d-%d-%d-%s", receiverId, nparticipants, sats, fundraiseid),
				),
			},
		},
	}
}

func settleFundraise(sats int, toId int, fromIds []int) (receiver User, err error) {
	txn, err := pg.Beginx()
	if err != nil {
		return
	}
	defer txn.Rollback()

	receiver, _ = loadUser(toId)
	giverNames := make([]string, 0, len(fromIds))

	msats := sats * 1000

	random, err := randomPreimage()
	if err != nil {
		return
	}
	receiverHash := calculateHash(random) // for the proxied transaction

	for _, fromId := range fromIds {
		if fromId == toId {
			continue
		}

		// A->proxy->B (for many A, one B)
		_, err = txn.Exec(`
INSERT INTO lightning.transaction (from_id, to_id, amount, tag)
VALUES ($1, $2, $3, 'fundraise')
    `, fromId, s.ProxyAccount, msats)
		if err != nil {
			return
		}

		_, err = txn.Exec(`
INSERT INTO lightning.transaction AS t (payment_hash, from_id, to_id, amount, tag)
VALUES ($1, $2, $3, $4, 'fundraise')
ON CONFLICT (payment_hash) DO UPDATE SET amount = t.amount + $4
    `, receiverHash, s.ProxyAccount, toId, msats)
		if err != nil {
			return
		}

		balance := getBalance(txn, fromId)
		if balance < 0 {
			err = errors.New("insufficient balance")
			return
		}

		// check proxy balance (should be always zero)
		proxybalance := getBalance(txn, s.ProxyAccount)
		if proxybalance != 0 {
			log.Error().Err(err).Int64("balance", proxybalance).
				Msg("proxy balance isn't 0")
			err = errors.New("proxy balance isn't 0")
			return
		}

		giver, _ := loadUser(fromId)
		giverNames = append(giverNames, giver.AtName())

		send(ctx, giver, t.FUNDRAISEGIVERMSG, t.T{
			"IndividualSats": sats,
			"Receiver":       receiver.AtName(),
		})
	}

	err = txn.Commit()
	if err != nil {
		return
	}

	send(ctx, receiver, t.FUNDRAISERECEIVERMSG, t.T{
		"TotalSats": sats * len(fromIds),
		"Senders":   strings.Join(giverNames, " "),
	})
	return
}

// rename groups
func renameKeyboard(
	renamerId int,
	chatId int64,
	sats int,
	name string,
	locale string,
) *tgbotapi.InlineKeyboardMarkup {
	hash := sha256.Sum256([]byte(name))
	renameId := hex.EncodeToString(hash[:])[:12]

	rds.Set(
		fmt.Sprintf("rename:%s", renameId),
		fmt.Sprintf("%d|~|%d|~|%s", chatId, sats, name),
		time.Minute*60,
	)

	return &tgbotapi.InlineKeyboardMarkup{
		[][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData(
					translate(ctx, t.CANCEL),
					fmt.Sprintf("cancel=%d", renamerId),
				),
				tgbotapi.NewInlineKeyboardButtonData(
					translate(ctx, t.YES),
					fmt.Sprintf("rnm=%s", renameId),
				),
			},
		},
	}
}
