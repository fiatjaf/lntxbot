package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/lntxbot/t"
)

func handleDiscordMessage(dgs *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Message.Author.Bot {
		return
	}

	// this is just to send to amplitude
	var group *int64 = nil

	// declaring stuff so we can use goto
	var (
		u           User
		messageText string
		opts        docopt.Opts
		isCommand   bool
	)

	if m.Message.Content[0] != '$' {
		if bolt11, lnurltext, ok := searchForInvoice(u, m.Message); ok {
			if bolt11 != "" {
				opts, _, _ = parse("/pay " + bolt11)
				goto parsed
			}
			if lnurltext != "" {
				opts, _, _ = parse("/lnurl " + lnurltext)
				goto parsed
			}
		}

		return
	}

	messageText = "/" + m.Message.Content[1:]

	if m.Message.GuildID != "" {
		u = User{
			DiscordChannelId: m.Message.ChannelID,
			Locale:           "en",
		}
	} else {
		user, tcase, err := ensureDiscordUser(
			m.Message.Author.ID,
			m.Message.Author.Username+"#"+m.Message.Author.Discriminator,
			m.Message.Author.Locale)
		if err != nil {
			log.Warn().Err(err).Int("case", tcase).
				Str("username",
					m.Message.Author.Username+"#"+m.Message.Author.Discriminator).
				Str("id", m.Message.Author.ID).
				Msg("failed to ensure user")
			return
		}
		u = user

		// stop if temporarily banned
		if _, ok := s.Banned[u.Id]; ok {
			log.Debug().Int("id", u.Id).Msg("got request from banned user")
			return
		}
	}

	// by default we use the user locale for the group object, because
	// we may end up sending the message to the user instead of to the group
	// (if, for example, the user calls /coinflip on his own chat) then
	// we at least want the correct language used there.
	// g := GroupChat{TelegramId: message.Chat.ID, Locale: u.Locale}

	if m.Message.GuildID == "" {
		// after ensuring the user we should always enable him to
		// receive payment notifications and so on, as not all people will
		// remember to call /start
		u.setChannel(m.Message.ChannelID)
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

	log.Debug().Str("t", messageText).Int("user", u.Id).Msg("got message")

	opts, isCommand, err = parse(messageText)
	if !isCommand {
		// is this a reply we're waiting for?
	}
	if err != nil {
		if m.Message.GuildID == "" {
			// only tell we don't understand commands when in a private chat
			// because these commands we're not understanding
			// may be targeting other bots in a group, so we're spamming people.
			log.Debug().Err(err).Str("command", messageText).
				Msg("failed to parse command")

			method := strings.Split(messageText, " ")[0][1:]
			handled := handleHelp(u, method)
			if !handled {
				u.notify(t.WRONGCOMMAND, nil)
			}
		}

		// save the fact that we didn't understand this so it can be edited
		// and reevaluated
		rds.Set(fmt.Sprintf("parseerror:%s", m.Message.ID), "1", time.Minute*5)

		return
	}

	go u.track("command", map[string]interface{}{
		"command": strings.Split(strings.Split(messageText, " ")[0], "_")[0],
		"group":   group,
	})

parsed:
	// if we reached this point we should make sure the command won't be editable again
	rds.Del(fmt.Sprintf("parseerror:%s", m.Message.ID))

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
	}
}
