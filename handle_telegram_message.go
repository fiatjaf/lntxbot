package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/lucsky/cuid"
)

func handleTelegramMessage(ctx context.Context, message *tgbotapi.Message) {
	ctx = context.WithValue(ctx, "message", message)

	u, tcase, err := ensureTelegramUser(message)
	if err != nil {
		log.Warn().Err(err).Int("case", tcase).
			Str("username", message.From.UserName).
			Int("id", message.From.ID).
			Msg("failed to ensure telegram user")
		return
	}

	// stop if temporarily banned
	if _, ok := s.Banned[u.Id]; ok {
		log.Debug().Stringer("id", &u).Msg("got request from banned user")
		return
	}

	ctx = context.WithValue(ctx, "initiator", u)

	// by default we use the user locale for the group object, because
	// we may end up sending the message to the user instead of to the group
	// (if, for example, the user calls /coinflip on his own chat) then
	// we at least want the correct language used there.
	g := GroupChat{TelegramId: message.Chat.ID, Locale: u.Locale}

	// this is just to send to amplitude
	var groupId *int64 = nil

	if message.Chat.Type == "private" {
		// after ensuring the user we should always enable him to
		// receive payment notifications and so on, as not all people will
		// remember to call /start
		u.setChat(message.Chat.ID)
	} else if isChannelOrGroupUser(message.From) {
		// if the user is not a real user, but instead a channel/group entity
		// make their chat be this one even though it's public
		u.setChat(message.Chat.ID)
	}

	if message.Chat.Type != "private" {
		// when we're in a group, load the group
		loadedGroup, err := loadTelegramGroup(message.Chat.ID)
		if err != nil {
			if err != sql.ErrNoRows {
				log.Warn().Err(err).Int64("id", message.Chat.ID).Msg("failed to load group")
			}
			// proceed with an empty group (manually defined before)
		} else {
			// we manage to load a group, use it then
			g = loadedGroup
		}

		groupId = &message.Chat.ID

		if message.Entities == nil || len(*message.Entities) == 0 ||
			// unless in the private chat, only messages starting with
			// bot commands will work
			(*message.Entities)[0].Type != "bot_command" ||
			(*message.Entities)[0].Offset != 0 {
			return
		}
	}

	// may be the user chat fake-group
	ctx = context.WithValue(ctx, "group", g)

	var (
		opts        = make(docopt.Opts)
		isCommand   = false
		messageText = strings.ReplaceAll(
			regexp.MustCompile("/([\\w_]+)@"+s.ServiceId).ReplaceAllString(message.Text, "/$1"),
			"—", "--",
		)
	)

	// when receiving a forwarded invoice (from messages from other people?)
	// or just the full text of a an invoice (shared from a phone wallet?)
	if !strings.HasPrefix(messageText, "/") {
		if bolt11, lnurltext, address, ok := searchForInvoice(ctx); ok {
			if bolt11 != "" {
				opts, _, err = parse("/pay " + bolt11)
				if err != nil {
					return
				}
				goto parsed
			}
			if lnurltext != "" {
				opts, _, err = parse("/lnurl " + lnurltext)
				if err != nil {
					return
				}
				goto parsed
			}

			if address != "" {
				opts, _, err = parse("/lnurl " + address)
				if err != nil {
					return
				}
				goto parsed
			}
		}
	}

	// manage the underlying node
	if message.Chat.Type == "private" &&
		s.AdminAccount > 0 &&
		u.Id == s.AdminAccount &&
		strings.HasPrefix(messageText, "/cliche ") {

		handleClicheCommand(ctx, message, messageText)
		return
	}

	// otherwise parse the slash command
	opts, isCommand, err = parse(messageText)
	log.Debug().Str("t", messageText).Stringer("user", &u).Err(err).
		Msg("telegram message")
	if !isCommand {
		if message.ReplyToMessage != nil &&
			message.ReplyToMessage.From.ID == bot.Self.ID {
			// may be a written reply to a specific bot prompt
			handleReply(ctx)
		}

		return
	}
	if err != nil {
		if message.Chat.Type == "private" {
			// only tell we don't understand commands when in a private chat
			// because these commands we're not understanding
			// may be targeting other bots in a group, so we're spamming people.
			method := strings.Split(messageText, " ")[0][1:]
			handled := handleHelp(ctx, method)
			if !handled {
				send(ctx, u, t.WRONGCOMMAND)
			}
		}
		return
	}

	go u.track("command", map[string]interface{}{
		"command": strings.Split(strings.Split(message.Text, " ")[0], "_")[0],
		"group":   groupId,
	})

parsed:
	if opts["paynow"].(bool) {
		opts["pay"] = true
		opts["now"] = true
	}

	switch {
	case opts["start"].(bool):
		handleStart(ctx)
		go u.track("start", nil)
		break
	case opts["stop"].(bool):
		if message.Chat.Type == "private" {
			u.unsetChat()
			send(ctx, u, t.STOPNOTIFY)
			go u.track("stop", nil)
		}
		break
	case opts["bluewallet"].(bool), opts["zeus"].(bool), opts["lndhub"].(bool):
		go handleBlueWallet(ctx, opts)
	case opts["api"].(bool):
		go handleAPI(ctx, opts)
	case opts["lightningatm"].(bool):
		go handleLightningATM(ctx)
	case opts["tx"].(bool):
		go handleSingleTransaction(ctx, opts)
	case opts["send"].(bool), opts["tip"].(bool), opts["honk"].(bool):
		go u.track("send", map[string]interface{}{
			"group":     groupId,
			"reply-tip": message.ReplyToMessage != nil,
		})
		handleSend(ctx, opts)
	case opts["giveaway"].(bool):
		msats, err := parseSatoshis(opts)
		if err != nil {
			send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
			break
		}
		if !u.checkBalanceFor(ctx, msats, "giveaway") {
			break
		}

		sats := int(msats / 1000)

		send(ctx, g, FORCESPAMMY, t.GIVEAWAYMSG, t.T{
			"User": u.AtName(ctx),
			"Sats": sats,
		}, giveawayKeyboard(ctx, u.Id, sats, ""))

		go u.track("giveaway created", map[string]interface{}{
			"group": groupId,
			"sats":  sats,
		})
		break
	case opts["giveflip"].(bool):
		msats, err := parseSatoshis(opts)
		if err != nil {
			send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
			break
		}
		if !u.checkBalanceFor(ctx, msats, "giveflip") {
			break
		}

		sats := int(msats / 1000)
		var nparticipants int
		if n, err := opts.Int("<num_participants>"); err == nil {
			if n < 2 || n > 100 {
				send(ctx, u, t.INVALIDPARTNUMBER, t.T{"Number": strconv.Itoa(n)})
				break
			} else {
				nparticipants = n
			}
		} else {
			nparticipants = 2
		}

		giveflipid := cuid.Slug()
		send(ctx, g, FORCESPAMMY,
			t.GIVEFLIPMSG, t.T{
				"User":         u.AtName(ctx),
				"Sats":         sats,
				"Participants": nparticipants,
			}, giveflipKeyboard(ctx, giveflipid, u.Id, nparticipants, sats))

		go u.track("giveflip created", map[string]interface{}{
			"group": groupId,
			"sats":  sats,
			"n":     nparticipants,
		})
		break
	case opts["coinflip"].(bool), opts["lottery"].(bool):
		enabled := g.areCoinflipsEnabled()
		if !enabled {
			forwardMessage(message, u.TelegramChatId)
			deleteMessage(message)
			send(ctx, u, t.COINFLIPSENABLEDMSG, t.T{"Enabled": false})
			break
		}

		// open a lottery between a number of users in a group
		msats, err := parseSatoshis(opts)
		if err != nil {
			send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
			break
		}
		if !u.checkBalanceFor(ctx, msats+COINFLIP_TAX, "coinflip") {
			break
		}

		sats := int(msats / 1000)
		nparticipants := 2
		if n, err := opts.Int("<num_participants>"); err == nil {
			if n < 2 || n > 100 {
				send(ctx, u, t.INVALIDPARTNUMBER, t.T{"Number": strconv.Itoa(n)})
				break
			} else {
				nparticipants = n
			}
		}

		send(ctx, g, t.LOTTERYMSG, FORCESPAMMY, t.T{
			"EntrySats":    sats,
			"Participants": nparticipants,
			"Prize":        sats * nparticipants,
			"Registered":   u.AtName(ctx),
		}, coinflipKeyboard(ctx, "", u.Id, nparticipants, sats))

		// save this to limit coinflip creation per user
		go u.track("coinflip created", map[string]interface{}{
			"group": groupId,
			"sats":  sats,
			"n":     nparticipants,
		})
		rds.Set(fmt.Sprintf("recentcoinflip:%d", u.Id), "t", time.Minute*30)
	case opts["fundraise"].(bool), opts["crowdfund"].(bool):
		// many people join, we get all the money and transfer to the target
		msats, err := parseSatoshis(opts)
		if err != nil {
			send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
			break
		}
		if !u.checkBalanceFor(ctx, msats, "fundraise") {
			break
		}
		sats := int(msats / 1000)

		nparticipants, err := opts.Int("<num_participants>")
		if err != nil || nparticipants < 2 || nparticipants > 100 {
			send(ctx, u, t.INVALIDPARTNUMBER, t.T{"Number": nparticipants})
			break
		}

		receiver, err := examineTelegramUsername(opts["<receiver>"].(string))
		if err != nil {
			log.Warn().Err(err).Msg("parsing fundraise receiver")
			send(ctx, u, t.FAILEDUSER)
			break
		}

		send(ctx, g, t.FUNDRAISEAD, FORCESPAMMY, t.T{
			"ToUser":       receiver.AtName(ctx),
			"Participants": nparticipants,
			"Sats":         sats,
			"Fund":         sats * nparticipants,
			"Registered":   u.AtName(ctx),
		}, fundraiseKeyboard(ctx, "", u.Id, receiver.Id, nparticipants, sats))

		go u.track("fundraise created", map[string]interface{}{
			"group": groupId,
			"sats":  sats,
			"n":     nparticipants,
		})
	case opts["hide"].(bool):
		hiddenid := getHiddenId(message) // deterministic

		hiddenmessage := HiddenMessage{
			Public: true,
		}

		// if there's a replyto, use that as a forward/copy
		if message.ReplyToMessage != nil {
			hiddenmessage.CopyMessage = &TelegramCopyMessage{
				MessageID: message.ReplyToMessage.MessageID,
				ChatID:    message.Chat.ID,
			}
		}

		// or use the inline message
		// -- or if there's a replyo and inline, the inline part is the preview
		if icontent, ok := opts["<message>"]; ok {
			message := strings.Join(icontent.([]string), " ")
			if hiddenmessage.CopyMessage != nil {
				// if we are using the replyto forward,
				// this is the preview
				hiddenmessage.Preview = message
			} else {
				// otherwise parse the ~ thing
				contentparts := strings.SplitN(message, "~", 2)
				if len(contentparts) == 2 {
					hiddenmessage.Preview = contentparts[0]
					hiddenmessage.Content = contentparts[1]
				} else {
					hiddenmessage.Content = message
				}
			}
		}

		if hiddenmessage.Content == "" && hiddenmessage.CopyMessage == nil {
			// no content found
			send(ctx, u, t.ERROR, t.T{"Err": "No content to hide."})
			return
		}

		msats, err := parseSatoshis(opts)
		if err != nil || msats == 0 {
			send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
			return
		}
		hiddenmessage.Satoshis = int(msats / 1000)

		if private := opts["--private"].(bool); private {
			hiddenmessage.Public = false
		}

		hiddenmessage.Crowdfund, _ = opts.Int("--crowdfund")
		if hiddenmessage.Crowdfund > 1 {
			hiddenmessage.Public = true
		} else {
			hiddenmessage.Crowdfund = 1
		}

		hiddenmessage.Times, _ = opts.Int("--revealers")
		if hiddenmessage.Times > 1 {
			hiddenmessage.Public = false
			hiddenmessage.Crowdfund = 1
		} else {
			hiddenmessage.Times = 0
		}

		hiddenmessagejson, err := json.Marshal(hiddenmessage)
		if err != nil {
			send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
			return
		}

		err = rds.Set(fmt.Sprintf("hidden:%d:%s", u.Id, hiddenid), string(hiddenmessagejson), s.HiddenMessageTimeout).Err()
		if err != nil {
			send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
			return
		}

		templateParams := t.T{
			"HiddenId": hiddenid,
			"Message":  hiddenmessage,
		}

		var shareKeyboard interface{}
		if hiddenmessage.CopyMessage != nil && hiddenmessage.Public {
			// copyMessages can't be sent in public groups through the inline thing
			// so don't show the keyboard in this case
			templateParams["WithInstructions"] = true
		} else {
			// normal flow, has the share button (that creates an inline query)
			siq := "reveal " + hiddenid
			shareKeyboard = &tgbotapi.InlineKeyboardMarkup{
				InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
					{
						tgbotapi.InlineKeyboardButton{
							Text:              translate(ctx, t.HIDDENSHAREBTN),
							SwitchInlineQuery: &siq,
						},
					},
				},
			}
		}

		send(ctx, u, t.HIDDENWITHID, templateParams, shareKeyboard, message.MessageID)

		go u.track("hide", map[string]interface{}{
			"sats":      hiddenmessage.Satoshis,
			"times":     hiddenmessage.Times,
			"crowdfund": hiddenmessage.Crowdfund,
			"public":    hiddenmessage.Public,
		})

		break
	case opts["reveal"].(bool):
		go func() {
			hiddenid := opts["<hidden_message_id>"].(string)

			redisKey, ok := findHiddenKey(hiddenid)
			if !ok {
				send(ctx, u, t.HIDDENMSGNOTFOUND, nil, message.MessageID)
				return
			}

			_, _, hidden, err := getHiddenMessage(ctx, redisKey)
			if err != nil {
				send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
				return
			}

			send(ctx, u, g, FORCESPAMMY,
				hidden.Preview, revealKeyboard(ctx, redisKey, hidden, 0))
		}()
	case opts["transactions"].(bool):
		go handleTransactionList(ctx, opts)
	case opts["balance"].(bool):
		go handleBalance(ctx, opts)
	case opts["pay"].(bool), opts["withdraw"].(bool), opts["decode"].(bool):
		if opts["lnurl"].(bool) {
			// create an lnurl-withdraw voucher
			handleCreateLNURLWithdraw(ctx, opts)
		} else {
			// normal payment flow
			handlePay(ctx, u, opts)
		}
	case opts["receive"].(bool), opts["invoice"].(bool), opts["fund"].(bool):
		desc := getVariadicFieldOrReplyToContent(ctx, opts, "<description>")
		go handleInvoice(ctx, opts, desc)
	case opts["lnurl"].(bool):
		go handleLNURL(ctx, opts["<lnurl>"].(string), handleLNURLOpts{
			anonymous: opts["--anonymous"].(bool),
		})
	case opts["rename"].(bool):
		go func() {
			ctx = context.WithValue(ctx, "spammy", true)

			if message.Chat.Type == "private" {
				send(ctx, u, t.MUSTBEGROUP)
				return
			}

			name := getVariadicFieldOrReplyToContent(ctx, opts, "<name>")

			price := g.getRenamePrice()
			if price == 0 {
				send(ctx, g, t.GROUPNOTRENAMABLE)
				return
			}
			if !isAdmin(message.Chat, &bot.Self) {
				send(ctx, g, t.GROUPNOTRENAMABLE)
				return
			}

			send(ctx, g, t.RENAMEPROMPT, t.T{
				"Sats": price,
				"Name": name,
			}, renameKeyboard(ctx, u.Id, message.Chat.ID, price, name))

			go u.track("rename started", map[string]interface{}{
				"group": groupId,
				"sats":  price,
			})
		}()
	case opts["fine"].(bool):
		go handleFine(ctx, opts)
	case opts["help"].(bool):
		command, _ := opts.String("<command>")
		go u.track("help", map[string]interface{}{"command": command})
		handleHelp(ctx, command)
		break
	case opts["toggle"].(bool):
		go func() {
			if message.Chat.Type == "private" {
				// on private chats we can use /toggle language <lang>, nothing else
				switch {
				case opts["language"].(bool):
					if lang, err := opts.String("<lang>"); err == nil {
						go u.track("toggle language", map[string]interface{}{
							"lang":     lang,
							"personal": true,
						})
						log.Info().Stringer("user", &u).Str("language", lang).
							Msg("toggling language")
						err := setLanguage(u.TelegramChatId, lang)
						if err != nil {
							log.Warn().Err(err).Msg("failed to toggle language")
							send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
							break
						}
						send(ctx, u, t.LANGUAGEMSG, t.T{"Language": lang})
					} else {
						send(ctx, u, t.LANGUAGEMSG, t.T{"Language": u.Locale})
					}
				default:
					send(ctx, u, t.MUSTBEGROUP)
					return
				}

				return
			}
			if !isAdmin(message.Chat, message.From) {
				send(ctx, u, t.MUSTBEADMIN)
				return
			}

			g, err := ensureTelegramGroup(message.Chat.ID, u.Locale)
			if err != nil {
				log.Warn().Err(err).Stringer("user", &u).Int64("group", message.Chat.ID).
					Msg("failed to ensure group")
				return
			}

			ctx = context.WithValue(ctx, "spammy", true)
			switch {
			case opts["ticket"].(bool):
				log.Info().Stringer("group", &g).Msg("toggling ticket")
				msats, err := parseSatoshis(opts)
				if err != nil {
					g.setTicketPrice(0)
					send(ctx, g, t.FREEJOIN)
				}
				sats := int(msats / 1000)

				go u.track("toggle ticket", map[string]interface{}{
					"group": groupId,
					"sats":  sats,
				})

				g.setTicketPrice(sats)
				if sats > 0 {
					send(ctx, g, t.TICKETSET, t.T{"Sat": sats})
				}
			case opts["expensive"].(bool):
				log.Info().Stringer("group", &g).Msg("toggling expensive")
				msats, _ := parseSatoshis(opts)
				pattern, _ := opts.String("<pattern>")
				pattern = strings.ToLower(pattern)
				sats := int(msats / 1000)

				if sats != 0 && (sats > 50 || sats < 5) {
					send(ctx, g, t.ERROR, t.T{
						"Err": "price per message must be between 5 and 50 sat.",
					})
					return
				}

				if sats == 0 {
					g.setExpensive(0, "")
					send(ctx, g, t.FREETALK)
				} else if _, err := regexp.Compile(pattern); err != nil {
					send(ctx, g, t.ERROR, t.T{"Err": err.Error()})
					return
				}

				go u.track("toggle expensive", map[string]interface{}{
					"group":   groupId,
					"sats":    sats,
					"pattern": pattern,
				})

				g.setExpensive(sats, pattern)
				if sats > 0 {
					send(ctx, g, t.EXPENSIVEMSG, t.T{
						"Price":   sats,
						"Pattern": pattern,
					})
				}
			case opts["renamable"].(bool):
				log.Info().Stringer("group", &g).Msg("toggling renamable")
				msats, err := parseSatoshis(opts)
				if err != nil {
					g.setTicketPrice(0)
					send(ctx, g, t.FREEJOIN)
				}
				sats := int(msats / 1000)

				go u.track("toggle renamable", map[string]interface{}{
					"group": groupId,
					"sats":  sats,
				})

				g.setRenamePrice(sats)
				if sats > 0 {
					send(ctx, g, t.RENAMABLEMSG, t.T{"Sat": sats})
				}
			case opts["spammy"].(bool):
				log.Debug().Stringer("group", &g).Msg("toggling spammy")
				spammy, err := g.toggleSpammy()
				if err != nil {
					log.Warn().Err(err).Msg("failed to toggle spammy")
					send(ctx, g, t.ERROR, t.T{"Err": err.Error()})
					break
				}

				go u.track("toggle spammy", map[string]interface{}{
					"group":  groupId,
					"spammy": spammy,
				})

				send(ctx, g, t.SPAMMYMSG, t.T{"Spammy": spammy})
			case opts["coinflips"].(bool):
				log.Debug().Stringer("group", &g).Msg("toggling coinflips")
				enabled, err := g.toggleCoinflips()
				if err != nil {
					log.Warn().Err(err).Msg("failed to toggle coinflips")
					send(ctx, g, t.ERROR, t.T{"Err": err.Error()})
					break
				}

				go u.track("toggle coinflips", map[string]interface{}{
					"group":   groupId,
					"enabled": enabled,
				})

				send(ctx, g, t.COINFLIPSENABLEDMSG, t.T{"Enabled": enabled})
			case opts["language"].(bool):
				if lang, err := opts.String("<lang>"); err == nil {
					log.Info().Stringer("group", &g).Str("language", lang).
						Msg("toggling language")
					err := setLanguage(message.Chat.ID, lang)
					if err != nil {
						log.Warn().Err(err).Msg("failed to toggle language")
						send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
						break
					}

					go u.track("toggle language", map[string]interface{}{
						"group": groupId,
						"lang":  lang,
					})

					send(ctx, g, t.LANGUAGEMSG, t.T{"Language": lang})
				} else {
					send(ctx, g, t.LANGUAGEMSG, t.T{"Language": g.Locale})
				}

			}
		}()
	case opts["sats4ads"].(bool):
		handleSats4Ads(ctx, u, opts)
	case opts["satoshis"].(bool), opts["calc"].(bool):
		msats, err := parseSatoshis(opts)
		if err == nil {
			send(ctx, fmt.Sprintf("%.15g sat", float64(msats)/1000))
		}
	case opts["moon"].(bool):
		moonURLs := []string{
			"https://www.currexy.com/upload/naujienos/original/2017/09/moon-btc-34899.jpg",
			"https://cryptocurrencies.com.au/wp-content/uploads/2019/06/bitcoin-moon-art.jpg",
			"https://cryptodailygazette.com/wp-content/uploads/2019/03/bitcoin-to-the-moon-1-650x364.jpg",
			"https://assets.pando.com/uploads/2015/01/bitcoin-to-the-moon.jpg",
			"https://blokt.com/wp-content/uploads/2019/02/Rocket-launch-to-moon-as-a-Bitcoin-price-increase-concept.-The-elements-of-this-image-furnished-by-NASA-Image.jpg",
			"https://miro.medium.com/max/1838/0*xJEt4-dCPp9L03fi.jpg",
			"http://www.tothemoon.com/wp-content/uploads/2018/05/bitcoin-to-the-moon-crptocurrency-645x366.jpg",
		}

		choice := moonURLs[rand.Intn(len(moonURLs))]
		moonURL, _ := url.Parse(choice)
		send(ctx, g, FORCESPAMMY, moonURL)
	}
}
