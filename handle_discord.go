package main

import (
	"context"
	"fmt"
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
	if message.Author.Bot || len(message.Content) == 0 {
		return
	}

	ctx = context.WithValue(ctx, "message", message)

	// this is just to send to amplitude
	var groupId *string = nil

	// declaring stuff so we can use goto
	var (
		err         error
		u           User
		g           GroupChat
		messageText string
		opts        docopt.Opts
		isCommand   bool
		commandName string
	)

	if message.Content[0] != '$' {
		if bolt11, lnurltext, address, ok := searchForInvoice(ctx); ok {
			if bolt11 != "" {
				commandName = "$pay"
				opts, _, err = parse("/pay " + bolt11)
				if err != nil {
					return
				}
				goto parsed
			}
			if lnurltext != "" {
				commandName = "$lnurl"
				opts, _, err = parse("/lnurl " + lnurltext)
				if err != nil {
					return
				}
				goto parsed
			}
			if address != "" {
				commandName = "$lnurl"
				opts, _, err = parse("/lnurl " + address)
				if err != nil {
					return
				}
				goto parsed
			}
		}

		return
	}

	messageText = "/" + message.Content[1:]

	opts, isCommand, err = parse(messageText)
	log.Debug().Str("t", messageText).Stringer("user", &u).Err(err).Msg("discord message")
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
			method := strings.Split(messageText, " ")[0][1:]
			handled := handleHelp(ctx, method)
			if !handled {
				send(ctx, u, t.WRONGCOMMAND)
			}
		}
		return
	}

	commandName = "$" + strings.Split(strings.Split(messageText, " ")[0], "_")[0][1:]
	go u.track("command", map[string]interface{}{
		"command": commandName,
		"group":   groupId,
	})

parsed:
	ctx = context.WithValue(ctx, "command", commandName)

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
	ctx = context.WithValue(ctx, "initiator", u)

	// stop if temporarily banned
	if _, ok := s.Banned[u.Id]; ok {
		log.Debug().Int("id", u.Id).Msg("got request from banned user")
		return
	}

	if message.GuildID == "" {
		// after ensuring the user we should always enable him to
		// receive payment notifications and so on, as not all people will
		// remember to call /start
		u.setChannel(message.ChannelID)
	} else {
		// when we're in a group, put that in the context
		g = GroupChat{DiscordGuildId: message.GuildID, Locale: u.Locale}
		ctx = context.WithValue(ctx, "group", g)

		groupId = &message.GuildID
	}

	if opts["paynow"].(bool) {
		opts["pay"] = true
		opts["now"] = true
	}

	switch {
	case opts["start"].(bool):
		handleStart(ctx)
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
		desc, _ := opts.String("<description>")
		go handleInvoice(ctx, opts, desc)
	case opts["lnurl"].(bool):
		go handleLNURL(ctx, opts["<lnurl>"].(string), handleLNURLOpts{
			anonymous: opts["--anonymous"].(bool),
		})
	case opts["send"].(bool), opts["tip"].(bool):
		go u.track("send", map[string]interface{}{
			"group":     groupId,
			"reply-tip": false,
		})
		handleSend(ctx, opts)
	case opts["help"].(bool):
		command, _ := opts.String("<command>")
		go u.track("help", map[string]interface{}{"command": command})
		go handleHelp(ctx, command)
	case opts["satoshis"].(bool), opts["calc"].(bool):
		msats, err := parseSatoshis(opts)
		if err == nil {
			send(ctx, fmt.Sprintf("%.15g sat", float64(msats)/1000))
		}
	default:
		send(ctx, u, t.ERROR, t.T{"Err": "not implemented on Discord yet."})
	}
}

func handleDiscordReaction(dgs *discordgo.Session, m *discordgo.MessageReactionAdd) {
	ctx := context.WithValue(context.Background(), "origin", "discord")
	reaction := m.MessageReaction

	ctx = context.WithValue(ctx,
		"discordMessageID",
		discordMessageID(reaction.GuildID, reaction.ChannelID, reaction.MessageID))

	switch reaction.Emoji.Name {
	case "⚡":
		// potentially an user confirming a $pay command
		handlePayReactionConfirm(ctx, reaction)
	}
}
