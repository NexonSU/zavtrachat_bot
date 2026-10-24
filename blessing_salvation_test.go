package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestBlessingSalvationOutcomes(t *testing.T) {
	bot, client := setupTestEnv(t)
	DB.Create(&Salvation{Text: "отскочил от катамари"})
	// Exercise the entire 0..99 domain rather than a flaky random sample.
	for roll := 0; roll < 100; roll++ {
		resetSuicideState()
		if err := blessingWithSalvationRoll(bot, testContext(bot, "/blessing", testUser(42)), func() int { return roll }); err != nil {
			t.Fatal(err)
		}
	}
	if got := statTotal(t, salvationStatType, 42); got != 33 {
		t.Fatalf("saves = %d, want 33", got)
	}
	if got := statTotal(t, 8, 42); got != 67 {
		t.Fatalf("deaths = %d, want 67", got)
	}
	if duelistDeaths(t, 42) != 67 || len(client.callsFor("restrictChatMember")) != 67 {
		t.Fatal("wrong death/mute count")
	}
}

func TestBlessingSalvationMessageAndCounters(t *testing.T) {
	bot, client := setupTestEnv(t)
	resetSuicideState()
	DB.Create(&Salvation{Text: "отскочил от <катамари> & выжил"})
	DB.Create(&Duelist{UserID: 42, Kills: 2, Deaths: 5})
	statsAdd(8, GetStartOfDay(), 42, 5)
	statsAdd(7, GetStartOfDay(), 42, 600)
	firstSuicide, lastSuicide, burst, lastVideoSent = 10, 20, 4, 30
	user := testUser(42)
	user.FirstName, user.LastName, user.IsPremium = "<Ник>", "", true
	for _, command := range []string{"blessing", "suicide", "isekai"} {
		client.clearCalls()
		found := false
		for _, entry := range commandListConfig() {
			if entry.command.Command == command {
				found = reflect.ValueOf(entry.response).Pointer() == reflect.ValueOf(Blessing).Pointer()
			}
		}
		if !found {
			t.Fatalf("/%s does not use Blessing", command)
		}
		if err := blessingWithSalvationRoll(bot, testContext(bot, "/"+command, user), func() int { return 32 }); err != nil {
			t.Fatal(err)
		}
		texts := client.sentTexts()
		if len(texts) != 1 || texts[0] != "<Ник> отскочил от <катамари> & выжил.\nЖиви. Пока что." {
			t.Fatalf("salvation message = %q", texts)
		}
		calls := client.callsFor("sendMessage")
		if mode := calls[0].Params["parse_mode"]; mode != nil && mode != "" {
			t.Fatal("salvation must be plain text")
		}
		if len(client.callsFor("restrictChatMember")) != 0 || len(client.callsFor("sendVideo")) != 0 {
			t.Fatal("salvation muted user or sent death video")
		}
	}
	if got := statTotal(t, salvationStatType, 42); got != 3 {
		t.Fatalf("saves = %d", got)
	}
	if duelistDeaths(t, 42) != 5 || statTotal(t, 8, 42) != 5 || muteSeconds(t, 42) != 600 {
		t.Fatal("salvation changed death/mute counters")
	}
	if firstSuicide != 10 || lastSuicide != 20 || burst != 4 || lastVideoSent != 30 {
		t.Fatal("salvation changed death burst state")
	}
	resetSuicideState()
}

func TestBlessingSalvationAdminRicochet(t *testing.T) {
	for _, status := range []string{"administrator", "creator", "sysadmin"} {
		t.Run(status, func(t *testing.T) {
			bot, client := setupTestEnv(t)
			resetSuicideState()
			DB.Create(&Salvation{Text: "отскочил"})
			seedChatStats(777)
			id := int64(55)
			if status == "sysadmin" {
				id = 100
			} else {
				client.setMemberStatus(id, status)
			}
			if err := blessingWithSalvationRoll(bot, testContext(bot, "/blessing", testUser(id)), func() int { t.Fatal("admin must not roll salvation"); return 0 }); err != nil {
				t.Fatal(err)
			}
			calls := client.callsFor("restrictChatMember")
			if len(calls) != 1 || calls[0].Params["user_id"] != int64(777) {
				t.Fatal("ricochet victim was not muted")
			}
			if duelistDeaths(t, 777) != 1 || statTotal(t, salvationStatType, 777) != 0 {
				t.Fatal("ricochet victim got salvation")
			}
		})
	}
}

func TestBlessingSalvationEmptyAndFailure(t *testing.T) {
	t.Run("empty pool", func(t *testing.T) {
		bot, client := setupTestEnv(t)
		resetSuicideState()
		if err := blessingWithSalvationRoll(bot, testContext(bot, "/blessing", testUser(42)), func() int { return 0 }); err != nil {
			t.Fatal(err)
		}
		if len(client.callsFor("restrictChatMember")) != 1 || statTotal(t, salvationStatType, 42) != 0 {
			t.Fatal("empty pool did not fall back to blessing")
		}
	})
	t.Run("failed phrase query", func(t *testing.T) {
		bot, client := setupTestEnv(t)
		DB.Migrator().DropTable(&Salvation{})
		if err := blessingWithSalvationRoll(bot, testContext(bot, "/blessing", testUser(42)), func() int { return 0 }); err == nil {
			t.Fatal("DB error lost")
		}
		if len(client.callsFor("restrictChatMember")) != 0 || statTotal(t, 8, 42) != 0 {
			t.Fatal("DB error caused a death")
		}
	})
	t.Run("send error", func(t *testing.T) {
		bot, client := setupTestEnv(t)
		DB.Create(&Salvation{Text: "выжил!"})
		client.setError("sendMessage", errors.New("unavailable"))
		if err := blessingWithSalvationRoll(bot, testContext(bot, "/blessing", testUser(42)), func() int { return 0 }); err == nil {
			t.Fatal("send error lost")
		}
		if len(client.callsFor("restrictChatMember")) != 0 {
			t.Fatal("send error caused a mute")
		}
	})
}

func TestSalvationStatistics(t *testing.T) {
	bot, client := setupTestEnv(t)
	day := GetStartOfDay()
	statsAdd(salvationStatType, day, 42, 2)
	statsAdd(salvationStatType, day-86400, 42, 3)
	statsAdd(salvationStatType, day-7*86400, 42, 10)
	for _, tc := range []struct{ period, want string }{{"", "15 спасений"}, {" day", "2 спасения"}, {" week", "5 спасений"}} {
		client.clearCalls()
		if err := StatsTop(bot, testContext(bot, "/stats saves"+tc.period, testUser(42))); err != nil {
			t.Fatal(err)
		}
		texts := client.sentTexts()
		if len(texts) != 1 || !strings.Contains(texts[0], tc.want) || !strings.Contains(texts[0], "по спасениям") {
			t.Fatalf("stats saves = %q", texts)
		}
	}
	client.clearCalls()
	if err := Duelstats(bot, testContext(bot, "/duelstats", testUser(42))); err != nil {
		t.Fatal(err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || texts[0] != "У тебя 0 побед, 0 смертей и 15 спасений" {
		t.Fatalf("save-only statistics = %q", texts)
	}
}
