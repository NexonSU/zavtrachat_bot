package main

import (
	"crypto/rand"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// The DSN must be supplied explicitly by the disposable CI service. Each test
// gets a private schema and a pool whose search_path cannot reach bot tables.
func isolatedPostgresDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("ZAVTRA_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set ZAVTRA_TEST_POSTGRES_DSN for disposable PostgreSQL tests")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := admin.DB()
	t.Cleanup(func() { pool.Close() })
	name := "remaining_test_" + strings.ToLower(rand.Text())
	if err := admin.Exec(fmt.Sprintf("CREATE SCHEMA %q", name)).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec(fmt.Sprintf("DROP SCHEMA %q CASCADE", name)).Error; err != nil {
			t.Error(err)
		}
	})
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		q := u.Query()
		q.Set("search_path", name)
		u.RawQuery = q.Encode()
		return u.String()
	}
	return dsn + " search_path=" + name
}

func closeTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })
}

func TestFullPostgresMigrationAndReconnect(t *testing.T) {
	setupTestEnv(t)
	dsn := isolatedPostgresDSN(t)
	// Model an older deployment, before caption/title, kills and mute history.
	legacy, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	closeTestDB(t, legacy)
	for _, sql := range []string{
		`CREATE TABLE gets (name text PRIMARY KEY, type text, data text, creator bigint)`,
		`INSERT INTO gets VALUES ('legacy','Text','<b>old &amp; safe</b>',42)`,
		`CREATE TABLE duelists (user_id bigint PRIMARY KEY, deaths bigint)`,
		`INSERT INTO duelists VALUES (42,7)`,
	} {
		if err := legacy.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	db, err := DataBaseInit(dsn)
	if err != nil {
		t.Fatal(err)
	}
	closeTestDB(t, &db)
	models := []any{&gotgbot.User{}, &Get{}, &Warn{}, &PidorStats{}, &PidorList{}, &Duelist{}, &Bless{}, &Salvation{}, &Nope{}, &Stats{}, &StatsWords{}, &Bets{}, &MuteEvent{}}
	for _, model := range models {
		if !db.Migrator().HasTable(model) {
			t.Fatalf("missing table %T", model)
		}
	}
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	update := int64(9001)
	seeds := []any{
		&Warn{UserID: 42, Amount: 2, LastWarn: now},
		&Bets{UserID: 42, Text: "existing bet", Timestamp: 123},
		&PidorList{Id: 42, FirstName: "Игрок"}, &PidorStats{Date: now, UserID: 42},
		&Stats{ContextID: 42, StatType: 8, Count: 7, DayTimestamp: 123},
		&StatsWords{Word: "кошка", ShortWord: "кошк"},
		&Bless{Text: "old phrase"}, &Salvation{Text: "old salvation"}, &Nope{Text: "old denial"},
		&MuteEvent{UserID: 42, ChatID: -100, Source: "warn", Muted: true, UntilDate: 123, TelegramUpdateID: &update},
	}
	for _, row := range seeds {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	// Close the migrated connection: the next init opens a new pool and migrates again.
	pool, _ := db.DB()
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := DataBaseInit(dsn)
	if err != nil {
		t.Fatal(err)
	}
	closeTestDB(t, &reopened)
	var get Get
	if err := reopened.First(&get, "name = ?", "legacy").Error; err != nil || get.Data != "<b>old &amp; safe</b>" || get.Creator != 42 {
		t.Fatalf("legacy get lost: %+v %v", get, err)
	}
	var duelist Duelist
	if err := reopened.First(&duelist, 42).Error; err != nil || duelist.Deaths != 7 || duelist.Kills != 0 {
		t.Fatalf("legacy counters changed: %+v %v", duelist, err)
	}
	var warn Warn
	if err := reopened.First(&warn, 42).Error; err != nil || warn.Amount != 2 || !warn.LastWarn.Equal(now) {
		t.Fatalf("warning changed: %+v %v", warn, err)
	}
	for _, row := range seeds {
		var count int64
		if err := reopened.Model(row).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("migration changed %T: %d %v", row, count, err)
		}
	}
	// Existing unique update index must still prevent duplicate mute events.
	if err := reopened.Create(&MuteEvent{TelegramUpdateID: &update}).Error; err == nil {
		t.Fatal("mute update uniqueness lost")
	}
}

func TestPostgresConcurrentGameCommands(t *testing.T) {
	bot, client := setupTestEnv(t)
	dsn := isolatedPostgresDSN(t)
	db, err := DataBaseInit(dsn)
	if err != nil {
		t.Fatal(err)
	}
	closeTestDB(t, &db)
	DB = db
	DB.Create(&Bless{Text: "да"})
	var wg sync.WaitGroup
	errors := make(chan error, 24)
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				errors <- Kill(bot, testReplyContext(bot, "/kill", testUser(100), testMessage("target", testUser(42))))
			} else {
				errors <- blessingWithSalvationRoll(bot, testContext(bot, "/blessing", testUser(42)), func() int { return 99 })
			}
		}(i)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Error(err)
		}
	}
	var row Duelist
	if err := DB.First(&row, 42).Error; err != nil || row.Deaths != 24 || statTotal(t, 8, 42) != 24 {
		t.Fatalf("lost game deaths: %+v %v", row, err)
	}
	if len(client.callsFor("restrictChatMember")) != 24 {
		t.Fatal("lost/repeated restriction")
	}
	if err := DB.Create(&Warn{UserID: 42, Amount: 1, LastWarn: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	warnErrors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			warnErrors <- WarnUser(bot, testReplyContext(bot, "/warn", testUser(100), testMessage("target", testUser(42))))
		}()
	}
	wg.Wait()
	close(warnErrors)
	for err := range warnErrors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var warn Warn
	DB.First(&warn, 42)
	if warn.Amount != 3 || len(client.callsFor("restrictChatMember")) != 25 {
		t.Fatalf("parallel PostgreSQL warning threshold: %+v", warn)
	}

}

func TestFullPostgresFreshSchema(t *testing.T) {
	setupTestEnv(t)
	dsn := isolatedPostgresDSN(t)
	db, err := DataBaseInit(dsn)
	if err != nil {
		t.Fatal(err)
	}
	closeTestDB(t, &db)
	for _, model := range []any{&gotgbot.User{}, &Get{}, &Warn{}, &PidorStats{}, &PidorList{}, &Duelist{}, &Bless{}, &Salvation{}, &Nope{}, &Stats{}, &StatsWords{}, &Bets{}, &MuteEvent{}} {
		if !db.Migrator().HasTable(model) {
			t.Fatalf("missing table %T", model)
		}
	}
	DB = db
	if _, err := recordGameDeath(42); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Get{Name: "new", Type: "Document", Data: "file", Caption: "<b>full caption</b>"}).Error; err != nil {
		t.Fatal(err)
	}
	reopened, err := DataBaseInit(dsn)
	if err != nil {
		t.Fatal(err)
	}
	closeTestDB(t, &reopened)
	var row Duelist
	if err := reopened.First(&row, 42).Error; err != nil || row.Deaths != 1 {
		t.Fatalf("fresh counter lost: %+v %v", row, err)
	}
	var get Get
	if err := reopened.First(&get, "name = ?", "new").Error; err != nil || get.Caption != "<b>full caption</b>" {
		t.Fatalf("fresh get lost: %+v %v", get, err)
	}
}

func TestPostgresMigrationFailureClosesPool(t *testing.T) {
	setupTestEnv(t)
	dsn := isolatedPostgresDSN(t)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	closeTestDB(t, db)
	if err := db.Exec(`CREATE VIEW gets AS SELECT 'broken'::text AS name`).Error; err != nil {
		t.Fatal(err)
	}
	failed, err := DataBaseInit(dsn)
	if err == nil {
		t.Fatal("expected migration error from conflicting view")
	}
	pool, poolErr := failed.DB()
	if poolErr != nil {
		t.Fatal(poolErr)
	}
	if err := pool.Ping(); err == nil {
		t.Fatal("migration failure leaked an open database pool")
	}
}
