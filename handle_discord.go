package main

import (
	"context"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/lntxbot/t"
)

func addDiscordHandlers() {
	discord.AddHandler(handleDiscordMessage)
	discord.AddHandler(handleDiscordReaction)
}

func handleDiscordMessage(dgs *discordgo.Session, m *discordgo.MessageCreate) {
	ctx := context.WithValue(context.Background(), "origin", "discord")

	message := m.Message
	if message.Author.Bot {
		return
	}

	ctx = context.WithValue(ctx, "message", message)

	// this is just to send to amplitude
	var group *int64 = nil

	// declaring stuff so we can use goto
	var (
		u           User
		messageText string
		opts        docopt.Opts
		isCommand   bool
		commandName string
	)

	if message.Content[0] != '$' {
		if bolt11, lnurltext, ok := searchForInvoice(u, message); ok {
			if bolt11 != "" {
				commandName = "$pay"
				opts, _, _ = parse("/pay " + bolt11)
				goto parsed
			}
			if lnurltext != "" {
				commandName = "$lnurl"
				opts, _, _ = parse("/lnurl " + lnurltext)
				goto parsed
			}
		}

		return
	}

	messageText = "/" + message.Content[1:]
	log.Debug().Str("t", messageText).Int("user", u.Id).Msg("got discord message")

	opts, isCommand, err = parse(messageText)
	if !isCommand {
		// is this a reply we're waiting for?
		// TODO
		return
	}
	if err != nil {
		if message.GuildID == "" {
			// only tell we don't understand commands when in a private chat
			// because these commands we're not understanding
			// may be targeting other bots in a group, so we're spamming people.
			log.Debug().Err(err).Str("command", messageText).
				Msg("failed to parse command")

			method := strings.Split(messageText, " ")[0][1:]
			handled := handleHelp(u, method)
			if !handled {
				send(ctx, u, t.WRONGCOMMAND)
			}
		}
		return
	}

	commandName = "$" + strings.Split(strings.Split(messageText, " ")[0], "_")[0][1:]
	go u.track("command", map[string]interface{}{
		"command": commandName,
		"group":   group,
	})

parsed:
	ctx = context.WithValue(ctx, "command", command)

	if message.GuildID != "" {
		// TODO
	} else {
		user, err := ensureDiscordUser(
			message.Author.ID,
			message.Author.Username+"#"+message.Author.Discriminator,
			message.Author.Locale)
		if err != nil {
			log.Warn().Err(err).
				Str("username",
					message.Author.Username+"#"+message.Author.Discriminator).
				Str("id", message.Author.ID).
				Msg("failed to ensure user")
			return
		}
		u = user

		// stop if temporarily banned
		if _, ok := s.Banned[u.Id]; ok {
			log.Debug().Int("id", u.Id).Msg("got request from banned user")
			return
		}

		ctx = context.WithValue(ctx, "initiator", u)
	}

	// by default we use the user locale for the group object, because
	// we may end up sending the message to the user instead of to the group
	// (if, for example, the user calls /coinflip on his own chat) then
	// we at least want the correct language used there.
	// g := GroupChat{TelegramId: message.Chat.ID, Locale: u.Locale}

	if message.GuildID == "" {
		// after ensuring the user we should always enable him to
		// receive payment notifications and so on, as not all people will
		// remember to call /start
		u.setChannel(message.ChannelID)
		// g.TelegramId = -g.TelegramId // because we invert when sending a message

	} else {
		// when we're in a group, load the group
		// loadedGroup, err := loadGroup(message.Chat.ID)
		// if err != nil {
		// 	if err != sql.ErrNoRows {
		// 		log.Warn().Err(err).Int64("id", message.Chat.ID).Msg("failed to load group")
		// 	}
		// 	// proceed with an empty group (manually defined before)
		// } else {
		// 	// we manage to load a group, use it then
		// 	g = loadedGroup
		// }

		// group = &message.Chat.ID
	}

	if opts["paynow"].(bool) {
		opts["pay"] = true
		opts["now"] = true
	}

	switch {
	case opts["dollar"].(bool):
		sats, err := parseSatoshis(opts)
		if err == nil {
			sendDiscordMessage(u.DiscordChannelId, getDollarPrice(int64(sats)*1000))
		}
		break
	case opts["start"].(bool), opts["tutorial"].(bool):
		if message.GuildID == "" {
			if tutorial, err := opts.String("<tutorial>"); err != nil || tutorial == "" {
				handleTutorial(u, tutorial)
			} else {
				send(ctx, u, t.WELCOME)
				handleTutorial(u, "")
			}
			go u.track("start", nil)
		}
		break
	case opts["stop"].(bool):
		if message.GuildID == "" {
			u.unsetChannel()
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
	case opts["log"].(bool):
		go handleLogView(ctx, opts)
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
			handlePay(ctx, opts)
		}
	case opts["receive"].(bool), opts["invoice"].(bool), opts["fund"].(bool):
		desc, _ := opts.String("<description>")
		go handleInvoice(u, opts, desc, 0)
	case opts["lnurl"].(bool):
		go handleLNURL(u, opts["<lnurl>"].(string), handleLNURLOpts{})
	case opts["help"].(bool):
		command, _ := opts.String("<command>")
		go u.track("help", map[string]interface{}{"command": command})
		go handleHelp(u, command)
		break
	default:
		send(ctx, u, t.ERROR, t.T{"Err": "not implemented on Discord yet."})
	}
}

func handleDiscordReaction(dgs *discordgo.Session, m *discordgo.MessageReactionAdd) {
	ctx := context.WithValue(context.Background(), "origin", "discord")
	reaction := m.MessageReaction

	log.Print("got emoji ", reaction.Emoji.Name)

	switch reaction.Emoji.Name {
	case "⚡":
		log.Print("lightning emoji!")
		// potentially an user confirming a $pay command
		handlePayReactionConfirm(reaction)
	}
}
