package main

import (
	"time"

	"github.com/fiatjaf/lntxbot/t"
)

func handleTutorial(u User, section string) {
	log.Debug().Str("section", section).Str("username", u.Username).Msg("user going through tutorial")

	switch section {
	case "wallet":
		tutorialWallet(u)
	case "bluewallet":
		tutorialBlueWallet(u)
	case "apps":
		tutorialApps(u)
	case "":
		// do all sections
		go func() {
			time.Sleep(15 * time.Second)
			tutorialWallet(u)
			time.Sleep(120 * time.Second)
			tutorialBlueWallet(u)
			time.Sleep(120 * time.Second)
			tutorialApps(u)
			time.Sleep(240 * time.Second)
			tutorialTwitter(u)
		}()
	}
}

func tutorialWallet(u User) {
	sendTelegramMessageWithAnimationId(u.TelegramChatId, s.TutorialWalletVideoId,
		translateTemplate(t.TUTORIALWALLET, u.Locale, t.T{"BotName": s.ServiceId}))
}

func tutorialBlueWallet(u User) {
	sendTelegramMessageWithAnimationId(u.TelegramChatId, s.TutorialBlueVideoId,
		translateTemplate(t.TUTORIALBLUE, u.Locale, t.T{"BotName": s.ServiceId}))
}

func tutorialApps(u User) {
	sendTelegramMessage(u.TelegramChatId, translateTemplate(t.TUTORIALAPPS, u.Locale, t.T{"BotName": s.ServiceId}))
}

func tutorialTwitter(u User) {
	sendTelegramMessage(u.TelegramChatId, translateTemplate(t.TUTORIALTWITTER, u.Locale, t.T{"BotName": s.ServiceId}))
}
