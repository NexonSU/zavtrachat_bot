package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRemindTimeParsing(t *testing.T) {
	now := time.Date(2026, 12, 31, 22, 30, 0, 0, time.UTC) // Jan 1, 01:30 MSK.
	for _, tc := range []struct {
		input string
		want  string
		used  int
	}{
		{"2ч text", "01.01.2027 03:30", 1},
		{"30м text", "01.01.2027 02:00", 1},
		{"1д", "02.01.2027 01:30", 1},
		{"2H", "01.01.2027 03:30", 1},
		{"366d", "02.01.2028 01:30", 1},
		{"сегодня 19:00 text", "01.01.2027 19:00", 2},
		{"завтра 19:00 text", "02.01.2027 19:00", 2},
		{"02.01.2027 19:00 text", "02.01.2027 19:00", 2},
	} {
		due, used, err := parseRemindTime(strings.Fields(tc.input), now)
		if err != nil || used != tc.used || due.In(betMoscow).Format("02.01.2006 15:04") != tc.want {
			t.Fatalf("%s: %v %d %v", tc.input, due, used, err)
		}
	}
	for _, input := range []string{"", "0м", "-1ч", "1.5ч", "99999999999999999999999999ч", "367д", "213504д", "8785h", "сегодня 01:30", "31.02.2027 19:00", "завтра 25:00", "завтра", "yesterday 19:00"} {
		if _, _, err := parseRemindTime(strings.Fields(input), now); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}

func TestRemindRegisteredMenuCommand(t *testing.T) {
	group, private, admin := commandMenus(commandListConfig())
	for _, menu := range [][]gotgbot.BotCommand{group, private, admin} {
		if !slices.ContainsFunc(menu, func(c gotgbot.BotCommand) bool { return c.Command == "remind" && c.IsEphemeral }) {
			t.Fatal("/remind is missing from a menu")
		}
	}
	for _, userID := range []int64{42, 100} {
		bot, client := setupTestEnv(t)
		Config.Chat = -1001234567890
		Config.AntiSpamCount = 100
		d := newBotDispatcher()
		registerBotHandlers(d)
		msg := testMessage("/remind@testbot 2ч", testUser(userID))
		msg.Chat = gotgbot.Chat{Id: Config.Chat, Type: "supergroup"}
		msg.MessageId = 0
		msg.EphemeralMessageId = 456
		msg.ReplyToMessage = &gotgbot.Message{MessageId: 777, Text: "<b>исходник & 😼</b>"}
		update := &gotgbot.Update{UpdateId: 1000, Message: msg}
		processTestUpdate(t, d, bot, update)
		processTestUpdate(t, d, bot, update)
		var rows []PersonalReminder
		if err := DB.Find(&rows).Error; err != nil || len(rows) != 1 {
			t.Fatalf("duplicate/missing row: %v %v", rows, err)
		}
		row := rows[0]
		if row.UserID != userID || row.Text != msg.ReplyToMessage.Text || row.DueAt != msg.Date+7200 || row.SourceURL != "https://t.me/c/1234567890/777" {
			t.Fatal(row)
		}
		pm := 0
		for _, call := range client.callsFor("sendMessage") {
			if call.Params["chat_id"] == userID {
				pm++
				if !strings.Contains(call.Params["text"].(string), "&lt;b&gt;исходник &amp; 😼&lt;/b&gt;") {
					t.Fatal(call)
				}
			} else {
				eph, _ := call.Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
				if eph == nil || eph.ReceiverUserId != userID {
					t.Fatal("public response", call)
				}
			}
		}
		if pm != 1 || len(client.callsFor("deleteMessage"))+len(client.callsFor("deleteEphemeralMessage")) != 0 {
			t.Fatal("duplicate confirmation or explicit deletion")
		}
	}
}

func TestRemindPrivateTextCaptionAndMedia(t *testing.T) {
	for _, tc := range []struct {
		command, caption, want string
	}{
		{"/remind завтра 19:00 свой текст", "подпись", "свой текст"},
		{"/remind 2ч", "подпись", "подпись"},
		{"/remind 2ч", "", "Посмотреть сообщение"},
	} {
		bot, client := setupTestEnv(t)
		now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
		msg := testMessage(tc.command, testUser(42))
		msg.Date = now.Unix()
		msg.Chat = gotgbot.Chat{Id: 42, Type: "private"}
		msg.ReplyToMessage = &gotgbot.Message{MessageId: 77, Caption: tc.caption}
		ctx := ext.NewContext(bot, &gotgbot.Update{Message: msg}, nil)
		if err := remindAt(bot, ctx, now); err != nil {
			t.Fatal(err)
		}
		var row PersonalReminder
		if err := DB.First(&row).Error; err != nil || row.Text != tc.want || row.SourceURL != "" || row.PrivateReplyID != 77 {
			t.Fatalf("%+v %v", row, err)
		}
		calls := client.callsFor("sendMessage")
		if len(calls) != 1 || calls[0].Params["chat_id"] != int64(42) || calls[0].Params["ephemeral_message_parameters"] != nil {
			t.Fatal(calls)
		}
		for _, call := range calls {
			reply, _ := call.Params["reply_parameters"].(*gotgbot.ReplyParameters)
			if reply == nil || reply.MessageId != 77 || !reply.AllowSendingWithoutReply {
				t.Fatal("private original is unreachable", call)
			}
		}
		client.clearCalls()
		if err := sendDuePersonalReminders(context.Background(), bot, &DB, now.Add(48*time.Hour)); err != nil {
			t.Fatal(err)
		}
		calls = client.callsFor("sendMessage")
		if len(calls) != 1 {
			t.Fatal(calls)
		}
		reply, _ := calls[0].Params["reply_parameters"].(*gotgbot.ReplyParameters)
		if reply == nil || reply.MessageId != 77 || !reply.AllowSendingWithoutReply {
			t.Fatal(calls)
		}
	}
}

func TestRemindValidationAndFailedConfirmation(t *testing.T) {
	bot, client := setupTestEnv(t)
	now := time.Now().Truncate(time.Second)
	for _, text := range []string{"/remind", "/remind 0ч x", "/remind 2ч", "/remind 1ч " + strings.Repeat("😼", 1751)} {
		ctx := testContext(bot, text, testUser(42))
		if err := remindAt(bot, ctx, now); err != nil {
			t.Fatal(err)
		}
	}
	ctx := testContext(bot, "/remind 1м too late", testUser(42))
	ctx.Message.Date = now.Add(-time.Hour).Unix()
	if err := remindAt(bot, ctx, now); err != nil {
		t.Fatal(err)
	}
	ctx = testContext(bot, "/remind 2ч text", testUser(42))
	client.setError("sendMessage", &gotgbot.TelegramError{Code: 403, Description: "blocked"})
	if err := remindAt(bot, ctx, now); err == nil {
		t.Fatal("the group response also fails in this fake")
	}
	var count int64
	if err := DB.Model(&PersonalReminder{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("invalid or unconfirmed reminders persisted: %d %v", count, err)
	}
	client.setError("sendMessage", nil)
	if err := remindAt(bot, ctx, now); err != nil {
		t.Fatal(err)
	}
	DB.Model(&PersonalReminder{}).Count(&count)
	if count != 1 {
		t.Fatal("cannot retry after unblocking")
	}
}

func TestRemindListCancelOwnership(t *testing.T) {
	bot, client := setupTestEnv(t)
	now := time.Now().Truncate(time.Second)
	row := PersonalReminder{RequestKey: "test", UserID: 42, Text: "<b>private & text</b>", DueAt: now.Add(time.Hour).Unix(), NextAttemptAt: now.Add(time.Hour).Unix()}
	if err := DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	for _, user := range []int64{43, 100} {
		if err := Remind(bot, testContext(bot, "/remind list", testUser(user))); err != nil {
			t.Fatal(err)
		}
		if err := Remind(bot, testContext(bot, fmt.Sprintf("/remind cancel %d", row.ID), testUser(user))); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Contains(strings.Join(client.sentTexts(), "\n"), "private") {
		t.Fatal("another user could see the reminder")
	}
	client.clearCalls()
	if err := Remind(bot, testContext(bot, "/remind list", testUser(42))); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(client.sentTexts()[0], "&lt;b&gt;private &amp; text&lt;/b&gt;") {
		t.Fatal(client.sentTexts())
	}
	if err := Remind(bot, testContext(bot, fmt.Sprintf("/remind cancel %d", row.ID), testUser(42))); err != nil {
		t.Fatal(err)
	}
	client.clearCalls()
	if err := sendDuePersonalReminders(context.Background(), bot, &DB, now.Add(2*time.Hour)); err != nil || len(client.sentTexts()) != 0 {
		t.Fatal("cancelled reminder was sent", err)
	}
}

func TestPersonalReminderDueRetryAndRestart(t *testing.T) {
	bot, client := setupTestEnv(t)
	now := time.Now().Truncate(time.Second)
	row := PersonalReminder{RequestKey: "due", UserID: 42, Text: "<b>text & 😼</b>", SourceURL: "https://t.me/c/1234567890/777", DueAt: now.Unix(), NextAttemptAt: now.Unix()}
	if err := DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := sendDuePersonalReminders(context.Background(), bot, &DB, now.Add(-time.Second)); err != nil || len(client.sentTexts()) != 0 {
		t.Fatal("early delivery", err)
	}
	client.setError("sendMessage", &gotgbot.TelegramError{Code: 429, Description: "retry", ResponseParams: &gotgbot.ResponseParameters{RetryAfter: 120}})
	if err := sendDuePersonalReminders(context.Background(), bot, &DB, now); err == nil {
		t.Fatal("expected delivery failure")
	}
	if err := DB.First(&row, row.ID).Error; err != nil || row.SentAt != 0 || row.NextAttemptAt != now.Unix()+120 {
		t.Fatalf("retry not persisted: %+v %v", row, err)
	}
	client.setError("sendMessage", nil)
	client.clearCalls()
	if err := sendDuePersonalReminders(context.Background(), bot, &DB, now.Add(time.Minute)); err != nil || len(client.sentTexts()) != 0 {
		t.Fatal("ignored retry-after", err)
	}
	// A new invocation has no process-local state; catch up after downtime.
	if err := sendDuePersonalReminders(context.Background(), bot, &DB, now.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 || calls[0].Params["chat_id"] != int64(42) || calls[0].Params["ephemeral_message_parameters"] != nil || !strings.Contains(calls[0].Params["text"].(string), "&lt;b&gt;text &amp; 😼&lt;/b&gt;") || !strings.Contains(calls[0].Params["text"].(string), row.SourceURL) {
		t.Fatal(calls)
	}
	if err := sendDuePersonalReminders(context.Background(), bot, &DB, now.Add(25*time.Hour)); err != nil || len(client.sentTexts()) != 1 {
		t.Fatal("repeated after restart", err)
	}
}

func TestPersonalReminderPermanentFailureAndShutdown(t *testing.T) {
	bot, client := setupTestEnv(t)
	now := time.Now().Truncate(time.Second)
	row := PersonalReminder{RequestKey: "blocked", UserID: 42, DueAt: now.Unix(), NextAttemptAt: now.Unix()}
	if err := DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sendDuePersonalReminders(ctx, bot, &DB, now); !errors.Is(err, context.Canceled) || len(client.sentTexts()) != 0 {
		t.Fatal("ignored shutdown", err)
	}
	runPersonalReminders(ctx, bot, &DB)
	client.setError("sendMessage", &gotgbot.TelegramError{Code: 403, Description: "secret text and token"})
	err := sendDuePersonalReminders(context.Background(), bot, &DB, now)
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal("missing/sensitive failure report", err)
	}
	if err := DB.First(&row, row.ID).Error; err != nil || row.FailedAt != now.Unix() {
		t.Fatalf("permanent failure not saved: %+v %v", row, err)
	}
	client.clearCalls()
	if err := sendDuePersonalReminders(context.Background(), bot, &DB, now.Add(time.Hour)); err != nil || len(client.sentTexts()) != 0 {
		t.Fatal("retrying a permanently blocked user", err)
	}
}

func TestRemindSourceLinks(t *testing.T) {
	for _, tc := range []struct {
		chat gotgbot.Chat
		want string
	}{
		{gotgbot.Chat{Id: -1001234567890, Type: "supergroup"}, "https://t.me/c/1234567890/77"},
		{gotgbot.Chat{Id: -123, Type: "group"}, ""},
		{gotgbot.Chat{Id: 42, Type: "private", Username: "person"}, ""},
	} {
		if got := remindSourceURL(tc.chat, 77); got != tc.want {
			t.Fatalf("%+v: %s", tc.chat, got)
		}
		if got := remindSourceURL(tc.chat, 0); got != "" {
			t.Fatal("linked an ephemeral command")
		}
	}
}

func TestPersonalRemindersPostgresConcurrentDelivery(t *testing.T) {
	bot, client := setupTestEnv(t)
	dsn := isolatedPostgresDSN(t)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	closeTestDB(t, db)
	for range 2 {
		if err := db.AutoMigrate(&PersonalReminder{}); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().Truncate(time.Second)
	row := PersonalReminder{RequestKey: "concurrent", UserID: 42, Text: "one send", DueAt: now.Unix(), NextAttemptAt: now.Unix()}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	client.Observe = func(method string, _ map[string]any) {
		if method == "sendMessage" {
			close(entered)
			<-release
		}
	}
	first := make(chan error, 1)
	go func() { first <- sendDuePersonalReminders(context.Background(), bot, db, now) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("first worker did not start")
	}
	second := make(chan error, 1)
	go func() { second <- sendDuePersonalReminders(context.Background(), bot, db, now) }()
	select {
	case err := <-second:
		if err != nil {
			close(release)
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		close(release)
		<-first
		t.Fatal("second worker did not skip locked reminder")
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	// Reconnect, as on restart, and verify both persistence and the unique key.
	reopened, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	closeTestDB(t, reopened)
	if err := sendDuePersonalReminders(context.Background(), bot, reopened, now.Add(time.Hour)); err != nil || len(client.sentTexts()) != 1 {
		t.Fatal("repeated after reconnect", err)
	}
	row.ID = 0
	if err := reopened.Create(&row).Error; err == nil {
		t.Fatal("duplicate request key accepted")
	}
}
