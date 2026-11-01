package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func birthdayNow() time.Time { return time.Date(2026, 11, 3, 7, 0, 0, 0, time.UTC) }

func TestBirthdayImportIdempotentAndDeletionPersists(t *testing.T) {
	setupTestEnv(t)
	seedUser(42, "nakxwest", "Pan Kotek")
	if err := importBirthdays(&DB); err != nil {
		t.Fatal(err)
	}
	var n int64
	DB.Model(&Birthday{}).Count(&n)
	if n != 90 {
		t.Fatal(n)
	}
	var b Birthday
	if err := DB.Where("username = ?", "nakxwest").First(&b).Error; err != nil {
		t.Fatal(err)
	}
	if b.UserID == nil || *b.UserID != 42 || b.Day != 23 || b.Month != 6 {
		t.Fatal(b)
	}
	if deleted, err := deleteBirthday(&DB, gotgbot.User{Id: 42, Username: "nakxwest"}); err != nil || !deleted {
		t.Fatal(deleted, err)
	}
	if err := importBirthdays(&DB); err != nil {
		t.Fatal(err)
	}
	DB.First(&b, b.ID)
	if !b.Deleted {
		t.Fatal("import resurrected deleted birthday")
	}
	DB.Model(&Birthday{}).Count(&n)
	if n != 90 {
		t.Fatal(n)
	}
	var copies int64
	DB.Model(&Birthday{}).Where("username = ?", "ilfox").Count(&copies)
	if copies != 1 {
		t.Fatal(copies)
	}
}

func TestBirthdayCommandsOwnershipPrivateAndErrors(t *testing.T) {
	bot, c := setupTestEnv(t)
	u := testUser(42)
	for _, tc := range []struct{ text, want string }{
		{"/addbirthday 31.02", "Неверная дата"},
		{"/addbirthday 23.06 @other", "Укажи свой"},
		{"/addbirthday 23.06", "день рождения добавлен!"},
		{"/addbirthday 24.06", "день рождения уже в базе"},
	} {
		c.clearCalls()
		if err := AddBirthday(bot, testContext(bot, tc.text, u)); err != nil {
			t.Fatal(err)
		}
		calls := c.callsFor("sendMessage")
		if len(calls) != 1 || !strings.Contains(calls[0].Params["text"].(string), tc.want) {
			t.Fatal(calls)
		}
		eph, _ := calls[0].Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
		if eph == nil || eph.ReceiverUserId != 42 {
			t.Fatal("public response")
		}
	}
	var b Birthday
	DB.First(&b)
	if b.Day != 23 || b.AddedBy != 42 || b.UserID == nil || *b.UserID != 42 {
		t.Fatal(b)
	}
	// Other callers cannot delete the owner's record, even with a target argument.
	if err := DelBirthday(bot, testContext(bot, "/delbirthday @testpupkin", testUser(43))); err != nil {
		t.Fatal(err)
	}
	DB.First(&b)
	if b.Deleted {
		t.Fatal("another user's birthday deleted")
	}
	if err := DelBirthday(bot, testContext(bot, "/delbirthday", u)); err != nil {
		t.Fatal(err)
	}
	if err := DelBirthday(bot, testContext(bot, "/delbirthday", u)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(c.sentTexts(), " "), "дня рождения нет в базе") {
		t.Fatal(c.sentTexts())
	}
	c.setError("sendMessage", errors.New("private response failed"))
	c.clearCalls()
	if err := AddBirthday(bot, testContext(bot, "/addbirthday 29.02", u)); err == nil {
		t.Fatal("lost delivery error")
	}
	if len(c.callsFor("sendMessage")) != 1 {
		t.Fatal("public fallback")
	}
}

func TestBirthdayNoUsernameAndRename(t *testing.T) {
	setupTestEnv(t)
	u := gotgbot.User{Id: 42, FirstName: "Имя"}
	for _, id := range []int64{42, 43} {
		u.Id = id
		if ok, err := addBirthday(&DB, u, 29, 2); err != nil || !ok {
			t.Fatal(ok, err)
		}
	}
	u.Id = 42
	u.Username = "renamed"
	if ok, err := addBirthday(&DB, u, 1, 1); err != nil || ok {
		t.Fatal(ok, err)
	}
	if ok, err := deleteBirthday(&DB, u); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if ok, err := addBirthday(&DB, u, 1, 1); err != nil || !ok {
		t.Fatal(ok, err)
	}
}

func TestBirthdayUpcomingYearBoundaryAndLeapDay(t *testing.T) {
	setupTestEnv(t)
	for i := 1; i <= 15; i++ {
		u := gotgbot.User{Id: int64(i)}
		if _, err := addBirthday(&DB, u, i, 1); err != nil {
			t.Fatal(err)
		}
	}
	u := gotgbot.User{Id: 99}
	addBirthday(&DB, u, 31, 12)
	now := time.Date(2026, 12, 30, 22, 0, 0, 0, time.UTC) // Already December 31 in Moscow.
	rows, err := upcomingBirthdays(&DB, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 10 || rows[0].Day != 31 || rows[1].Day != 1 || nextBirthday(rows[1], now).Year() != 2027 {
		t.Fatal(rows)
	}
	for _, tc := range []struct{ year, day int }{{2027, 28}, {2028, 29}, {2100, 28}} {
		if d := birthdayDate(Birthday{Day: 29, Month: 2}, tc.year); d.Month() != 2 || d.Day() != tc.day {
			t.Fatal(d)
		}
	}
}

func TestBirthdayLookupImportedAndDeleted(t *testing.T) {
	bot, c := setupTestEnv(t)
	if err := importBirthdays(&DB); err != nil {
		t.Fatal(err)
	}
	if err := WhenYourBirthday(bot, testContext(bot, "/whenyourbirthday @NAKXWEST", testUser(42))); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.sentTexts()[0], "23.06") {
		t.Fatal(c.sentTexts())
	}
	u := gotgbot.User{Id: 42, Username: "nakxwest"}
	deleteBirthday(&DB, u)
	c.clearCalls()
	if err := WhenYourBirthday(bot, testContext(bot, "/whenyourbirthday @nakxwest", &u)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.sentTexts()[0], "нет в базе") {
		t.Fatal(c.sentTexts())
	}
	if err := Birthdays(bot, testContext(bot, "/birthdays", &u)); err != nil {
		t.Fatal(err)
	}
}

func TestBirthdaySchedulerTimeMembershipRetryAndRestart(t *testing.T) {
	bot, c := setupTestEnv(t)
	u := gotgbot.User{Id: 42, Username: "old"}
	addBirthday(&DB, u, 3, 11)
	now := birthdayNow()
	if err := sendDueBirthdays(bot, &DB, Config.Chat, now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(c.callsFor("getChatMember")) != 0 {
		t.Fatal("checked before 10 Moscow")
	}
	for _, status := range []string{"left", "kicked"} {
		c.setMemberStatus(42, status)
		if err := sendDueBirthdays(bot, &DB, Config.Chat, now); err != nil {
			t.Fatal(err)
		}
	}
	if len(c.callsFor("sendPhoto")) != 0 {
		t.Fatal("congratulated absent user")
	}
	c.setResult("getChatMember", json.RawMessage(`{"status":"restricted","is_member":true,"user":{"id":42,"first_name":"Pan Kotek","username":"nakxwest"}}`))
	c.setError("sendPhoto", errors.New("temporary upload failure"))
	if err := sendDueBirthdays(bot, &DB, Config.Chat, now); err == nil {
		t.Fatal("lost upload failure")
	}
	var count int64
	DB.Model(&BirthdayDelivery{}).Count(&count)
	if count != 0 {
		t.Fatal("failed send marked delivered")
	}
	c.setError("sendPhoto", nil)
	c.clearCalls()
	if err := sendDueBirthdays(bot, &DB, Config.Chat, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	photos := c.callsFor("sendPhoto")
	if len(photos) != 1 {
		t.Fatal(photos)
	}
	if !strings.Contains(photos[0].Params["caption"].(string), "@nakxwest") || photos[0].Params["chat_id"] != Config.Chat {
		t.Fatal(photos[0])
	}
	if eph := photos[0].Params["ephemeral_message_parameters"]; eph != nil {
		t.Fatal("private birthday card")
	}
	if err := sendDueBirthdays(bot, &DB, Config.Chat, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	deleteBirthday(&DB, u)
	addBirthday(&DB, u, 3, 11)
	if err := sendDueBirthdays(bot, &DB, Config.Chat, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(c.callsFor("sendPhoto")) != 1 {
		t.Fatal("duplicate after restart or delete/readd")
	}
}

func TestBirthdaySchedulerUnresolvedLeapAndMembershipFailure(t *testing.T) {
	bot, c := setupTestEnv(t)
	b := Birthday{Username: "unknown", Name: "Неизвестный", Day: 29, Month: 2}
	DB.Create(&b)
	now := time.Date(2027, 2, 28, 7, 0, 0, 0, time.UTC)
	if err := sendDueBirthdays(bot, &DB, Config.Chat, now); err != nil {
		t.Fatal(err)
	}
	if len(c.callsFor("sendPhoto")) != 0 {
		t.Fatal("unknown identity sent")
	}
	seedUser(42, "unknown", "Известный")
	c.setError("getChatMember", errors.New("membership failed"))
	if err := sendDueBirthdays(bot, &DB, Config.Chat, now); err == nil {
		t.Fatal("unverified membership")
	}
	c.setError("getChatMember", nil)
	if err := sendDueBirthdays(bot, &DB, Config.Chat, now); err != nil {
		t.Fatal(err)
	}
	if len(c.callsFor("sendPhoto")) != 1 {
		t.Fatal("leap day not celebrated February 28")
	}
}

func TestBirthdayAdminTestDoesNotConsumeAnnualGreeting(t *testing.T) {
	bot, c := setupTestEnv(t)
	seedUser(77, "nakxwest", "Pan Kotek")
	if err := TestBirthday(bot, testContext(bot, "/testbirthday", testUser(42))); err != nil {
		t.Fatal(err)
	}
	if len(c.callsFor("sendPhoto")) != 0 {
		t.Fatal("nonadmin sent greeting")
	}
	if err := TestBirthday(bot, testContext(bot, "/testbirthday", testUser(100))); err != nil {
		t.Fatal(err)
	}
	if len(c.callsFor("sendPhoto")) != 1 {
		t.Fatal("admin did not send greeting")
	}
	var count int64
	DB.Model(&BirthdayDelivery{}).Count(&count)
	if count != 0 {
		t.Fatal("test consumed annual greeting")
	}
}

func TestBirthdayRenderAllBackgrounds(t *testing.T) {
	backgrounds, err := loadBirthdayBackgrounds()
	if err != nil || len(backgrounds) != 10 {
		t.Fatal(len(backgrounds), err)
	}
	for i := range backgrounds {
		var avatar = quoteTestAvatar()
		if i == 0 {
			avatar = nil
		}
		data, err := renderBirthday("Pan Kotek", avatar, i)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil || img.Bounds().Dx() != 1200 || img.Bounds().Dy() != 1200 {
			t.Fatal(err)
		}
		// Optional local previews used for visual QA, never part of runtime.
		if dir := os.Getenv("BIRTHDAY_PREVIEW_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("birthday-%02d.png", i)), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := renderBirthday(strings.Repeat("Кот 🐈 ", 100), quoteTestAvatar(), 9); err != nil {
		t.Fatal(err)
	}
}

func TestBirthdayPostgresConcurrentDeliveryAndRegistration(t *testing.T) {
	bot, c := setupTestEnv(t)
	dsn := isolatedPostgresDSN(t)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	closeTestDB(t, db)
	if err := db.AutoMigrate(&gotgbot.User{}, &Birthday{}, &BirthdayDelivery{}, &BirthdayImport{}); err != nil {
		t.Fatal(err)
	}
	if err := importBirthdays(db); err != nil {
		t.Fatal(err)
	}
	u := gotgbot.User{Id: 42, Username: "fresh"}
	start := make(chan struct{})
	results := make(chan bool, 2)
	errs := make(chan error, 2)
	for range 2 {
		go func() { <-start; ok, err := addBirthday(db, u, 3, 11); results <- ok; errs <- err }()
	}
	close(start)
	added := 0
	for range 2 {
		if <-results {
			added++
		}
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if added != 1 {
		t.Fatal("duplicate registrations", added)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	c.Observe = func(method string, _ map[string]any) {
		if method == "sendPhoto" {
			close(entered)
			<-release
		}
	}
	first := make(chan error, 1)
	go func() { first <- sendDueBirthdays(bot, db, Config.Chat, birthdayNow()) }()
	select {
	case <-entered:
	case <-time.After(15 * time.Second):
		close(release)
		t.Fatal("first worker did not send")
	}
	second := make(chan error, 1)
	go func() { second <- sendDueBirthdays(bot, db, Config.Chat, birthdayNow()) }()
	select {
	case err := <-second:
		if err != nil {
			close(release)
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		close(release)
		<-first
		t.Fatal("worker failed to skip locked birthday")
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	c.Observe = nil
	reopened, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	closeTestDB(t, reopened)
	if err := importBirthdays(reopened); err != nil {
		t.Fatal(err)
	}
	if err := sendDueBirthdays(bot, reopened, Config.Chat, birthdayNow()); err != nil {
		t.Fatal(err)
	}
	if len(c.callsFor("sendPhoto")) != 1 {
		t.Fatal("duplicate after reconnect")
	}
}
