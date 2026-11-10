package main

import (
	"strings"
	"testing"
)

func TestAddGamePhrasesWithBothVariants(t *testing.T) {
	bot, client := setupTestEnv(t)
	if err := AddBless(bot, testContext(bot, "/addbless села попой на кактус", testUser(100))); err != nil {
		t.Fatal(err)
	}
	if err := AddSalvation(bot, testContext(bot, "/addsalvation выжил, хотя кактус упал", testUser(100))); err != nil {
		t.Fatal(err)
	}
	var blessing Bless
	if err := DB.First(&blessing, "text = ?", "села попой на кактус").Error; err != nil {
		t.Fatal(err)
	}
	if blessing.Male != "сел попой на кактус" || blessing.Female != "села попой на кактус" {
		t.Fatalf("blessing variants = %+v", blessing.PhraseVariants)
	}
	var salvation Salvation
	if err := DB.First(&salvation).Error; err != nil {
		t.Fatal(err)
	}
	if salvation.Male != "выжил, хотя кактус упал" || salvation.Female != "выжила, хотя кактус упал" {
		t.Fatalf("salvation variants = %+v", salvation.PhraseVariants)
	}
	if texts := strings.Join(client.sentTexts(), "\n"); !strings.Contains(texts, "Женский: <code>выжила, хотя кактус упал</code>") {
		t.Fatalf("confirmation missing variants: %s", texts)
	}
}

func TestBlessingUsesVictimGender(t *testing.T) {
	bot, client := setupTestEnv(t)
	resetSuicideState()
	t.Cleanup(resetSuicideState)
	if err := DB.Model(&Bless{}).Where("text = ?", "да").
		Updates(map[string]any{"male_text": "сел попой на кактус", "female_text": "села попой на кактус"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&Salvation{
		Text:           "выжил",
		PhraseVariants: PhraseVariants{Male: "выжил", Female: "выжила"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := swapPhraseGender(42); err != nil {
		t.Fatal(err)
	}
	if err := blessingWithSalvationRoll(bot, testContext(bot, "/blessing", testUser(42)), func() int { return 0 }); err != nil {
		t.Fatal(err)
	}
	if texts := client.sentTexts(); len(texts) != 1 || !strings.Contains(texts[0], "выжила.\nЖиви.") {
		t.Fatalf("salvation text = %q", texts)
	}
	client.clearCalls()
	seedChatStats(42)
	if err := blessingWithSalvationRoll(bot, testContext(bot, "/blessing", testUser(100)), func() int {
		t.Fatal("admin must not roll salvation")
		return 0
	}); err != nil {
		t.Fatal(err)
	}
	if texts := client.sentTexts(); len(texts) != 1 || !strings.Contains(texts[0], "села попой на кактус") {
		t.Fatalf("ricochet text = %q", texts)
	}
}

func TestExplicitPhraseFormsUpdateExistingRow(t *testing.T) {
	bot, _ := setupTestEnv(t)
	if err := DB.Create(&Bless{Text: "опрометчиво ткнул кактус"}).Error; err != nil {
		t.Fatal(err)
	}
	ctx := testContext(bot, "/addbless опрометчиво {ткнул|ткнула} кактус", testUser(100))
	if err := AddBless(bot, ctx); err != nil {
		t.Fatal(err)
	}
	var rows []Bless
	if err := DB.Find(&rows, "text = ?", "опрометчиво ткнул кактус").Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Female != "опрометчиво ткнула кактус" {
		t.Fatalf("edited phrase = %+v", rows)
	}
}
