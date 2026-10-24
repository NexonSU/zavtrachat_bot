package main

import (
	"strings"
	"testing"
	"unicode/utf16"
)

func TestBlessListFull(t *testing.T) {
	bot, client := setupBlessListTest(t, "яблоко <b>&", "абрикос", strings.Repeat("🍎", 4100))
	if err := BlessListFull(bot, blessPrivateContext(bot, "/blesslistfull", 100)); err != nil {
		t.Fatal(err)
	}
	var parts []string
	for _, call := range client.callsFor("sendMessage") {
		text := call.Params["text"].(string)
		if len(utf16.Encode([]rune(text))) > 4000 {
			t.Fatal("message exceeds limit")
		}
		if call.Params["parse_mode"] != nil && call.Params["parse_mode"] != "" {
			t.Fatal("expected plain text")
		}
		parts = append(parts, text)
	}
	var phrases []Bless
	DB.Order("text").Find(&phrases)
	var want []string
	for _, phrase := range phrases {
		want = append(want, phrase.Text)
	}
	if strings.Join(parts, "") != strings.Join(want, "\n\n") {
		t.Fatal("phrases changed or missing")
	}
}

func TestBlessListFullAccess(t *testing.T) {
	for _, id := range []int64{42, 100} {
		bot, client := setupBlessListTest(t, "secret phrase")
		ctx := blessPrivateContext(bot, "/blesslistfull", id)
		if id == 100 {
			ctx.EffectiveChat.Type = "group"
		}
		if err := BlessListFull(bot, ctx); err != nil {
			t.Fatal(err)
		}
		for _, call := range client.callsFor("sendMessage") {
			if strings.Contains(call.Params["text"].(string), "secret phrase") {
				t.Fatal("unauthorized disclosure")
			}
		}
	}
}

func TestBlessListFullEmpty(t *testing.T) {
	bot, client := setupBlessListTest(t)
	if err := BlessListFull(bot, blessPrivateContext(bot, "/blesslistfull", 100)); err != nil {
		t.Fatal(err)
	}
	if got := client.callsFor("sendMessage"); len(got) != 1 || got[0].Params["text"] != "Фраз пока нет." {
		t.Fatal("missing empty response")
	}
}
