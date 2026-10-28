package main

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestUserCacheChangesAndConcurrentMessages(t *testing.T) {
	setupTestEnv(t)
	counter := &statsQueryCounter{Interface: logger.Discard}
	db := DB.Session(&gorm.Session{Logger: counter})
	user := *testUser(42)
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			if err := saveUser(db, user); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if n := counter.count.Load(); n != 1 {
		t.Fatalf("32 identical concurrent users produced %d queries, want 1", n)
	}
	// Changes include fields becoming empty/false and flags beyond the name.
	user.Username, user.LastName, user.LanguageCode, user.IsPremium = "newname", "", "ru", true
	if err := saveUser(db, user); err != nil {
		t.Fatal(err)
	}
	user.Username, user.LanguageCode, user.IsPremium = "", "", false
	if err := saveUser(db, user); err != nil {
		t.Fatal(err)
	}
	if n := counter.count.Load(); n != 3 {
		t.Fatalf("two changes produced %d total queries, want 3", n)
	}
	var saved gotgbot.User
	if err := DB.First(&saved, user.Id).Error; err != nil || saved != user {
		t.Fatalf("user changes were lost: %+v %v", saved, err)
	}
}

func TestUserCacheExpiryFailureAndDatabaseIsolation(t *testing.T) {
	setupTestEnv(t)
	cache, err := userWritesFor(&DB)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	user := *testUser(42)
	if err := cache.save(&DB, user, now); err != nil {
		t.Fatal(err)
	}
	if err := DB.Delete(&gotgbot.User{}, user.Id).Error; err != nil {
		t.Fatal(err)
	}
	// Expiry must be based on the last successful persistence, not the last hit.
	if err := cache.save(&DB, user, now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := cache.save(&DB, user, now.Add(userCacheTTL)); err != nil {
		t.Fatal(err)
	}
	var saved gotgbot.User
	if err := DB.First(&saved, user.Id).Error; err != nil || saved != user {
		t.Fatalf("expired cache did not repair missing database row: %+v %v", saved, err)
	}
	if err := DB.Migrator().DropTable(&gotgbot.User{}); err != nil {
		t.Fatal(err)
	}
	changed := user
	changed.FirstName = "Новое имя"
	if err := cache.save(&DB, changed, now.Add(6*time.Minute)); err == nil {
		t.Fatal("failed user write was swallowed")
	}
	if err := DB.AutoMigrate(&gotgbot.User{}); err != nil {
		t.Fatal(err)
	}
	if err := cache.save(&DB, changed, now.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	firstDB := DB
	// Another GORM session on the same pool shares the cache.
	if same, _ := userWritesFor(DB.Session(&gorm.Session{})); same != cache {
		t.Fatal("GORM session discarded user cache")
	}
	setupTestEnv(t)
	if err := saveUser(&DB, changed); err != nil {
		t.Fatal(err)
	}
	if err := DB.First(&saved, changed.Id).Error; err != nil || saved != changed {
		t.Fatalf("user from another database's cache was skipped: %+v %v", saved, err)
	}
	DB = firstDB
}

func TestUserCacheBoundsAndOversizedChanges(t *testing.T) {
	setupTestEnv(t)
	cache, _ := userWritesFor(&DB)
	now := time.Now()
	for id := int64(1); id <= userCacheSize+1; id++ {
		if err := cache.save(&DB, *testUser(id), now); err != nil {
			t.Fatal(err)
		}
	}
	if len(cache.users) != userCacheSize || len(cache.order) != userCacheSize {
		t.Fatal("user cache exceeded its bound")
	}
	if _, ok := cache.users[1]; ok {
		t.Fatal("oldest user was not evicted")
	}
	user := *testUser(42)
	oversized := user
	oversized.FirstName = strings.Repeat("x", userCacheMaxBytes+1)
	if err := cache.save(&DB, oversized, now); err != nil {
		t.Fatal(err)
	}
	if err := cache.save(&DB, user, now); err != nil {
		t.Fatal(err)
	}
	var saved gotgbot.User
	if err := DB.First(&saved, user.Id).Error; err != nil || saved != user {
		t.Fatalf("oversized change left an incorrect cached snapshot: %+v %v", saved, err)
	}
}

func TestUserCachePostgresConditionalUpdates(t *testing.T) {
	setupTestEnv(t)
	db, err := gorm.Open(postgres.Open(isolatedPostgresDSN(t)), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	closeTestDB(t, db)
	if err := db.AutoMigrate(&gotgbot.User{}); err != nil {
		t.Fatal(err)
	}
	cache, _ := userWritesFor(db)
	user := *testUser(42)
	now := time.Now()
	if err := cache.save(db, user, now); err != nil {
		t.Fatal(err)
	}
	version := func() string {
		var v string
		if err := db.Raw("SELECT xmin::text FROM users WHERE id = ?", user.Id).Scan(&v).Error; err != nil {
			t.Fatal(err)
		}
		return v
	}
	before := version()
	if err := cache.save(db, user, now.Add(userCacheTTL)); err != nil {
		t.Fatal(err)
	}
	if version() != before {
		t.Fatal("unchanged user caused a physical PostgreSQL UPDATE after expiry")
	}
	user.LastName, user.Username, user.IsPremium = "", "", true
	if err := cache.save(db, user, now.Add(userCacheTTL)); err != nil {
		t.Fatal(err)
	}
	var saved gotgbot.User
	if err := db.First(&saved, user.Id).Error; err != nil || saved != user || version() == before {
		t.Fatalf("changed PostgreSQL user was not updated: %+v %v", saved, err)
	}
}
