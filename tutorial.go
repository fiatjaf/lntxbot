package main

import (
	"context"
	"time"

	"github.com/fiatjaf/lntxbot/t"
)

func handleTutorial(ctx context.Context, section string) {
	log.Debug().Str("section", section).Msg("user going through tutorial")

	switch section {
	case "wallet":
		tutorialWallet(ctx)
	case "apps":
		tutorialApps(ctx)
	case "":
		// do all sections
		go func() {
			time.Sleep(15 * time.Second)
			tutorialWallet(ctx)
			time.Sleep(120 * time.Second)
			tutorialApps(ctx)
			time.Sleep(240 * time.Second)
			tutorialTwitter(ctx)
		}()
	}
}

func tutorialWallet(ctx context.Context) {
	// text := translateTemplate(ctx, t.TUTORIALWALLET, t.T{"BotName": s.ServiceId})

	// if u.TelegramChatId != 0 {
	// 	sendTelegramMessageWithAnimationId(
	// 		u.TelegramChatId,
	// 		s.TutorialWalletVideoId,
	// 		text,
	// 	)
	// } else {
	// 	md, _ := mdConverter.ConvertString(text)
	// 	discord.ChannelMessageSendEmbed(u.DiscordChannelId, &discordgo.MessageEmbed{
	// 		Description: md,
	// 		Video: &discordgo.MessageEmbedVideo{
	// 			URL: s.ServiceURL + "/static/wallet-demo.mp4",
	// 		},
	// 	})
	// }
}

func tutorialApps(ctx context.Context) {
	send(ctx, t.TUTORIALAPPS, t.T{"BotName": s.ServiceId})
}

func tutorialTwitter(ctx context.Context) {
	send(ctx, t.TUTORIALTWITTER, t.T{"BotName": s.ServiceId})
}
