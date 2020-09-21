package main

import (
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/fiatjaf/lntxbot/t"
)

func handleTutorial(u User, section string) {
	log.Debug().Str("section", section).Str("username", u.Username).Msg("user going through tutorial")

	switch section {
	case "wallet":
		tutorialWallet(u)
	case "apps":
		tutorialApps(u)
	case "":
		// do all sections
		go func() {
			time.Sleep(15 * time.Second)
			tutorialWallet(u)
			time.Sleep(120 * time.Second)
			tutorialApps(u)
			time.Sleep(240 * time.Second)
			tutorialTwitter(u)
		}()
	}
}

func tutorialWallet(u User) {
	text := translateTemplate(t.TUTORIALWALLET, u.Locale, t.T{"BotName": s.ServiceId})

	if u.isTelegram() {
		sendTelegramMessageWithAnimationId(
			u.TelegramChatId,
			s.TutorialWalletVideoId,
			text,
		)
	} else if u.isDiscord() {
		md, _ := mdConverter.ConvertString(text)
		discord.ChannelMessageSendEmbed(u.DiscordChannelId, &discordgo.MessageEmbed{
			Description: md,
			Video: &discordgo.MessageEmbedVideo{
				URL: s.ServiceURL + "/static/wallet-demo.mp4",
			},
		})
	}
}

func tutorialApps(u User) {
	u.notify(t.TUTORIALAPPS, t.T{"BotName": s.ServiceId})
}

func tutorialTwitter(u User) {
	u.notify(t.TUTORIALTWITTER, t.T{"BotName": s.ServiceId})
}
