package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"strings"
	"testing"
	"time"
)

func TestQuoteSharedBudget(t *testing.T) {
	setupTestEnv(t)
	now := time.Date(2026, 10, 30, 10, 30, 0, 0, time.UTC)
	for i := 0; i < 7; i++ {
		left, _, err := reserveQuote(now)
		if err != nil || left != 6-i {
			t.Fatalf("%d %d %v", i, left, err)
		}
	}
	left, _, err := reserveQuote(now)
	if err != nil || left != -1 {
		t.Fatal(left, err)
	}
	left, hour, err := reserveQuote(now.Add(time.Hour))
	if err != nil || left != 6 {
		t.Fatal(left, err)
	}
	if quoteBudgetText(left, hour) != "Лимит цитат в час исчерпан. Новые цитаты будут доступны в 15:00." {
		t.Fatal(quoteBudgetText(left, hour))
	}
}

func TestQuoteExhaustionPrivateWithoutCleanup(t *testing.T) {
	bot, client := setupTestEnv(t)
	for i := 0; i < 7; i++ {
		if _, _, err := reserveQuote(time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	ctx := testReplyContext(bot, "/quote", testUser(42), testMessage("Хорошая длинная цитата.", testUser(77)))
	if err := Quote(bot, ctx); err != nil {
		t.Fatal(err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatal(calls)
	}
	text, _ := calls[0].Params["text"].(string)
	eph, _ := calls[0].Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
	if !strings.Contains(text, "Лимит цитат в час исчерпан") || eph == nil || eph.ReceiverUserId != 42 {
		t.Fatal("wrong exhaustion notice", text, eph)
	}
	if len(client.callsFor("sendPhoto")) != 0 || len(client.callsFor("getUserProfilePhotos")) != 0 || len(client.callsFor("deleteMessage")) != 0 {
		t.Fatal("exhausted quote started work or deleted command")
	}
}
