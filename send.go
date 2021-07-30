package main

import (
	"context"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/go-lnurl"
	"github.com/fiatjaf/lntxbot/t"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func handleSend(ctx context.Context, opts docopt.Opts) {
	u := ctx.Value("initiator").(User)

	var g GroupChat
	if ig := ctx.Value("group"); ig != nil {
		g, _ = ig.(GroupChat)
		ctx = context.WithValue(ctx, "spammy", g.isSpammy())
	}

	// sending money to others
	var (
		msats       int64
		receiver    *User
		username    string
		description string
	)

	// get quantity
	msats, err := parseSatoshis(opts)
	amtraw := opts["<satoshis>"].(string)

	if err != nil || msats <= 0 {
		send(ctx, u, t.ERROR, t.T{"Err": err.Error()})
		return
	} else {
		username, _ = opts.String("<receiver>")
	}

	anonymous := false
	if opts["anonymously"].(bool) ||
		opts["--anonymous"].(bool) || opts["sendanonymously"].(bool) {
		anonymous = true
	}

	if extra, ok := opts["<description>"].([]string); ok {
		description = strings.Join(extra, " ")
	}

	switch message := ctx.Value("message").(type) {
	case *discordgo.Message: // discord
		receiver, err = examineDiscordUsername(username)
		if err != nil {
			log.Warn().Err(err).Str("username", username).
				Msg("failed to examine discord username")
			send(ctx, g, u, t.SAVERECEIVERFAIL)
			return
		}

		goto ensured
	case *tgbotapi.Message: // telegram
		receiver, err = examineTelegramUsername(username)
		if receiver != nil {
			goto ensured
		}

		// no username, this may be a reply-tip
		if message.ReplyToMessage != nil {
			// the <receiver> part is useless as a username,
			// but it can part of the tip description
			description = username + " " + description

			log.Debug().Str("desc", description).Msg("it's a reply-tip")
			reply := message.ReplyToMessage

			var cas int
			rec, cas, err := ensureTelegramUser(
				reply.From.ID, reply.From.UserName, reply.From.LanguageCode)
			receiver = &rec
			if err != nil {
				send(ctx, g, u, t.SAVERECEIVERFAIL)
				log.Warn().Err(err).Int("case", cas).
					Str("username", reply.From.UserName).
					Int("id", reply.From.ID).
					Msg("failed to ensure user on reply-tip")
				return
			}
			goto ensured
		}
	default:
		// maybe this is a lightning address like username@domain.com?
		if _, _, ok := lnurl.ParseInternetIdentifier(username); ok {
			handleLNURL(ctx, username, handleLNURLOpts{
				payAmountWithoutPrompt: &msats,
			})
			// end here since the flow will proceed on handleLNURL
			return
		}
	}

	// if we ever reach this point then it's because the receiver is missing.
	if err != nil {
		log.Warn().Err(err).Str("username", username).Msg("error parsing username")
	}
	send(ctx, g, u, t.CANTSENDNORECEIVER, t.T{"Sats": opts["<satoshis>"]})
	return

ensured:
	err = u.sendInternally(
		ctx,
		*receiver,
		anonymous,
		msats,
		int64(float64(msats)*0.003),
		strings.TrimSpace(description),
		"",
		"",
	)
	if err != nil {
		log.Warn().Err(err).
			Str("from", u.Username).
			Str("to", receiver.AtName(ctx)).
			Msg("failed to send/tip")
		send(ctx, g, u, t.FAILEDSEND, t.T{"Err": err.Error()})
		return
	}

	// notify sender
	send(ctx, u, t.USERSENTTOUSER, t.T{
		"User":    receiver.AtName(ctx),
		"Sats":    msats / 1000,
		"RawSats": amtraw,
		"ReceiverHasNoChat": receiver.TelegramChatId == 0 &&
			receiver.DiscordChannelId == "",
	})

	// notify receiver
	if receiver.hasPrivateChat() && !ctx.Value("spammy").(bool) {
		// if possible privately
		if anonymous {
			send(ctx, receiver, t.RECEIVEDSATSANON, t.T{"Sats": msats / 1000})
		} else {
			send(ctx, receiver, t.USERSENTYOUSATS, t.T{
				"User":    u.AtName(ctx),
				"Sats":    msats / 1000,
				"RawSats": amtraw,
			})
		}
	}

	if !receiver.hasPrivateChat() || ctx.Value("spammy").(bool) {
		// publicly if the receiver doesn't have a chat or if the group is spammy
		send(ctx, g, u, t.SATSGIVENPUBLIC, t.T{
			"From": u.AtName(ctx),
			"To":   receiver.AtName(ctx),
			"Sats": msats / 1000,
			"ClaimerHasNoChat": receiver.TelegramChatId == 0 &&
				receiver.DiscordChannelId == "",
			"BotName": s.ServiceId,
		}, ctx.Value("message"), FORCESPAMMY)
	}
}
