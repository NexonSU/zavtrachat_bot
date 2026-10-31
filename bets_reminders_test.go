package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestBetStoresMessageTimeAndKeepsOriginalOnDuplicate(t *testing.T) {
	bot, client := setupTestEnv(t)
	// The host's date is still October 31; Moscow is already November 1.
	now := time.Date(2026, 10, 31, 22, 30, 0, 0, time.UTC)
	ctx := testContext(bot, "/bet 01.11.2026 too late", testUser(42))
	if err := betAt(bot, ctx, now); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(client.sentTexts()[0], "02.11.2026") {
		t.Fatal(client.sentTexts())
	}
	ctx = testContext(bot, "/bet 02.11.2026 <b>test & prediction</b>", testUser(42))
	ctx.Message.Date = now.Add(-time.Minute).Unix()
	if err := betAt(bot, ctx, now); err != nil {
		t.Fatal(err)
	}
	var row Bets
	if err := DB.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.CreatedTimestamp != ctx.Message.Date || row.AuthorName != "@testpupkin" || row.ReminderSentAt != 0 {
		t.Fatalf("wrong persisted bet: %+v", row)
	}
	if time.Unix(row.Timestamp, 0).UTC().Format("02.01.2006") != "02.11.2026" {
		t.Fatal(row.Timestamp)
	}
	ctx.Message.Date += 30
	if err := betAt(bot, ctx, now); err != nil {
		t.Fatal(err)
	}
	var again Bets
	if err := DB.First(&again).Error; err != nil {
		t.Fatal(err)
	}
	if again.CreatedTimestamp != row.CreatedTimestamp {
		t.Fatal("duplicate changed the original creation time")
	}
	if got := client.sentTexts(); got[len(got)-1] != "Такая ставка уже добавлена" {
		t.Fatal(got)
	}
}

func TestBetRemindersNoonPublicMentionAndRestart(t *testing.T) {
	bot, client := setupTestEnv(t)
	seedUser(42, "new_name", "Новое имя")
	noon := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	bet := Bets{UserID: 42, Text: "<b>Игра</b> & кот", Timestamp: betCalendarDay(noon).Unix(), CreatedTimestamp: time.Date(2026, 10, 31, 22, 0, 0, 0, time.UTC).Unix(), AuthorName: "@old_name"}
	if err := DB.Create(&bet).Error; err != nil {
		t.Fatal(err)
	}
	if err := sendDueBetReminders(bot, &DB, Config.Chat, noon.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(client.sentTexts()) != 0 {
		t.Fatal("sent before noon")
	}
	if err := sendDueBetReminders(bot, &DB, Config.Chat, noon); err != nil {
		t.Fatal(err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatal(calls)
	}
	want := "01.11.2026 <a href=\"tg://user?id=42\">@new_name</a> сказал:\n<blockquote>&lt;b&gt;Игра&lt;/b&gt; &amp; кот</blockquote>\nНу что, сбылось?"
	if calls[0].Params["text"] != want || calls[0].Params["chat_id"] != Config.Chat {
		t.Fatal(calls[0])
	}
	if v, ok := calls[0].Params["ephemeral_message_parameters"]; ok && v != nil {
		t.Fatal("reminder is private", v)
	}
	var saved Bets
	if err := DB.First(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if saved.ReminderSentAt != noon.Unix() {
		t.Fatal(saved)
	}
	// There is no in-memory delivery state: a new invocation still skips it.
	if err := sendDueBetReminders(bot, &DB, Config.Chat, noon.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(client.sentTexts()) != 1 {
		t.Fatal("duplicate reminder")
	}
}

func TestBetRemindersCatchUpLegacyDatesAndFailure(t *testing.T) {
	bot, client := setupTestEnv(t)
	now := time.Date(2026, 11, 2, 20, 59, 0, 0, time.UTC) // 23:59 MSK
	day := betCalendarDay(now).Unix()
	for _, bet := range []Bets{
		{UserID: 42, Text: "today", Timestamp: day},
		{UserID: 43, Text: "yesterday", Timestamp: day - 86400},
		{UserID: 44, Text: "tomorrow", Timestamp: day + 86400},
		{UserID: 45, Text: "already sent", Timestamp: day, ReminderSentAt: now.Unix()},
	} {
		if err := DB.Create(&bet).Error; err != nil {
			t.Fatal(err)
		}
	}
	client.setError("sendMessage", errors.New("temporary failure"))
	if err := sendDueBetReminders(bot, &DB, Config.Chat, now); err == nil {
		t.Fatal("expected send error")
	}
	var row Bets
	if err := DB.Where("user_id = 42").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ReminderSentAt != 0 {
		t.Fatal("failed delivery marked sent")
	}
	client.setError("sendMessage", nil)
	client.clearCalls()
	if err := sendDueBetReminders(bot, &DB, Config.Chat, now); err != nil {
		t.Fatal(err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.HasPrefix(texts[0], "Ранее <a href=\"tg://user?id=42\">42</a>") || !strings.Contains(texts[0], "today") {
		t.Fatal(texts)
	}
	if err := sendDueBetReminders(bot, &DB, Config.Chat, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(client.sentTexts()) != 1 {
		t.Fatal("sent tomorrow's bet before noon")
	}
}

func TestBetDeletedBeforeNoonAndAuthorWithoutUsername(t *testing.T) {
	bot, client := setupTestEnv(t)
	now := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	bet := Bets{UserID: 42, Text: "deleted", Timestamp: betCalendarDay(now).Unix()}
	if err := DB.Create(&bet).Error; err != nil {
		t.Fatal(err)
	}
	if err := DelBet(bot, testContext(bot, "/delbet 02.11.2026 deleted", testUser(42))); err != nil {
		t.Fatal(err)
	}
	client.clearCalls()
	if err := sendDueBetReminders(bot, &DB, Config.Chat, now); err != nil {
		t.Fatal(err)
	}
	if len(client.sentTexts()) != 0 {
		t.Fatal("reminded about a deleted bet")
	}
	u := &gotgbot.User{Id: 43, FirstName: "Кот <&>"}
	text := betReminderText(Bets{UserID: u.Id, Text: "prediction"}, betUserLabel(u))
	if !strings.Contains(text, "tg://user?id=43\">Кот &lt;&amp;&gt;</a>") {
		t.Fatal(text)
	}
}

func TestBetReminderLegacyLongTextAndCancellation(t *testing.T) {
	text := strings.Repeat("😼<&>", 1600)
	parts := betReminderParts(text)
	if strings.Join(parts, "") != text {
		t.Fatal("lost legacy text")
	}
	for _, part := range parts {
		if betTextUnits(part) > 3500 {
			t.Fatal("oversized part")
		}
	}
	bot, _ := setupTestEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() { runBetReminders(ctx, bot, &DB, Config.Chat); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler failed to stop")
	}
}

func TestBetMultiplePredictionsAndCreationDateInList(t *testing.T) {
	bot, client := setupTestEnv(t)
	seedUser(42, "testpupkin", "Тест")
	created := time.Now().Add(-time.Hour)
	day := betCalendarDay(time.Now())
	for _, text := range []string{"first", "second"} {
		if err := DB.Create(&Bets{UserID: 42, Text: text, Timestamp: day.Unix(), CreatedTimestamp: created.Unix()}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := AllBets(bot, testContext(bot, "/allbets", testUser(42))); err != nil {
		t.Fatal(err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "first") || !strings.Contains(texts[0], "second") || !strings.Contains(texts[0], created.In(betMoscow).Format("02.01.2006 15:04")+" МСК") {
		t.Fatal(texts)
	}
	client.clearCalls()
	noon := time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, betMoscow)
	if err := sendDueBetReminders(bot, &DB, Config.Chat, noon); err != nil {
		t.Fatal(err)
	}
	if len(client.sentTexts()) != 2 {
		t.Fatal("expected one reminder per prediction", client.sentTexts())
	}
}

func TestBetSQLiteLegacyMigration(t *testing.T) {
	setupTestEnv(t)
	if err := DB.Migrator().DropTable(&Bets{}); err != nil {
		t.Fatal(err)
	}
	if err := DB.Exec(`CREATE TABLE bets (user_id integer, text text, timestamp integer, PRIMARY KEY (user_id, text, timestamp))`).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Exec(`INSERT INTO bets VALUES (42, 'old prediction', 1793577600)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.AutoMigrate(&Bets{}); err != nil {
		t.Fatal(err)
	}
	var bet Bets
	if err := DB.First(&bet).Error; err != nil {
		t.Fatal(err)
	}
	if bet.UserID != 42 || bet.Text != "old prediction" || bet.Timestamp != 1793577600 || bet.CreatedTimestamp != 0 || bet.ReminderSentAt != 0 {
		t.Fatal(bet)
	}
}

func TestBetListSplitsLongPredictionsAndUsesSavedAuthor(t *testing.T) {
	bot, client := setupTestEnv(t)
	for _, text := range []string{strings.Repeat("x", 2200), strings.Repeat("y", 2200)} {
		if err := DB.Create(&Bets{UserID: 42, Text: text, Timestamp: betCalendarDay(time.Now()).Unix(), AuthorName: "@saved_author"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := AllBets(bot, testContext(bot, "/allbets", testUser(42))); err != nil {
		t.Fatal(err)
	}
	texts := client.sentTexts()
	if len(texts) != 2 {
		t.Fatal("long predictions should use separate messages", texts)
	}
	for _, text := range texts {
		if !strings.Contains(text, "@saved_author") || !strings.Contains(text, "Дата создания неизвестна") || len(text) > 4096 {
			t.Fatal(text)
		}
	}
}

func TestBetPostgresLegacyMigrationAndConcurrentDelivery(t *testing.T) {
	bot, client := setupTestEnv(t)
	dsn := isolatedPostgresDSN(t)
	legacy, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	closeTestDB(t, legacy)
	now := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	if err := legacy.Exec(`CREATE TABLE bets (user_id bigint, text text, timestamp bigint, PRIMARY KEY (user_id, text, timestamp))`).Error; err != nil {
		t.Fatal(err)
	}
	if err := legacy.Exec(`INSERT INTO bets VALUES (42, 'legacy', ?)`, betCalendarDay(now).Unix()).Error; err != nil {
		t.Fatal(err)
	}
	db, err := DataBaseInit(dsn)
	if err != nil {
		t.Fatal(err)
	}
	closeTestDB(t, &db)
	var bet Bets
	if err := db.First(&bet).Error; err != nil {
		t.Fatal(err)
	}
	if bet.CreatedTimestamp != 0 || bet.ReminderSentAt != 0 || bet.Text != "legacy" {
		t.Fatal(bet)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	client.Observe = func(method string, _ map[string]any) {
		if method == "sendMessage" {
			close(entered)
			<-release
		}
	}
	first := make(chan error, 1)
	go func() { first <- sendDueBetReminders(bot, &db, Config.Chat, now) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("first worker did not send")
	}
	second := make(chan error, 1)
	go func() { second <- sendDueBetReminders(bot, &db, Config.Chat, now) }()
	select {
	case err := <-second:
		if err != nil {
			close(release)
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		close(release)
		<-first
		t.Fatal("second worker did not skip locked bet")
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	client.Observe = nil
	reconnected, err := DataBaseInit(dsn)
	if err != nil {
		t.Fatal(err)
	}
	closeTestDB(t, &reconnected)
	if err := sendDueBetReminders(bot, &reconnected, Config.Chat, now); err != nil {
		t.Fatal(err)
	}
	if len(client.sentTexts()) != 1 {
		t.Fatal("duplicate after reconnect")
	}
}
