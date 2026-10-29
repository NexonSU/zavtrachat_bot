package main

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"math/rand"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func newBotDispatcher() *ext.Dispatcher {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		// If an error is returned by a handler, log it and continue going.
		Error: func(bot *gotgbot.Bot, context *ext.Context, err error) ext.DispatcherAction {
			details := updateContext(context)
			safeError := err.Error()
			if Config.Token != "" {
				safeError = strings.ReplaceAll(safeError, Config.Token, "TOKEN")
			}
			text := "Ошибка: " + html.EscapeString(safeError) + "\n\n" + details
			var reportErr error
			var privateQuote *quotePrivateError
			if errors.As(err, &privateQuote) {
				reportErr = sendZBHelp(bot, context, text)
			} else if context.Message != nil {
				reportErr = Reply(text, *context)
			} else {
				reportErr = sendZBHelp(bot, context, text)
			}
			if reportErr != nil {
				Logger.Error("error when reporting a... error: " + reportErr.Error())
			}
			Logger.Error("an error occurred while handling update: " + err.Error() + " | " + strings.ReplaceAll(details, "\n", " "))
			return ext.DispatcherActionNoop
		},
		Logger:      Logger,
		MaxRoutines: -1,
	})
	dispatcher.AddHandlerToGroup(NewGlobalMiddleware(), -1)
	return dispatcher
}

func BotInit() error {
	// In gotgbot rc.36 BotOpts.RequestOpts only applies to the initial GetMe.
	// To make ALL API calls use the custom API URL, it must be set as the
	// DefaultRequestOpts on the bot client.
	reqOpts := &gotgbot.RequestOpts{
		Timeout: time.Second * 30,
		APIURL:  Config.BotApiUrl,
	}
	bot, err := gotgbot.NewBot(Config.Token, &gotgbot.BotOpts{
		DisableTokenCheck: false,
		BotClient: &gotgbot.BaseBotClient{
			DefaultRequestOpts: reqOpts,
		},
		RequestOpts: reqOpts,
	})
	if err != nil {
		return err
	}
	// Create updater and dispatcher.
	dispatcher := newBotDispatcher()
	updater := ext.NewUpdater(dispatcher, &ext.UpdaterOpts{
		UnhandledErrFunc: ErrorReporting,
	})

	// The globals go in before polling or the webhook server start: the updater
	// reports its failures through ErrorReporting, which answers on the global
	// Bot. A first request that fails (proxy down, 401) used to dereference the
	// nil global in a goroutine and take the whole process down.
	Bot = bot
	BotDispatcher = dispatcher
	BotUpdater = updater

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
		// The report is a notice, not a startup requirement: a chat that
		// refuses it (the bot is not an admin there) must not take the running
		// bot down.
		if err := SysAdminReply(startupReport(bot, connectionType)); err != nil {
			Logger.Warn("startup report failed: " + err.Error())
		}
	}

	return nil
}

// ErrorReporting is the last resort for errors the dispatcher cannot answer:
// the sysadmin of ZAVTRA_SYSADMIN gets the text with the token removed as an
// ephemeral message in the main chat. Without a sysadmin, or before the bot
// exists, it stays in the log.
func ErrorReporting(err error) {
	Logger.Error(err.Error())

	if Bot == nil || Config.SysAdmin == 0 {
		return
	}
	// The notice itself is not reported: SysAdminReply answers through the same
	// Bot, so a failure here would feed ErrorReporting its own error.
	SysAdminReply(strings.ReplaceAll(err.Error(), Config.Token, "TOKEN"))
}

// updateContext describes the update a handler failed on: the command text
// with its arguments, the sender and the chat, so an error report says what
// triggered it. The text is HTML escaped: reports go in HTML parse mode.
func updateContext(context *ext.Context) string {
	if context == nil {
		return "Update: неизвестно чего"
	}
	text := context.GetType()
	switch {
	case context.Message != nil:
		if context.Message.Text != "" {
			text = context.Message.Text
		} else {
			text = fmt.Sprintf("сообщение #%v без текста", context.Message.MessageId)
		}
	case context.CallbackQuery != nil:
		text = "callback: " + context.CallbackQuery.Data
	case context.InlineQuery != nil:
		text = "inline: " + context.InlineQuery.Query
	}
	details := fmt.Sprintf("Update: <code>%v</code>", html.EscapeString(text))
	if context.EffectiveUser != nil {
		details += fmt.Sprintf("\nЮзер: %v (%v)", MentionUser(context.EffectiveUser), context.EffectiveUser.Id)
	}
	if context.EffectiveChat != nil && context.EffectiveChat.Id != 0 {
		details += fmt.Sprintf("\nЧат: %v", context.EffectiveChat.Id)
	}
	return details
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

const antispamCleanupInterval = time.Minute
const antispamReportCooldown int64 = 10

type GlobalMiddleware struct {
	mu           sync.Mutex
	timestamps   map[int64][]time.Time
	lastReportAt map[int64]int64
	lastCleanup  time.Time
}

func NewGlobalMiddleware() *GlobalMiddleware {
	return &GlobalMiddleware{
		timestamps:   make(map[int64][]time.Time),
		lastReportAt: make(map[int64]int64),
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
		cntxBytes, err := MarshalOmitEmptyAll(cntx.MessageReaction)
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

	// Anonymous/channel messages must not reach user-only command handlers.
	if cntx.Message != nil && cntx.Message.From == nil {
		return ext.EndGroups
	}
	if cntx.EffectiveUser == nil {
		return nil
	}

	if m.recordAntispamUpdate(cntx.EffectiveUser.Id, time.Now()) {
		// Sending can wait on Telegram. The shared state and report cooldown
		// are already updated, so other updates never wait for this request.
		Logger.Warn("[SPAM] " + logText)
		SysAdminReply(fmt.Sprintf("Возможно спам.\nType: %s\nSender: %s\nChat: %s\nText: <code>%s</code>", cntx.GetType(), sender, chat, text))
	}
	return nil
}

func (m *GlobalMiddleware) recordAntispamUpdate(userID int64, now time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Prune lazily: no background goroutine is needed, and idle users do not
	// stay in these maps forever. Keep a recent report even when its message
	// window expired, since the report cooldown may outlast that window.
	if m.lastCleanup.IsZero() || now.Sub(m.lastCleanup) >= antispamCleanupInterval {
		for id, timestamps := range m.timestamps {
			active := false
			for _, timestamp := range timestamps {
				if int(now.Sub(timestamp).Seconds()) <= Config.AntiSpamWindow {
					active = true
					break
				}
			}
			if !active {
				delete(m.timestamps, id)
			}
		}
		for id, reportedAt := range m.lastReportAt {
			if now.Unix()-reportedAt > antispamReportCooldown {
				delete(m.lastReportAt, id)
			}
		}
		m.lastCleanup = now
	}

	var validTimestamps []time.Time
	for _, t := range m.timestamps[userID] {
		if int(now.Sub(t).Seconds()) <= Config.AntiSpamWindow {
			validTimestamps = append(validTimestamps, t)
		}
	}

	report := len(validTimestamps) >= Config.AntiSpamCount && now.Unix()-m.lastReportAt[userID] > antispamReportCooldown
	if report {
		m.lastReportAt[userID] = now.Unix()
	}

	validTimestamps = append(validTimestamps, now)
	m.timestamps[userID] = validTimestamps

	return report
}
