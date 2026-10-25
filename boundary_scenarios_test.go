package main

import (
	"fmt"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	ical "github.com/arran4/golang-ical"
)

func TestSedMalformedInputsAndEscaping(t *testing.T) {
	bot, client := setupTestEnv(t)
	for _, text := range []string{"/sed", "/sed s", "/sed s/", "/sed s/a", "/sed s/a/", "/sed s//b/", "/sed s/a//", "/sed s/a/b/ extra", "/sed nonsense", "/sed s/a/b/extra"} {
		client.clearCalls()
		if err := Sed(bot, testReplyContext(bot, text, testUser(42), testMessage("a <b>&", testUser(43)))); err != nil {
			t.Fatal(err)
		}
		texts := client.sentTexts()
		if len(texts) != 1 || !strings.Contains(texts[0], "Пример использования") {
			t.Fatalf("malformed %q: %q", text, texts)
		}
	}
	client.clearCalls()
	if err := Sed(bot, testReplyContext(bot, "/sed s/a/<new>&/", testUser(42), testMessage("a <b>&", testUser(43)))); err != nil {
		t.Fatal(err)
	}
	if texts := client.sentTexts(); len(texts) != 1 || texts[0] != "&lt;new&gt;&amp; &lt;b&gt;&amp;" {
		t.Fatalf("replacement can inject markup: %q", texts)
	}
	if err := quick.Check(func(s string) bool { a, b, ok := sedPattern(s); return !ok || (a != "" && b != "") }, &quick.Config{MaxCount: 1000}); err != nil {
		t.Fatal(err)
	}
}

func FuzzSedPattern(f *testing.F) {
	for _, text := range []string{"", "/sed", "/sed s/a/b/", "/sed@bot s/кот/пёс/", "/x/foo/bar y", "///", "/sed s/<>&/😀/"} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		a, b, ok := sedPattern(text)
		if ok && (a == "" || b == "") {
			t.Fatal("empty replacement accepted")
		}
	})
}

func TestMalformedMentionsAndReplies(t *testing.T) {
	setupTestEnv(t)
	for _, msg := range []gotgbot.Message{
		{Entities: []gotgbot.MessageEntity{{Type: "text_mention"}}},
		{ReplyToMessage: &gotgbot.Message{}},
		{Entities: []gotgbot.MessageEntity{{Type: "text_mention"}, {Type: "unknown", Offset: -1, Length: 99999}}},
	} {
		if _, err := FindUserInMessage(msg); err == nil {
			t.Fatal("malformed user accepted")
		}
	}
	name := &gotgbot.User{Id: 42, FirstName: "<admin>", LastName: "& friend"}
	if got := MentionUser(name); got != `<a href="tg://user?id=42">&lt;admin&gt; &amp; friend</a>` {
		t.Fatalf("unsafe mention: %q", got)
	}
}

func TestMalformedCallbackEntitiesDoNotChangeData(t *testing.T) {
	for _, entities := range [][]gotgbot.MessageEntity{nil, {{Type: "text_mention"}}, {{Type: "text_mention", User: testUser(777)}, {Type: "text_mention"}}} {
		bot, client := setupTestEnv(t)
		resetDuelState(t)
		msg := testMessage("broken invitation", testUser(1))
		msg.Entities = entities
		seedDuelInvitation(msg)
		ctx := testCallbackContext(bot, msg, testUser(777))
		if err := Accept(bot, ctx); err != nil {
			t.Fatal(err)
		}
		if len(client.callsFor("restrictChatMember")) != 0 {
			t.Fatal("malformed invitation restricted user")
		}
		var count int64
		DB.Model(&Duelist{}).Count(&count)
		if count != 0 {
			t.Fatal("malformed invitation changed counters")
		}
	}
}

func TestReleasesMalformedCalendarAndDates(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	feed := "BEGIN:VCALENDAR\n" +
		"BEGIN:VEVENT\nSUMMARY:Missing date\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nDTSTART:20261026\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nDTSTART:1\nSUMMARY:Short date\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nDTSTART:20261399\nSUMMARY:Bad date\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nDTSTART:2026-10-26\nSUMMARY:Wrong format\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nDTSTART:20261026T120000Z\nSUMMARY:<Game> & Co (PS5)\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nDTSTART:20261025\nSUMMARY:Game\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nDTSTART:20261025\nSUMMARY:Game Extended\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nDTSTART:20261027\nSUMMARY:Game (PC)\nEND:VEVENT\nEND:VCALENDAR\n"
	cal, err := ical.ParseCalendar(strings.NewReader(feed))
	if err != nil {
		t.Fatal(err)
	}
	got := releaseCalendarText(cal, now)
	want := "25.10.2026: Game\n25.10.2026: Game Extended\n26.10.2026: &lt;Game&gt; &amp; Co"
	if got != want {
		t.Fatalf("calendar output %q, want %q", got, want)
	}
	cal, err = ical.ParseCalendar(strings.NewReader("BEGIN:VCALENDAR\nEND:VCALENDAR\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := releaseCalendarText(cal, now); got != "В календаре пока нет ближайших релизов." {
		t.Fatalf("empty calendar: %q", got)
	}
}

func TestReleaseCalendarLimitAndDuplicateTitles(t *testing.T) {
	var feed strings.Builder
	feed.WriteString("BEGIN:VCALENDAR\n")
	for i := 0; i < 25; i++ {
		fmt.Fprintf(&feed, "BEGIN:VEVENT\nDTSTART:20261026\nSUMMARY:Game %02d\nEND:VEVENT\n", i)
	}
	feed.WriteString("END:VCALENDAR\n")
	cal, err := ical.ParseCalendar(strings.NewReader(feed.String()))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(releaseCalendarText(cal, time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)), "\n")
	if len(lines) != 20 || !strings.HasSuffix(lines[19], "Game 19") {
		t.Fatalf("wrong release limit: %v", lines)
	}
}

func TestWarningWeekBoundaries(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		age  time.Duration
		want int
	}{{7*24*time.Hour - time.Nanosecond, 3}, {7 * 24 * time.Hour, 2}, {7*24*time.Hour + time.Nanosecond, 2}, {14 * 24 * time.Hour, 1}, {21 * 24 * time.Hour, 0}, {-24 * time.Hour, 3}} {
		if got := warningAmountAt(Warn{Amount: 3, LastWarn: now.Add(-tc.age)}, now); got != tc.want {
			t.Fatalf("age %s: %d want %d", tc.age, got, tc.want)
		}
	}
	bot, _ := setupTestEnv(t)
	DB.Create(&Warn{UserID: 42, Amount: 3, LastWarn: now.Add(-7 * 24 * time.Hour)})
	if err := warnUserAt(bot, testReplyContext(bot, "/warn", testUser(100), testMessage("target", testUser(42))), now); err != nil {
		t.Fatal(err)
	}
	var row Warn
	DB.First(&row, 42)
	if row.Amount != 3 {
		t.Fatalf("boundary command amount=%d", row.Amount)
	}
}

func TestDailyDrawMidnightAndDST(t *testing.T) {
	bot, client := setupTestEnv(t)
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	midnight := time.Date(2026, 10, 25, 0, 0, 0, 0, loc)
	before := midnight.Add(-time.Nanosecond)
	DB.Create(&PidorList{Id: 42, Username: "yesterday"})
	DB.Create(&PidorList{Id: 43, Username: "today"})
	DB.Create(&PidorStats{Date: before, UserID: 42})
	DB.Create(&PidorStats{Date: midnight, UserID: 43})
	for _, tc := range []struct {
		now  time.Time
		want string
	}{{before, "yesterday"}, {midnight, "today"}, {midnight.Add(time.Nanosecond), "today"}} {
		client.clearCalls()
		if err := pidorAt(bot, testContext(bot, "/pidor", testUser(42)), tc.now); err != nil {
			t.Fatal(err)
		}
		texts := client.sentTexts()
		if len(texts) != 1 || !strings.Contains(texts[0], tc.want) {
			t.Fatalf("draw at %s: %q", tc.now, texts)
		}
	}
	start, end := dayBounds(midnight)
	if end.Sub(start) != 25*time.Hour {
		t.Fatalf("DST day incorrectly treated as 24 hours: %v", end.Sub(start))
	}
}
