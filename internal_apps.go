package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"git.alhur.es/fiatjaf/lntxbot/t"
	"github.com/go-telegram-bot-api/telegram-bot-api"
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
		hiddenmessage.Preview = translateTemplate(t.HIDDENDEFAULTPREVIEW, locale, t.T{"Sats": hiddenmessage.Satoshis})
	}

	return
}

func revealKeyboard(fullRedisKey string, hiddenmessage HiddenMessage, havepaid int, locale string) *tgbotapi.InlineKeyboardMarkup {
	return &tgbotapi.InlineKeyboardMarkup{
		[][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData(
					fmt.Sprintf(translateTemplate(t.HIDDENREVEALBUTTON, locale, t.T{
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
	txn, err := pg.BeginTxx(context.TODO(),
		&sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return
	}
	defer txn.Rollback()

	receiver, _ = loadUser(toId, 0)
	giverNames := make([]string, 0, len(fromIds))

	msats := sats * 1000

	for _, fromId := range fromIds {
		if fromId == toId {
			continue
		}

		_, err = txn.Exec(`
INSERT INTO lightning.transaction
  (from_id, to_id, amount, description)
VALUES ($1, $2, $3, 'reveal')
    `, fromId, toId, msats)
		if err != nil {
			return
		}

		var balance int
		err = txn.Get(&balance, `
SELECT balance::numeric(13) FROM lightning.balance WHERE account_id = $1
    `, fromId)
		if err != nil {
			return
		}

		if balance < 0 {
			err = errors.New("insufficient balance")
			return
		}

		giver, _ := loadUser(fromId, 0)
		giverNames = append(giverNames, giver.AtName())

		giver.notify(t.HIDDENREVEALMSG, t.T{
			"Sats": sats,
			"Id":   hiddenid,
		})
	}

	err = txn.Commit()
	if err != nil {
		return
	}

	receiver.notify(t.HIDDENSOURCEMSG, t.T{
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
					translate(t.CANCEL, locale),
					fmt.Sprintf("cancel=%d", giverId),
				),
				tgbotapi.NewInlineKeyboardButtonData(
					translate(t.GIVEAWAYCLAIM, locale),
					buttonData,
				),
			},
		},
	}
}

// giveflip
func giveflipKeyboard(giveflipid string, giverId, nparticipants, sats int, locale string) *tgbotapi.InlineKeyboardMarkup {
	return &tgbotapi.InlineKeyboardMarkup{
		[][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData(
					translate(t.CANCEL, locale),
					fmt.Sprintf("cancel=%d", giverId),
				),
				tgbotapi.NewInlineKeyboardButtonData(
					translate(t.GIVEFLIPJOIN, locale),
					fmt.Sprintf("gifl=%d-%d-%d-%s", giverId, nparticipants, sats, giveflipid),
				),
			},
		},
	}
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
					translate(t.COINFLIPJOIN, locale),
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
  AND description = 'coinflip'
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
	txn, err := pg.BeginTxx(context.TODO(),
		&sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return
	}
	defer txn.Rollback()

	receiver, _ = loadUser(toId, 0)
	giverNames := make([]string, 0, len(fromIds))

	msats := sats * 1000

	// receiver must also have the necessary sats in his balance at the time
	var receiverBalance int
	err = txn.Get(&receiverBalance, `
SELECT balance::numeric(13) FROM lightning.balance WHERE account_id = $1
    `, toId)
	if err != nil {
		return
	}
	if receiverBalance < msats {
		err = errors.New("Receiver has insufficient balance.")
		return
	}

	// then we create a transfer from each of the other participants
	for _, fromId := range fromIds {
		if fromId == toId {
			continue
		}

		_, err = txn.Exec(`
INSERT INTO lightning.transaction
  (from_id, to_id, amount, description)
VALUES ($1, $2, $3, 'coinflip')
    `, fromId, toId, msats)
		if err != nil {
			return
		}

		var balance int
		err = txn.Get(&balance, `
SELECT balance::numeric(13) FROM lightning.balance WHERE account_id = $1
    `, fromId)
		if err != nil {
			return
		}

		if balance < 0 {
			err = errors.New("insufficient balance")
			return
		}

		giver, _ := loadUser(fromId, 0)
		giverNames = append(giverNames, giver.AtName())

		giver.notify(t.COINFLIPGIVERMSG, t.T{
			"IndividualSats": sats,
			"Receiver":       receiver.AtName(),
		})
	}

	err = txn.Commit()
	if err != nil {
		return
	}

	receiver.notify(t.COINFLIPWINNERMSG, t.T{
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
					translate(t.FUNDRAISEJOIN, locale),
					fmt.Sprintf("raise=%d-%d-%d-%s", receiverId, nparticipants, sats, fundraiseid),
				),
			},
		},
	}
}

func settleFundraise(sats int, toId int, fromIds []int) (receiver User, err error) {
	txn, err := pg.BeginTxx(context.TODO(),
		&sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return
	}
	defer txn.Rollback()

	receiver, _ = loadUser(toId, 0)
	giverNames := make([]string, 0, len(fromIds))

	msats := sats * 1000

	for _, fromId := range fromIds {
		if fromId == toId {
			continue
		}

		_, err = txn.Exec(`
INSERT INTO lightning.transaction
  (from_id, to_id, amount, description)
VALUES ($1, $2, $3, 'fundraise')
    `, fromId, toId, msats)
		if err != nil {
			return
		}

		var balance int
		err = txn.Get(&balance, `
SELECT balance::numeric(13) FROM lightning.balance WHERE account_id = $1
    `, fromId)
		if err != nil {
			return
		}

		if balance < 0 {
			err = errors.New("insufficient balance")
			return
		}

		giver, _ := loadUser(fromId, 0)
		giverNames = append(giverNames, giver.AtName())

		giver.notify(t.FUNDRAISEGIVERMSG, t.T{
			"IndividualSats": sats,
			"Receiver":       receiver.AtName(),
		})
	}

	err = txn.Commit()
	if err != nil {
		return
	}

	receiver.notify(t.FUNDRAISERECEIVERMSG, t.T{
		"TotalSats": sats * len(fromIds),
		"Senders":   strings.Join(giverNames, " "),
	})
	return
}
