package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// The CI PostgreSQL service is disposable; an isolated schema ensures this
// migration/persistence check never touches the bot's production tables.
func TestMutedPostgresPersistence(t *testing.T) {
	dsn := os.Getenv("ZAVTRA_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set ZAVTRA_TEST_POSTGRES_DSN to run the PostgreSQL integration test")
	}
	setupTestEnv(t)
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	adminPool, _ := admin.DB()
	t.Cleanup(func() { adminPool.Close() })
	name := "muted_test_" + strings.ToLower(rand.Text())
	if err := admin.Exec(fmt.Sprintf("CREATE SCHEMA %q", name)).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec(fmt.Sprintf("DROP SCHEMA %q CASCADE", name)).Error; err != nil {
			t.Error(err)
		}
	})
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard, NamingStrategy: schema.NamingStrategy{TablePrefix: name + "."}})
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	t.Cleanup(func() { pool.Close() })
	DB = *db
	if err := db.AutoMigrate(&MuteEvent{}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			updateID := int64(9001)
			saveMuteEvent(&MuteEvent{ChatID: Config.Chat, UserID: 42, Muted: true, CompletedAt: 100, TelegramUpdateID: &updateID})
			saveMuteEvent(&MuteEvent{ChatID: Config.Chat, UserID: int64(1000 + i), Muted: true, Source: "blessing", CompletedAt: 100})
		}(i)
	}
	wg.Wait()
	var count int64
	if err := db.Model(&MuteEvent{}).Count(&count).Error; err != nil || count != 17 {
		t.Fatalf("nullable unique update ID / concurrent inserts: rows %d, err %v", count, err)
	}
	saveMuteEvent(&MuteEvent{ChatID: Config.Chat, UserID: 42, Muted: false, CompletedAt: 200})
	saveMuteEvent(&MuteEvent{ChatID: Config.Chat, UserID: 42, Muted: true, CompletedAt: 50})
	saveMuteEvent(&MuteEvent{ChatID: -999, UserID: 42, Muted: true, CompletedAt: 300})
	events, err := latestMuteEvents(Config.Chat)
	if err != nil || len(events) != 17 || events[0].UserID != 42 || events[0].Muted {
		t.Fatalf("persisted current events: %+v, err %v", events, err)
	}
}
