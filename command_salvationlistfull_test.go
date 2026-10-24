package main

import (
	"strings"
	"testing"
	"unicode/utf16"
)

func TestSalvationListFull(t *testing.T) {
	bot, client := setupSalvationListTest(t, "яблоко <b>&", "абрикос", strings.Repeat("🍎", 4100))
	if err := SalvationListFull(bot, salvationPrivateContext(bot, "/salvationlistfull", 100)); err != nil {
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
	var phrases []Salvation
	DB.Order("text").Find(&phrases)
	var want []string
	for _, phrase := range phrases {
		want = append(want, phrase.Text)
	}
	if strings.Join(parts, "") != strings.Join(want, "\n\n") {
		t.Fatal("phrases changed or missing")
	}
}

func TestSalvationListFullAccess(t *testing.T) {
	for _, id := range []int64{42, 100} {
		bot, client := setupSalvationListTest(t, "secret phrase")
		ctx := salvationPrivateContext(bot, "/salvationlistfull", id)
		if id == 100 {
			ctx.EffectiveChat.Type = "group"
		}
		if err := SalvationListFull(bot, ctx); err != nil {
			t.Fatal(err)
		}
		for _, call := range client.callsFor("sendMessage") {
			if strings.Contains(call.Params["text"].(string), "secret phrase") {
				t.Fatal("unauthorized disclosure")
			}
		}
	}
}

func TestSalvationListFullEmpty(t *testing.T) {
	bot, client := setupSalvationListTest(t)
	if err := SalvationListFull(bot, salvationPrivateContext(bot, "/salvationlistfull", 100)); err != nil {
		t.Fatal(err)
	}
	if got := client.callsFor("sendMessage"); len(got) != 1 || got[0].Params["text"] != "Фраз пока нет." {
		t.Fatal("missing empty response")
	}
}
