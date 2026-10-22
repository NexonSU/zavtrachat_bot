package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// CI supplies a disposable PostgreSQL service. A private random schema keeps
// all test rows isolated; production configuration and tables are never used.
func TestStatsPostgresConcurrent(t *testing.T) {
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
	name := "stats_test_" + strings.ToLower(rand.Text())
	if err := admin.Exec(fmt.Sprintf("CREATE SCHEMA %q", name)).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec(fmt.Sprintf("DROP SCHEMA %q CASCADE", name)).Error; err != nil {
			t.Error(err)
		}
	})
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:         logger.Discard,
		NamingStrategy: schema.NamingStrategy{TablePrefix: name + "."},
	})
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	pool.SetMaxOpenConns(16)
	t.Cleanup(func() { pool.Close() })
	DB = *db
	if err := db.AutoMigrate(&Stats{}, &StatsWords{}); err != nil {
		t.Fatal(err)
	}
	day := GetStartOfDay()
	before := time.Now().Unix()
	if err := db.Create(&Stats{ContextID: 42, StatType: 3, DayTimestamp: day - 86400, Count: 99}).Error; err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make(chan error, 32)
	ids := make(chan int64, 32)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			id, err := lookupStatsWord([]string{"кошки", "кошка"}[i%2])
			if err != nil {
				errs <- err
				return
			}
			ids <- id
			for j := 0; j < 20; j++ {
				for _, update := range []struct{ kind, context, amount int64 }{
					{3, 42, 1}, {7, 42, 17}, {4, id, 2},
				} {
					if err := statsAdd(update.kind, day, update.context, update.amount); err != nil {
						errs <- err
						return
					}
				}
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	close(ids)
	for err := range errs {
		t.Error(err)
	}
	var wordID int64
	for id := range ids {
		if id <= 0 || (wordID != 0 && wordID != id) {
			t.Fatalf("concurrent PostgreSQL dictionary IDs differ: %d, %d", wordID, id)
		}
		wordID = id
	}
	for _, tc := range []struct{ kind, context, day, want int64 }{
		{3, 42, day, 640}, {7, 42, day, 10880}, {4, wordID, day, 1280}, {3, 42, day - 86400, 99},
	} {
		var row Stats
		if err := db.Where("stat_type = ? AND context_id = ? AND day_timestamp = ?", tc.kind, tc.context, tc.day).First(&row).Error; err != nil {
			t.Fatal(err)
		}
		if row.Count != tc.want || (tc.day == day && row.LastUpdate < before) {
			t.Fatalf("PostgreSQL counter mismatch: %+v, want %d", row, tc.want)
		}
	}
	var n int64
	if err := db.Model(&StatsWords{}).Count(&n).Error; err != nil || n != 1 {
		t.Fatalf("concurrent word creation made duplicates: rows=%d, err=%v", n, err)
	}
}
