package main

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestPhraseImportAcceptsMainChatAdminInPrivate(t *testing.T) {
	bot, client := setupTestEnv(t)
	client.setMemberStatus(42, "administrator")

	for _, tc := range []struct {
		userID  int64
		private bool
		want    string
	}{
		{42, true, "Ответь /importphraseforms на JSON-файл миграции размером до 256 КБ."},
		{300, true, "Импорт доступен только администраторам основного чата и бота."},
		{42, false, ""},
	} {
		client.clearCalls()
		msg := testMessage("/importphraseforms", testUser(tc.userID))
		if tc.private {
			msg.Chat.Id, msg.Chat.Type = tc.userID, "private"
		}
		ctx := ext.NewContext(bot, &gotgbot.Update{Message: msg}, nil)
		if err := ImportPhraseForms(bot, ctx); err != nil {
			t.Fatal(err)
		}
		replies := client.sentTexts()
		if tc.want == "" {
			if len(replies) != 0 {
				t.Fatalf("unexpected public reply: %q", replies)
			}
		} else if len(replies) != 1 || replies[0] != tc.want {
			t.Fatalf("user %d received %q, want %q", tc.userID, replies, tc.want)
		}
	}
}

func TestPhraseImportUpdatesExistingRowsOnce(t *testing.T) {
	setupTestEnv(t)
	if err := DB.Create(&Bless{Text: " тестовая фраза"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&Salvation{Text: "тестовое спасение", PhraseVariants: PhraseVariants{
		Male: "тестовое спасение", Female: "ручная правка",
	}}).Error; err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"blessings":[{"male":" тестовая фраза","female":" проверенная форма"},{"male":"нет записи","female":"не создавать"}],"salvations":[{"male":"тестовое спасение","female":"не перезаписывать"}]}`)
	first, err := applyPhraseImport(&DB, data)
	if err != nil || first != (phraseImportResult{updated: 1, skipped: 1, missing: 1}) {
		t.Fatalf("first import = %+v, %v", first, err)
	}
	second, err := applyPhraseImport(&DB, data)
	if err != nil || second != (phraseImportResult{updated: 0, skipped: 2, missing: 1}) {
		t.Fatalf("second import = %+v, %v", second, err)
	}
	var bless Bless
	if err := DB.Take(&bless, "text = ?", " тестовая фраза").Error; err != nil {
		t.Fatal(err)
	}
	if bless.Male != " тестовая фраза" || bless.Female != " проверенная форма" {
		t.Fatalf("saved forms = %+v", bless.PhraseVariants)
	}
}

func TestPhraseImportRejectsDuplicatesBeforeUpdating(t *testing.T) {
	setupTestEnv(t)
	if err := DB.Create(&Bless{Text: "старая фраза"}).Error; err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"blessings":[{"male":"старая фраза","female":"первая"},{"male":"старая фраза","female":"вторая"}]}`)
	if _, err := applyPhraseImport(&DB, data); err == nil {
		t.Fatal("expected duplicate rows to be rejected")
	}
	var bless Bless
	if err := DB.Take(&bless, "text = ?", "старая фраза").Error; err != nil {
		t.Fatal(err)
	}
	if bless.Female != "" {
		t.Fatalf("invalid import updated the database: %+v", bless)
	}
}
