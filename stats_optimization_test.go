package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neonxp/StemmerRu"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type statsQueryCounter struct {
	logger.Interface
	count atomic.Int64
}

func TestMessageStatsPreservesCounts(t *testing.T) {
	bot, client := setupTestEnv(t)
	day := GetStartOfDay()
	for _, text := range []string{"Кот, кот! кошки кошка", "покупал покупал покупает", "/ping", "", "кот кот"} {
		if err := OnText(bot, testContext(bot, text, testUser(42))); err != nil {
			t.Fatal(err)
		}
	}
	var words []StatsWords
	if err := DB.Find(&words).Error; err != nil {
		t.Fatal(err)
	}
	want := map[string]int64{StemmerRu.Stem("кот"): 4, StemmerRu.Stem("кошки"): 2, StemmerRu.Stem("покупал"): 3}
	firstSpelling := map[string]string{StemmerRu.Stem("кот"): "кот", StemmerRu.Stem("кошки"): "кошки", StemmerRu.Stem("покупал"): "покупал"}
	if len(words) != len(want) {
		t.Fatalf("dictionary has %d rows, want %d", len(words), len(want))
	}
	for _, word := range words {
		var stat Stats
		if err := DB.Where("context_id = ? AND stat_type = 4 AND day_timestamp = ?", word.ID, day).First(&stat).Error; err != nil {
			t.Fatal(err)
		}
		if stat.Count != want[word.ShortWord] || word.Word != firstSpelling[word.ShortWord] {
			t.Fatalf("changed counts or representative word: %+v, %+v", word, stat)
		}
		if err := statsAdd(4, day-86400, word.ID, 20); err != nil {
			t.Fatal(err)
		}
	}
	var messages Stats
	DB.Where("context_id = 42 AND stat_type = 3 AND day_timestamp = ?", day).First(&messages)
	if messages.Count != 5 {
		t.Fatalf("message count = %d, want 5 including commands and non-text messages", messages.Count)
	}
	client.clearCalls()
	if err := StatsTop(bot, testContext(bot, "/stats words day", testUser(42))); err != nil {
		t.Fatal(err)
	}
	text := strings.Join(client.sentTexts(), "\n")
	if !strings.Contains(text, "кот - 4 раза") || !strings.Contains(text, "покупал - 3 раза") || !strings.Contains(text, "кошки - 2 раза") {
		t.Fatalf("daily top changed: %q", text)
	}
}

func TestStatsWordCacheIsolation(t *testing.T) {
	setupTestEnv(t)
	stem := StemmerRu.Stem("кот")
	DB.Create(&StatsWords{ID: 99, Word: "кот", ShortWord: stem})
	if id := getWordID("кот"); id != 99 {
		t.Fatalf("first database word ID = %d", id)
	}
	firstDB := DB
	firstCache, _ := statsWordsFor(&DB)
	DB = *DB.Session(&gorm.Session{Logger: logger.Discard})
	if same, _ := statsWordsFor(&DB); same != firstCache {
		t.Fatal("GORM sessions must reuse the same database cache")
	}
	setupTestEnv(t)
	DB.Create(&StatsWords{ID: 7, Word: "кот", ShortWord: stem})
	if id := getWordID("кот"); id != 7 {
		t.Fatalf("dictionary ID leaked from another database: %d", id)
	}
	DB = firstDB
	if id := getWordID("кот"); id != 99 {
		t.Fatalf("returning to the first database reused the wrong ID: %d", id)
	}
}

func TestStatsWordCacheBounded(t *testing.T) {
	cache := &statsWordCache{ids: make(map[string]int64)}
	for i := 0; i <= statsWordCacheSize; i++ {
		cache.put(fmt.Sprint(i), int64(i+1))
	}
	if len(cache.ids) != statsWordCacheSize || len(cache.order) != statsWordCacheSize {
		t.Fatal("word cache is not bounded")
	}
	if _, ok := cache.get("0"); ok {
		t.Fatal("oldest cache entry was not evicted")
	}
	if id, ok := cache.get(fmt.Sprint(statsWordCacheSize)); !ok || id != statsWordCacheSize+1 {
		t.Fatal("newest dictionary ID was not cached")
	}
	cache.put("invalid", 0)
	long := strings.Repeat("x", statsWordCacheMaxKeyBytes+1)
	cache.put(long, 10)
	if _, ok := cache.get("invalid"); ok {
		t.Fatal("invalid dictionary ID was cached")
	}
	if _, ok := cache.get(long); ok {
		t.Fatal("oversized word was retained in cache")
	}
}

func TestStatsWordsConcurrentColdLookup(t *testing.T) {
	setupTestEnv(t)
	pool, _ := DB.DB()
	pool.SetMaxOpenConns(1) // SQLite serializes writers; PostgreSQL is covered in CI.
	start := make(chan struct{})
	errs := make(chan error, 32)
	ids := make(chan int64, 32)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			word := []string{"кошки", "кошка"}[i%2]
			id, err := lookupStatsWord(word)
			if err == nil {
				err = statsAdd(4, 123, id, 1)
			}
			if err != nil {
				errs <- err
			}
			ids <- id
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
			t.Fatalf("concurrent lookups returned inconsistent IDs: %d, %d", wordID, id)
		}
		wordID = id
	}
	var n int64
	DB.Model(&StatsWords{}).Count(&n)
	var row Stats
	DB.Where("stat_type = 4").First(&row)
	if n != 1 || row.Count != 32 {
		t.Fatalf("dictionary rows = %d, occurrences = %d", n, row.Count)
	}
}

func TestStatsConcurrentNewCounter(t *testing.T) {
	setupTestEnv(t)
	pool, _ := DB.DB()
	pool.SetMaxOpenConns(1)
	const day = 1234
	DB.Create(&Stats{ContextID: 42, StatType: 8, DayTimestamp: day - 86400, Count: 99})
	start := make(chan struct{})
	errs := make(chan error, 32)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 20; j++ {
				if err := statsAdd(8, day, 42, 3); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	var rows []Stats
	DB.Order("day_timestamp").Find(&rows)
	if len(rows) != 2 || rows[0].Count != 99 || rows[1].Count != 32*20*3 || rows[1].LastUpdate < time.Now().Add(-time.Minute).Unix() {
		t.Fatalf("concurrent counts or historical row changed: %+v", rows)
	}
}

func TestStatsCounterZeroKeysAndAmounts(t *testing.T) {
	setupTestEnv(t)
	for _, update := range []struct{ kind, day, context, amount int64 }{
		{2, 123, 0, 5}, {2, 123, 0, -2}, {2, 123, 0, 0},
		{5, 123, 0, 7}, {2, 124, 0, 11},
	} {
		if err := statsAdd(update.kind, update.day, update.context, update.amount); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ kind, day, want int64 }{{2, 123, 3}, {5, 123, 7}, {2, 124, 11}} {
		var row Stats
		if err := DB.Where("context_id = 0 AND stat_type = ? AND day_timestamp = ?", tc.kind, tc.day).First(&row).Error; err != nil || row.Count != tc.want {
			t.Fatalf("midnight/Sunday or zero/negative amount changed: %+v, %v", row, err)
		}
	}
}

func TestStatsDatabaseFailureAndRecovery(t *testing.T) {
	bot, _ := setupTestEnv(t)
	DB.Migrator().DropTable(&StatsWords{})
	if id, err := lookupStatsWord("новое"); err == nil || id != 0 {
		t.Fatal("dictionary error was swallowed")
	}
	if err := recordMessageWords("новое", 123); err == nil {
		t.Fatal("failed lookup must not create a counter for ID zero")
	}
	var n int64
	DB.Model(&Stats{}).Count(&n)
	if n != 0 {
		t.Fatal("failed word lookup wrote statistics")
	}
	if err := DB.AutoMigrate(&StatsWords{}); err != nil {
		t.Fatal(err)
	}
	if id, err := lookupStatsWord("новое"); err != nil || id <= 0 {
		t.Fatalf("failed lookup was cached instead of retried: %d, %v", id, err)
	}
	DB.Migrator().DropTable(&Stats{})
	if err := statsAdd(7, 123, 42, 60); err == nil {
		t.Fatal("counter error was swallowed")
	}
	if err := OnText(bot, testContext(bot, "новое", testUser(42))); err == nil {
		t.Fatal("message processing swallowed a failed counter update")
	}
}

func (c *statsQueryCounter) Trace(_ context.Context, _ time.Time, _ func() (string, int64), _ error) {
	c.count.Add(1)
}

// Count actual SQL statements in OnText after the dictionary and daily
// counters have been populated. The original handler issues 25 in both cases.
func TestMessageStatsQueryCount(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want int64
	}{
		{"ten different words", "кот собака дом лес море солнце ветер река город книга", 15},
		{"ten repeated words", strings.TrimSpace(strings.Repeat("кот ", 10)), 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bot, _ := setupTestEnv(t)
			if err := OnText(bot, testContext(bot, tc.text, testUser(42))); err != nil {
				t.Fatal(err)
			}
			counter := &statsQueryCounter{Interface: logger.Discard}
			DB = *DB.Session(&gorm.Session{Logger: counter})
			if err := OnText(bot, testContext(bot, tc.text, testUser(42))); err != nil {
				t.Fatal(err)
			}
			got := counter.count.Load()
			t.Logf("SQL queries per warm message: %d", got)
			if got != tc.want {
				t.Fatalf("SQL queries = %d, want %d", got, tc.want)
			}
		})
	}
}
