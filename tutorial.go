package main

import (
	"time"

	"git.alhur.es/fiatjaf/lntxbot/t"
)

func handleTutorial(u User, section string) {
	log.Debug().Str("section", section).Str("username", u.Username).Msg("user going through tutorial")

	switch section {
	case "wallet":
		tutorialWallet(u)
	case "friends":
		tutorialFriends(u)
	case "bluewallet":
		tutorialBlueWallet(u)
	case "apps":
		tutorialApps(u)
	case "":
		// do all sections
		go func() {
			time.Sleep(15 * time.Second)
			tutorialWallet(u)
			time.Sleep(30 * time.Second)
			tutorialBlueWallet(u)
			time.Sleep(30 * time.Second)
			tutorialApps(u)
			time.Sleep(30 * time.Second)
			// tutorialFriends(u)
		}()
	}
}

func tutorialWallet(u User) {
	sendMessageWithAnimationId(u.ChatId, s.TutorialWalletVideoId,
		translateTemplate(t.TUTORIALWALLET, u.Locale, t.T{"BotName": s.ServiceId}))
}

func tutorialBlueWallet(u User) {
	sendMessageWithAnimationId(u.ChatId, s.TutorialBlueVideoId,
		translateTemplate(t.TUTORIALBLUE, u.Locale, t.T{"BotName": s.ServiceId}))
}

func tutorialFriends(u User) {

}

func tutorialApps(u User) {

}
