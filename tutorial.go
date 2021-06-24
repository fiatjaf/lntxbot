package main
//
import (
	"context"
	"time"

	"github.com/fiatjaf/lntxbot/t"
)

func handleTutorial(ctx context.Context, section string) {
	log.Debug().Str("section", section).Msg("user going through tutorial")

	switch section {
	case "wallet":
		send(ctx, t.TUTORIALWALLET, t.T{"BotName": s.ServiceId})
	case "":
		// do all sections
		go func() {
			time.Sleep(5 * time.Second)
			send(ctx, t.TUTORIALWALLET, t.T{"BotName": s.ServiceId})
			time.Sleep(10 * time.Second)
			send(ctx, t.TUTORIALAPPS, t.T{"BotName": s.ServiceId})
			time.Sleep(10 * time.Second)
			send(ctx, t.TUTORIALTWITTER, t.T{"BotName": s.ServiceId})
		}()
	}
}
