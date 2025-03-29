package main

import (
	tdctx "context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/gotd/contrib/bg"
	"github.com/gotd/td/telegram"
	tele "gopkg.in/telebot.v3"
)

var Bot tele.Bot
var GotdClient *telegram.Client
var GotdContext tdctx.Context

// BotInit initializes Telegram Bots
// Moved from auto init to manual init to make the code in utils package testable
func BotInit() {
	if Config.Token == "" {
		log.Fatal("Telegram bot token not found in config.json")
	}
	if Config.Chat == 0 {
		log.Fatal("Chat username not found in config.json")
	}
	Settings := tele.Settings{
		URL:       Config.BotApiUrl,
		Token:     Config.Token,
		ParseMode: tele.ModeHTML,
		OnError:   ErrorReporting,
		Poller: &tele.LongPoller{
			Timeout:        10 * time.Second,
			AllowedUpdates: Config.AllowedUpdates,
		},
	}
	if Config.EndpointPublicURL != "" || Config.Listen != "" {
		Settings.Poller = &tele.Webhook{
			Listen: Config.Listen,
			Endpoint: &tele.WebhookEndpoint{
				PublicURL: Config.EndpointPublicURL,
			},
			MaxConnections: Config.MaxConnections,
			AllowedUpdates: Config.AllowedUpdates,
		}
	} else {
		Settings.Poller = &tele.LongPoller{
			Timeout:        10 * time.Second,
			AllowedUpdates: Config.AllowedUpdates,
		}
	}
	var bot, err = tele.NewBot(Settings)
	if err != nil {
		log.Println(Config.BotApiUrl)
		log.Fatal(err)
	}
	if Config.SysAdmin != 0 {
		bot.Send(tele.ChatID(Config.SysAdmin), fmt.Sprintf("%v has finished starting up.", MentionUser(bot.Me)))
	}

	Bot = *bot

	go gotdClientInit()
}

func gotdClientInit() error {
	if Config.AppID == 0 || Config.AppHash == "" {
		return nil
	}
	client := telegram.NewClient(Config.AppID, Config.AppHash, telegram.Options{})
	return client.Run(tdctx.Background(), func(ctx tdctx.Context) error {
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

func ErrorReporting(err error, context tele.Context) {
	_, fn, line, _ := runtime.Caller(1)
	log.Printf("[%s:%d] %v", fn, line, err)
	if context != nil && context.Message() != nil && context.Chat().ID == Config.Chat {
		ReplyAndRemove(fmt.Sprintf("Ошибка: <code>%v</code>", err.Error()), context)
	}
	text := fmt.Sprintf("<pre>[%s:%d]\n%v</pre>", fn, line, strings.ReplaceAll(err.Error(), Config.Token, ""))
	if strings.Contains(err.Error(), "specified new message content and reply markup are exactly the same") {
		return
	}
	if strings.Contains(err.Error(), "message to delete not found") {
		return
	}
	if strings.Contains(err.Error(), "context does not contain message") {
		return
	}
	marshalledContext, _ := json.MarshalIndent(context.Update(), "", "    ")
	marshalledContextWithoutNil := regexp.MustCompile(`.*": (null|""|0|false)(,|)\n`).ReplaceAllString(string(marshalledContext), "")
	jsonMessage := html.EscapeString(marshalledContextWithoutNil)
	text += fmt.Sprintf("\n\nMessage:\n<pre>%v</pre>", jsonMessage)
	Bot.Send(tele.ChatID(Config.SysAdmin), text)
}
