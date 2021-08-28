package main

import (
	"context"

	"github.com/fiatjaf/lntxbot/t"
)

func handleStart(ctx context.Context) {
	yourname := ctx.Value("initiator").(User).Username
	if yourname == "" {
		yourname = "yourname"
	}
	send(ctx, t.START, t.T{"YourName": yourname})
}
