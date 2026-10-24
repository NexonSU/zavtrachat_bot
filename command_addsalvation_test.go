package main

import (
	"strings"
	"testing"
)

func TestAddSalvation(t *testing.T) {
	bot, client := setupTestEnv(t)

	// Non-admin -> animation easter egg, no salvation added.
	if err := AddSalvation(bot, testContext(bot, "/addsalvation text", testUser(42))); err != nil {
		t.Fatalf("AddSalvation (non-admin): %v", err)
	}
	if n := len(client.callsFor("sendAnimation")); n != 1 {
		t.Errorf("AddSalvation (non-admin): sendAnimation calls = %d, want 1", n)
	}
	if n := len(client.callsFor("sendMessage")); n != 0 {
		t.Errorf("AddSalvation (non-admin): sendMessage calls = %d, want 0", n)
	}

	// Usage message.
	client.Calls = nil
	if err := AddSalvation(bot, testContext(bot, "/addsalvation", testUser(100))); err != nil {
		t.Fatalf("AddSalvation (usage): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Пример использования") {
		t.Errorf("AddSalvation (usage) sent %q, want usage message", texts)
	}

	// From arguments.
	client.Calls = nil
	if err := AddSalvation(bot, testContext(bot, "/addsalvation salvation me please", testUser(100))); err != nil {
		t.Fatalf("AddSalvation: %v", err)
	}
	var salvation Salvation
	if err := DB.First(&salvation, "text = ?", "salvation me please").Error; err != nil {
		t.Errorf("AddSalvation: salvation not saved: %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || texts[0] != "Фраза спасения добавлена как <code>salvation me please</code>." {
		t.Errorf("AddSalvation sent %q, want add confirmation", texts)
	}

	// From a replied-to message.
	client.Calls = nil
	replyTo := testMessage("ответный salvation", testUser(42))
	if err := AddSalvation(bot, testReplyContext(bot, "/addsalvation", testUser(100), replyTo)); err != nil {
		t.Fatalf("AddSalvation (reply): %v", err)
	}
	var salvationReply Salvation
	if err := DB.First(&salvationReply, "text = ?", "ответный salvation").Error; err != nil {
		t.Errorf("AddSalvation (reply): salvation not saved: %v", err)
	}

	// Reply to a message without text.
	client.Calls = nil
	empty := testMessage("", testUser(42))
	if err := AddSalvation(bot, testReplyContext(bot, "/addsalvation", testUser(100), empty)); err != nil {
		t.Fatalf("AddSalvation (no text): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "не смог найти текст") {
		t.Errorf("AddSalvation (no text) sent %q, want missing-text message", texts)
	}

	// Too long.
	client.Calls = nil
	long := "/addsalvation " + strings.Repeat("а", 201)
	if err := AddSalvation(bot, testContext(bot, long, testUser(100))); err != nil {
		t.Fatalf("AddSalvation (long): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "длиннее 200 символов") {
		t.Errorf("AddSalvation (long) sent %q, want too-long message", texts)
	}
}

func TestSalvationPoolIsolation(t *testing.T) {
	bot, client := setupTestEnv(t)
	if err := DB.Create(&Bless{Text: "blessing-only"}).Error; err != nil {
		t.Fatal(err)
	}
	var before int64
	DB.Model(&Bless{}).Count(&before)
	if err := AddSalvation(bot, testContext(bot, "/addsalvation отскочил от <катамари> & выжил", testUser(300))); err != nil {
		t.Fatal(err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "&lt;катамари&gt; &amp;") {
		t.Fatalf("confirmation must escape HTML: %v", texts)
	}
	var phrase Salvation
	if err := DB.First(&phrase).Error; err != nil {
		t.Fatal(err)
	}
	if phrase.Text != "отскочил от <катамари> & выжил" {
		t.Fatal("stored text changed")
	}
	var after int64
	DB.Model(&Bless{}).Count(&after)
	if before != after {
		t.Fatal("salvation changed blessings")
	}
	if err := AddSalvation(bot, testContext(bot, "/addsalvation    ", testUser(100))); err != nil {
		t.Fatal(err)
	}
	var total int64
	DB.Model(&Salvation{}).Count(&total)
	if total != 1 {
		t.Fatal("blank phrase was saved")
	}
	for _, name := range []string{"addsalvation", "salvationlist", "salvationlistfull"} {
		found := false
		for _, entry := range commandListConfig() {
			if entry.command.Command == name {
				found = true
			}
		}
		if !found {
			t.Fatalf("command %s not registered", name)
		}
	}
}
