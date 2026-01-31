package main

import (
	"context"
	"fmt"
	"time"

	"github.com/celestix/gotgproto"
	"github.com/celestix/gotgproto/sessionMaker"
	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/contrib/middleware/ratelimit"
	"github.com/gotd/td/telegram"
	"golang.org/x/time/rate"
)

var GoTGProtoClient *gotgproto.Client
var GoTGProtoContext context.Context

func gotdClientInit() error {
	var err error

	waiter := floodwait.NewWaiter().WithCallback(func(ctx context.Context, wait floodwait.FloodWait) {
		fmt.Printf("Waiting for flood, dur: %d\n", wait.Duration)
	})
	ratelimiter := ratelimit.New(rate.Every(time.Millisecond*100), 30)

	GoTGProtoClient, err = gotgproto.NewClient(
		Config.AppID,
		Config.AppHash,
		gotgproto.ClientTypeBot(Config.Token),
		&gotgproto.ClientOpts{
			InMemory:    true,
			Session:     sessionMaker.SimpleSession(),
			Middlewares: []telegram.Middleware{waiter, ratelimiter},
			RunMiddleware: func(origRun func(ctx context.Context, f func(ctx context.Context) error) (err error), ctx context.Context, f func(ctx context.Context) (err error)) (err error) {
				return origRun(ctx, func(ctx context.Context) error {
					return waiter.Run(ctx, f)
				})
			},
		},
	)
	if err != nil {
		return err
	}

	GoTGProtoContext = GoTGProtoClient.CreateContext().Context

	return GoTGProtoClient.Idle()
}
