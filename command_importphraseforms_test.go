package main

import "testing"

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
