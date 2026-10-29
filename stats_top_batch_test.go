package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestStatsTopBatchUserQueries(t *testing.T) {
	for _, tc := range []struct {
		top     string
		kind    int64
		queries int64
	}{
		{"messages", 3, 2}, {"mutes", 8, 2}, {"mutetime", 7, 2}, {"saves", salvationStatType, 2}, {"words", 4, 1}, {"likes", 6, 1},
	} {
		t.Run(tc.top, func(t *testing.T) {
			bot, client := setupTestEnv(t)
			for i := int64(1); i <= 10; i++ {
				// One missing user must fall back to the ID, even with username-less users present.
				if i != 10 {
					if err := DB.Create(&gotgbot.User{Id: i, FirstName: fmt.Sprintf("Ranked%d", i)}).Error; err != nil {
						t.Fatal(err)
					}
				}
				if err := DB.Create(&StatsWords{ID: i, Word: fmt.Sprintf("word%d", i), ShortWord: fmt.Sprint(i)}).Error; err != nil {
					t.Fatal(err)
				}
				if err := statsAdd(tc.kind, GetStartOfDay(), i, 11-i); err != nil {
					t.Fatal(err)
				}
			}
			counter := &statsQueryCounter{Interface: logger.Discard}
			DB = *DB.Session(&gorm.Session{Logger: counter})
			if err := StatsTop(bot, testContext(bot, "/stats "+tc.top, testUser(42))); err != nil {
				t.Fatal(err)
			}
			if got := counter.count.Load(); got != tc.queries {
				t.Fatalf("queries=%d want=%d", got, tc.queries)
			}
			text := strings.Join(client.sentTexts(), "\n")
			if tc.queries == 2 && (!strings.Contains(text, "1. Ranked1 - ") || !strings.Contains(text, "10. 10 - ")) {
				t.Fatalf("wrong names or order: %s", text)
			}
		})
	}
}

func TestStatsTopEmptySkipsUserQuery(t *testing.T) {
	bot, _ := setupTestEnv(t)
	counter := &statsQueryCounter{Interface: logger.Discard}
	DB = *DB.Session(&gorm.Session{Logger: counter})
	if err := StatsTop(bot, testContext(bot, "/stats messages", testUser(42))); err != nil {
		t.Fatal(err)
	}
	if got := counter.count.Load(); got != 1 {
		t.Fatalf("empty top queries=%d", got)
	}
}

func TestStatsTopUserQueryFailure(t *testing.T) {
	bot, client := setupTestEnv(t)
	if err := statsAdd(3, GetStartOfDay(), 42, 1); err != nil {
		t.Fatal(err)
	}
	if err := DB.Migrator().DropTable(&gotgbot.User{}); err != nil {
		t.Fatal(err)
	}
	if err := StatsTop(bot, testContext(bot, "/stats messages", testUser(42))); err == nil {
		t.Fatal("user query failure hidden")
	}
	if len(client.sentTexts()) != 0 {
		t.Fatal("sent a misleading successful report")
	}
}
