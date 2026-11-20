package main

import (
	"testing"
	"time"
)

func TestQuoteUnlimitedWithLegacyExhaustedBudget(t *testing.T) {
	bot, client := setupTestEnv(t)
	hour := time.Now().Unix() / 3600
	if err := DB.Create(&QuoteBudget{Hour: hour, Used: 7}).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		ctx := testReplyContext(bot, "/quote", testUser(int64(42+i)), testMessage("Хорошая длинная цитата.", testUser(77)))
		if err := Quote(bot, ctx); err != nil {
			t.Fatalf("quote %d: %v", i+1, err)
		}
	}
	if len(client.callsFor("sendPhoto")) != 8 || len(client.callsFor("deleteMessage")) != 8 {
		t.Fatal("hourly limit prevented quote delivery or cleanup")
	}
	if len(client.callsFor("sendMessage")) != 0 {
		t.Fatal("unexpected quote notice")
	}
	var budgets []QuoteBudget
	if err := DB.Find(&budgets).Error; err != nil || len(budgets) != 1 || budgets[0].Hour != hour || budgets[0].Used != 7 {
		t.Fatalf("quote changed the legacy budget: %+v, %v", budgets, err)
	}
}
