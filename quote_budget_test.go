package main

import (
	"testing"
	"time"
)

func TestQuoteSharedBudget(t *testing.T) {
	setupTestEnv(t)
	now := time.Date(2026, 10, 30, 10, 30, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		left, _, err := reserveQuote(now)
		if err != nil || left != 4-i {
			t.Fatalf("%d %d %v", i, left, err)
		}
	}
	left, _, err := reserveQuote(now)
	if err != nil || left != -1 {
		t.Fatal(left, err)
	}
	left, hour, err := reserveQuote(now.Add(time.Hour))
	if err != nil || left != 4 {
		t.Fatal(left, err)
	}
	if quoteBudgetText(left, hour) != "Еще 4 цитаты до 15:00" {
		t.Fatal(quoteBudgetText(left, hour))
	}
}
