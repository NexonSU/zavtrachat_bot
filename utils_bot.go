package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/gotd/contrib/bg"
	"github.com/gotd/td/telegram"
	"github.com/lrstanley/go-ytdlp"
)

var Bot *gotgbot.Bot
var BotDispatcher *ext.Dispatcher
var BotUpdater *ext.Updater
var GotdClient *telegram.Client
var GotdContext context.Context

func init() {
	bot, err := gotgbot.NewBot(Config.Token, &gotgbot.BotOpts{
		BotClient: middlewareClient(),
		RequestOpts: &gotgbot.RequestOpts{
			APIURL: Config.BotApiUrl,
		},
	})
	if err != nil {
		panic("failed to create new bot: " + err.Error())
	}
	// Create updater and dispatcher.
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		// If an error is returned by a handler, log it and continue going.
		Error: func(bot *gotgbot.Bot, context *ext.Context, err error) ext.DispatcherAction {
			log.Println("an error occurred while handling update:", err.Error())
			ReplyAndRemove("Ошибка: "+strings.ReplaceAll(err.Error(), Config.Token, "TOKEN"), *context)
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})
	updater := ext.NewUpdater(dispatcher, &ext.UpdaterOpts{
		UnhandledErrFunc: ErrorReporting,
	})

	connectionType := ""
	if Config.EndpointPublicURL != "" || Config.Listen != "" {
		connectionType = "webhook"
		// Start the webhook server. We start the server before we set the webhook itself, so that when telegram starts
		// sending updates, the server is already ready.
		wsl := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
		wss := make([]rune, 26)
		for i := range wss {
			wss[i] = wsl[rand.Intn(len(wsl))]
		}
		webhookSecret := string(wss)

		// The bot's urlPath can be anything. Here, we use "custom-path/<TOKEN>" as an example.
		// It can be a good idea for the urlPath to contain the bot token, as that makes it very difficult for outside
		// parties to find the update endpoint (which would allow them to inject their own updates).
		err = updater.StartWebhook(bot, bot.Username, ext.WebhookOpts{
			ListenAddr:  Config.Listen,
			SecretToken: webhookSecret,
		})
		if err != nil {
			panic("failed to start webhook: " + err.Error())
		}

		err = updater.SetAllBotWebhooks(Config.EndpointPublicURL, &gotgbot.SetWebhookOpts{
			MaxConnections:     Config.MaxConnections,
			AllowedUpdates:     Config.AllowedUpdates,
			SecretToken:        webhookSecret,
			DropPendingUpdates: false,
			RequestOpts: &gotgbot.RequestOpts{
				APIURL: Config.BotApiUrl,
			},
		})
		if err != nil {
			panic("failed to set webhook: " + err.Error())
		}
	} else {
		connectionType = "polling"
		err = updater.StartPolling(bot, &ext.PollingOpts{
			DropPendingUpdates:    false,
			EnableWebhookDeletion: true,
			GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
				Timeout:        10,
				AllowedUpdates: Config.AllowedUpdates,
				RequestOpts: &gotgbot.RequestOpts{
					Timeout: time.Second * 30,
					APIURL:  Config.BotApiUrl,
				},
			},
		})
		if err != nil {
			panic("failed to start polling: " + err.Error())
		}
	}
	if Config.SysAdmin != 0 {
		ex, err := os.Executable()
		if err != nil {
			panic(err)
		}
		exPath := filepath.Dir(ex)
		_, err = bot.SendMessage(Config.SysAdmin, fmt.Sprintf("<a href=\"tg://user?id=%v\">Bot</a> has finished starting up.\nConnection type: %v\nAPI Server: %v\nWorking directory: %v\nyt-dlp version: %v", bot.Id, connectionType, bot.GetAPIURL(nil), exPath, ytdlp.Version), &gotgbot.SendMessageOpts{})
		if err != nil {
			panic("failed to send message: " + err.Error())
		}
	}

	Bot = bot
	BotDispatcher = dispatcher
	BotUpdater = updater

	go gotdClientInit()
}

func gotdClientInit() error {
	if Config.AppID == 0 || Config.AppHash == "" {
		return nil
	}
	client := telegram.NewClient(Config.AppID, Config.AppHash, telegram.Options{})
	return client.Run(context.Background(), func(ctx context.Context) error {
		stop, err := bg.Connect(client)
		if err != nil {
			return err
		}
		defer func() { _ = stop() }()

		_, err = client.Auth().Bot(ctx, Bot.Token)
		if err != nil {
			return err
		}

		GotdClient = client
		GotdContext = ctx

		for {
			time.Sleep(time.Second * time.Duration(60))
		}
	})
}

func ErrorReporting(err error) {
	// _, fn, line, _ := runtime.Caller(1)
	// log.Printf("[%s:%d] %v", fn, line, err)
	// if context != nil && context.Message != nil && context.Chat().Id == Config.Chat {
	// 	ReplyAndRemove(fmt.Sprintf("Ошибка: <code>%v</code>", err.Error()), *context)
	// }
	// text := fmt.Sprintf("<pre>[%s:%d]\n%v</pre>", fn, line, strings.ReplaceAll(err.Error(), Config.Token, ""))
	if strings.Contains(err.Error(), "specified new message content and reply markup are exactly the same") {
		return
	}
	if strings.Contains(err.Error(), "message to delete not found") {
		return
	}
	if strings.Contains(err.Error(), "context does not contain message") {
		return
	}
	// marshalledContext, _ := json.MarshalIndent(context.Update(), "", "    ")
	// marshalledContextWithoutNil := regexp.MustCompile(`.*": (null|""|0|false)(,|)\n`).ReplaceAllString(string(marshalledContext), "")
	// jsonMessage := html.EscapeString(marshalledContextWithoutNil)
	// text += fmt.Sprintf("\n\nMessage:\n<pre>%v</pre>", jsonMessage)
	fmt.Println(err.Error())
	Bot.SendMessage(Config.SysAdmin, err.Error(), nil)
}

type middlewareBotClient struct {
	gotgbot.BotClient
}

func (b middlewareBotClient) RequestWithContext(ctx context.Context, token string, method string, params map[string]string, data map[string]gotgbot.FileReader, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	params["parse_mode"] = "HTML"

	return b.BotClient.RequestWithContext(ctx, token, method, params, data, opts)
}

func middlewareClient() middlewareBotClient {
	return middlewareBotClient{
		BotClient: &gotgbot.BaseBotClient{
			Client:             http.Client{},
			UseTestEnvironment: false,
			DefaultRequestOpts: &gotgbot.RequestOpts{
				Timeout: gotgbot.DefaultTimeout,
				APIURL:  Config.BotApiUrl,
			},
		},
	}
}
