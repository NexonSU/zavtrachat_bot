package main

import (
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestSweetExhaustedBudgetIsPrivateAndSeparateFromQuote(t *testing.T) {
	bot, client := setupTestEnv(t)
	now := time.Now()
	for i := 0; i < 5; i++ {
		left, _, err := reserveCard(now, "sweet_budgets", 5)
		if err != nil || left != 4-i {
			t.Fatalf("reservation %d: remaining %d, error %v", i, left, err)
		}
	}
	left, _, err := reserveQuote(now)
	if err != nil || left != 6 {
		t.Fatalf("/sweet affected /quote: remaining %d, error %v", left, err)
	}
	ctx := testReplyContext(bot, "/sweet", testUser(42), testMessage("Привет!", testUser(77)))
	if err := Sweet(bot, ctx); err != nil {
		t.Fatal(err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("expected a limit notice: %v", calls)
	}
	notice, _ := calls[0].Params["text"].(string)
	eph, _ := calls[0].Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
	if !strings.Contains(notice, "Лимит открыток /sweet в час исчерпан") || eph == nil || eph.ReceiverUserId != 42 {
		t.Fatalf("wrong limit notice: %q, %+v", notice, eph)
	}
	if len(client.callsFor("sendPhoto")) != 0 || len(client.callsFor("getUserProfilePhotos")) != 0 || len(client.callsFor("deleteMessage")) != 0 {
		t.Fatal("exhausted /sweet started rendering or deleted the command")
	}
	left, _, err = reserveCard(now.Add(time.Hour), "sweet_budgets", 5)
	if err != nil || left != 4 {
		t.Fatalf("next hour: remaining %d, error %v", left, err)
	}
}
