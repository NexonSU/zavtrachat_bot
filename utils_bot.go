package main

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func BotInit() error {
	bot, err := gotgbot.NewBot(Config.Token, &gotgbot.BotOpts{
		DisableTokenCheck: false,
		RequestOpts: &gotgbot.RequestOpts{
			Timeout: time.Second * 30,
			APIURL:  Config.BotApiUrl,
		},
	})
	if err != nil {
		return err
	}
	// Create updater and dispatcher.
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		// If an error is returned by a handler, log it and continue going.
		Error: func(bot *gotgbot.Bot, context *ext.Context, err error) ext.DispatcherAction {
			reportErr := Reply("Ошибка: "+strings.ReplaceAll(err.Error(), Config.Token, "TOKEN"), *context)
			if reportErr != nil {
				log.Println("error when reporting a... error: " + reportErr.Error())
			}
			log.Println("an error occurred while handling update:", err.Error())
			return ext.DispatcherActionNoop
		},
		Logger:      Logger,
		MaxRoutines: -1,
	})
	dispatcher.AddHandlerToGroup(NewGlobalMiddleware(5, 10*time.Second), -1)
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
			return err
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
			return err
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
			return err
		}
	}
	if Config.SysAdmin != 0 {
		ex, err := os.Executable()
		if err != nil {
			return err
		}
		exPath := filepath.Dir(ex)
		_, err = bot.SendMessage(Config.SysAdmin, fmt.Sprintf("<a href=\"tg://user?id=%v\">Bot</a> has finished starting up.\nConnection type: %v\nAPI Server: %v\nWorking directory: %v\nyt-dlp version: %v", bot.Id, connectionType, bot.GetAPIURL(nil), exPath, ytdlpGetVer()), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
		if err != nil {
			return err
		}
	}

	Bot = bot
	BotDispatcher = dispatcher
	BotUpdater = updater

	return nil
}

func ErrorReporting(err error) {
	// _, fn, line, _ := runtime.Caller(1)
	// log.Printf("[%s:%d] %v", fn, line, err)
	// if context != nil && context.Message != nil && context.Chat().Id == Config.Chat {
	// 	Reply(fmt.Sprintf("Ошибка: <code>%v</code>", err.Error()), *context)
	// }
	// text := fmt.Sprintf("<pre>[%s:%d]\n%v</pre>", fn, line, strings.ReplaceAll(err.Error(), Config.Token, ""))
	// if strings.Contains(err.Error(), "specified new message content and reply markup are exactly the same") {
	// 	return
	// }
	// if strings.Contains(err.Error(), "message to delete not found") {
	// 	return
	// }
	// if strings.Contains(err.Error(), "context does not contain message") {
	// 	return
	// }
	// marshalledContext, _ := json.MarshalIndent(context.Update(), "", "    ")
	// marshalledContextWithoutNil := regexp.MustCompile(`.*": (null|""|0|false)(,|)\n`).ReplaceAllString(string(marshalledContext), "")
	// jsonMessage := html.EscapeString(marshalledContextWithoutNil)
	// text += fmt.Sprintf("\n\nMessage:\n<pre>%v</pre>", jsonMessage)
	fmt.Println(err.Error())
	Bot.SendMessage(Config.SysAdmin, strings.ReplaceAll(err.Error(), Config.Token, "TOKEN"), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
}

func ytdlpGetVer() string {
	var stdout bytes.Buffer

	cmd := exec.Command("yt-dlp", "--version")
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "Unknown"
	}

	return strings.TrimSpace(stdout.String())
}

// Define your middleware struct
type GlobalMiddleware struct {
	mu           sync.Mutex
	timestamps   map[int64][]time.Time
	lastReportAt map[int64]int64
	MaxRequests  int
	WindowLength time.Duration
}

func NewGlobalMiddleware(maxRequests int, window time.Duration) *GlobalMiddleware {
	return &GlobalMiddleware{
		timestamps:   make(map[int64][]time.Time),
		lastReportAt: make(map[int64]int64),
		MaxRequests:  maxRequests,
		WindowLength: window,
	}
}

// Name gives the handler an identifier in the dispatcher
func (m *GlobalMiddleware) Name() string {
	return "global_middleware"
}

// CheckUpdate intercepts EVERYTHING by returning true
func (m *GlobalMiddleware) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	return true
}

func (m *GlobalMiddleware) HandleUpdate(b *gotgbot.Bot, cntx *ext.Context) error {
	logText := ""
	chat := ""
	sender := ""
	text := ""

	logText += fmt.Sprintf("Incoming update: %d", cntx.UpdateId)
	logText += fmt.Sprintf(" | Type: %s", cntx.GetType())

	if cntx.EffectiveChat != nil {
		chat = cntx.EffectiveChat.Title
		if cntx.EffectiveChat.FirstName != "" {
			chat = cntx.EffectiveChat.FirstName
			if cntx.EffectiveChat.LastName != "" {
				chat += " " + cntx.EffectiveChat.LastName
			}
		}
		if cntx.EffectiveChat.Username != "" {
			chat += " (" + cntx.EffectiveChat.Username + ")"
		}
	}
	if chat != "" {
		logText += fmt.Sprintf(" | Chat: %s", chat)
	}

	if cntx.EffectiveSender != nil {
		sender = fmt.Sprintf("%s (%d)", cntx.EffectiveSender.Name(), cntx.EffectiveSender.Id())
	}
	if sender != "" {
		logText += fmt.Sprintf(" | Sender: %s", sender)
	}

	switch {
	case cntx.Message != nil:
		text = cntx.Message.Text + cntx.Message.Caption
		logText += fmt.Sprintf(" | Text: %s", text)

	case cntx.EditedMessage != nil:
		text = cntx.EditedMessage.Text + cntx.EditedMessage.Caption
		logText += fmt.Sprintf(" | Text: %s", text)

	case cntx.ChannelPost != nil:
		text = cntx.ChannelPost.Text + cntx.ChannelPost.Caption
		logText += fmt.Sprintf(" | Text: %s", text)

	case cntx.EditedChannelPost != nil:
		text = cntx.EditedChannelPost.Text + cntx.EditedChannelPost.Caption
		logText += fmt.Sprintf(" | Text: %s", text)

	case cntx.BusinessConnection != nil:
		cntxBytes, err := MarshalOmitEmptyAll(cntx.BusinessConnection)
		if err == nil {
			text = string(cntxBytes)
			logText += fmt.Sprintf(" | Data: %s", text)
		}

	case cntx.BusinessMessage != nil:
		text = cntx.BusinessMessage.Text + cntx.BusinessMessage.Caption
		logText += fmt.Sprintf(" | Text: %s", text)

	case cntx.EditedBusinessMessage != nil:
		text = cntx.EditedBusinessMessage.Text + cntx.EditedBusinessMessage.Caption
		logText += fmt.Sprintf(" | Text: %s", text)

	case cntx.DeletedBusinessMessages != nil:
		cntxBytes, err := MarshalOmitEmptyAll(cntx.DeletedBusinessMessages)
		if err == nil {
			text = string(cntxBytes)
			logText += fmt.Sprintf(" | Data: %s", text)
		}

	case cntx.GuestMessage != nil:
		text = cntx.GuestMessage.Text + cntx.GuestMessage.Caption
		logText += fmt.Sprintf(" | Text: %s", text)

	case cntx.MessageReaction != nil:
		cntxBytes, err := MarshalOmitEmptyAll(cntx.DeletedBusinessMessages)
		if err == nil {
			text = string(cntxBytes)
			logText += fmt.Sprintf(" | Data: %s", text)
		}

	case cntx.MessageReactionCount != nil:
		cntxBytes, err := MarshalOmitEmptyAll(cntx.MessageReactionCount)
		if err == nil {
			text = string(cntxBytes)
			logText += fmt.Sprintf(" | Data: %s", text)
		}

	case cntx.InlineQuery != nil:
		text = cntx.InlineQuery.Query
		logText += fmt.Sprintf(" | Query: %s", text)

	case cntx.ChosenInlineResult != nil:
		text = cntx.ChosenInlineResult.ResultId
		logText += fmt.Sprintf(" | Chosen: %s", text)

	default:
		cntxBytes, err := MarshalOmitEmptyAll(cntx)
		if err == nil {
			text = string(cntxBytes)
			logText += fmt.Sprintf(" | Data: %s", text)
		}
	}

	Logger.Debug(logText)

	// 1. Skip checks if there is no physical user attached to the incoming update
	if cntx.EffectiveUser == nil {
		return nil
	}

	userID := cntx.EffectiveUser.Id
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	var validTimestamps []time.Time
	for _, t := range m.timestamps[userID] {
		if now.Sub(t) <= m.WindowLength {
			validTimestamps = append(validTimestamps, t)
		}
	}

	if len(validTimestamps) >= m.MaxRequests && now.Unix()-m.lastReportAt[userID] > 10 {
		m.lastReportAt[userID] = now.Unix()
		Logger.Warn("[SPAM] " + logText)
		Bot.SendMessage(Config.SysAdmin, fmt.Sprintf("Возможно спам.\nType: %s\nSender: %s\nChat: %s\nText: <code>%s</code>", cntx.GetType(), sender, chat, text), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
	}

	validTimestamps = append(validTimestamps, now)
	m.timestamps[userID] = validTimestamps

	return nil
}
