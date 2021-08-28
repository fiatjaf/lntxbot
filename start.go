package main

import (
	"context"

	"github.com/fiatjaf/lntxbot/t"
)

func handleStart(ctx context.Context) {
	send(ctx, t.START, t.T{})
}
