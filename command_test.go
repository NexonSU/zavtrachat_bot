package main

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var targetUser = gotgbot.User{Id: 777, FirstName: "Цель", LastName: "Целевич", Username: "target"}

func TestPing(t *testing.T) {
	bot, client := setupTestEnv(t)
	if err := Ping(bot, testContext(bot, "/ping", testUser(42))); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || texts[0] != "Pong!" {
		t.Errorf("Ping sent %q, want [Pong!]", texts)
	}
}

func TestMarco(t *testing.T) {
	bot, client := setupTestEnv(t)
	if err := Marco(bot, testContext(bot, "marco", testUser(42))); err != nil {
		t.Fatalf("Marco: %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || texts[0] != "Polo!" {
		t.Errorf("Marco sent %q, want [Polo!]", texts)
	}
}

func TestShrug(t *testing.T) {
	bot, client := setupTestEnv(t)
	if err := Shrug(bot, testContext(bot, "/shrug", testUser(42))); err != nil {
		t.Fatalf("Shrug: %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || texts[0] != `<code>¯\_(ツ)_/¯</code>` {
		t.Errorf("Shrug sent %q", texts)
	}
}

func TestDate(t *testing.T) {
	bot, client := setupTestEnv(t)
	if err := Date(bot, testContext(bot, "/date", testUser(42))); err != nil {
		t.Fatalf("Date: %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 {
		t.Fatalf("Date sent %q, want one message", texts)
	}
	if matched, _ := regexp.MatchString(`^\d{2}\.\d{2}\.\d{4} \d{2}:\d{2}$`, texts[0]); !matched {
		t.Errorf("Date sent %q, want format DD.MM.YYYY HH:MM", texts[0])
	}
}

func TestStatsTop(t *testing.T) {
	bot, client := setupTestEnv(t)

	// No argument -> the list of tops.
	if err := StatsTop(bot, testContext(bot, "/stats", testUser(42))); err != nil {
		t.Fatalf("StatsTop (no args): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "/stats likes") {
		t.Errorf("StatsTop (no args) sent %q, want usage", texts)
	}

	// Unknown top and unknown period.
	client.Calls = nil
	if err := StatsTop(bot, testContext(bot, "/stats bogus", testUser(42))); err != nil {
		t.Fatalf("StatsTop (bogus): %v", err)
	}
	if texts := client.sentTexts(); len(texts) != 1 || !strings.Contains(texts[0], "Непонятный топ") {
		t.Errorf("StatsTop (bogus) sent %q, want unknown top message", texts)
	}
	client.Calls = nil
	if err := StatsTop(bot, testContext(bot, "/stats likes month", testUser(42))); err != nil {
		t.Fatalf("StatsTop (month): %v", err)
	}
	if texts := client.sentTexts(); len(texts) != 1 || !strings.Contains(texts[0], "Непонятный период") {
		t.Errorf("StatsTop (month) sent %q, want unknown period message", texts)
	}

	// Seed one row per top: likes are per message, deaths/mute time/messages
	// per user, words per dictionary id.
	client.Calls = nil
	seedUser(42, "test42", "Тест")
	DB.Create(&Stats{ContextID: 1001, StatType: 6, DayTimestamp: 1, Count: 5})
	DB.Create(&Stats{ContextID: 1002, StatType: 6, DayTimestamp: 1, Count: 12})
	DB.Create(&Stats{ContextID: 42, StatType: 8, DayTimestamp: 1, Count: 3})
	DB.Create(&Stats{ContextID: 777, StatType: 8, DayTimestamp: 1, Count: 1})
	DB.Create(&Stats{ContextID: 42, StatType: 7, DayTimestamp: 1, Count: 90})
	DB.Create(&Stats{ContextID: 42, StatType: 3, DayTimestamp: 1, Count: 125})
	DB.Create(&StatsWords{ID: 1, Word: "бляха", ShortWord: "бляха"})
	DB.Create(&Stats{ContextID: 1, StatType: 4, DayTimestamp: 1, Count: 10})
	DB.Create(&StatsWords{ID: 2, Word: "сука", ShortWord: "сука"})
	DB.Create(&Stats{ContextID: 2, StatType: 4, DayTimestamp: 1, Count: 5})

	// Likes rank the messages themselves, most liked first, with a link.
	if err := StatsTop(bot, testContext(bot, "/stats likes", testUser(42))); err != nil {
		t.Fatalf("StatsTop (likes): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 ||
		!strings.Contains(texts[0], "Топ-10 по лайкам за всё время") ||
		strings.Index(texts[0], "1002") > strings.Index(texts[0], "1001") ||
		!strings.Contains(texts[0], "12 лайков") {
		t.Errorf("StatsTop (likes) sent %q, want ranked message links", texts)
	}

	// Deaths top: known users by name, unknown by id, declined counts; the
	// blessings alias titles the top "по блесингам".
	client.Calls = nil
	if err := StatsTop(bot, testContext(bot, "/stats blessings", testUser(42))); err != nil {
		t.Fatalf("StatsTop (blessings): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 ||
		!strings.Contains(texts[0], "Топ-10 по блесингам за всё время") ||
		!strings.Contains(texts[0], "1. Тест - 3 смерти") ||
		!strings.Contains(texts[0], "2. 777 - 1 смерть") {
		t.Errorf("StatsTop (blessings) sent %q, want ranked deaths", texts)
	}

	// Mute time top spells the durations out; the blesstime alias titles the
	// top "по времени в блесинге".
	client.Calls = nil
	if err := StatsTop(bot, testContext(bot, "/stats blesstime", testUser(42))); err != nil {
		t.Fatalf("StatsTop (blesstime): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 ||
		!strings.Contains(texts[0], "Топ-10 по времени в блесинге за всё время") ||
		!strings.Contains(texts[0], "1. Тест - 1 минута 30 секунд") {
		t.Errorf("StatsTop (blesstime) sent %q, want ranked durations", texts)
	}

	// Messages top.
	client.Calls = nil
	if err := StatsTop(bot, testContext(bot, "/stats messages", testUser(42))); err != nil {
		t.Fatalf("StatsTop (messages): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "1. Тест - 125 сообщений") {
		t.Errorf("StatsTop (messages) sent %q, want ranked message counts", texts)
	}

	// Words top shows the word itself from the dictionary.
	client.Calls = nil
	if err := StatsTop(bot, testContext(bot, "/stats words", testUser(42))); err != nil {
		t.Fatalf("StatsTop (words): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 ||
		!strings.Contains(texts[0], "1. бляха - 10 раз") ||
		!strings.Contains(texts[0], "2. сука - 5 раз") {
		t.Errorf("StatsTop (words) sent %q, want ranked words", texts)
	}

	// Period filters: day counts only today, week sums the last 7 days and
	// ignores the older seeded rows.
	client.Calls = nil
	today := GetStartOfDay()
	DB.Create(&Stats{ContextID: 42, StatType: 3, DayTimestamp: today, Count: 7})
	DB.Create(&Stats{ContextID: 42, StatType: 3, DayTimestamp: today - 3*86400, Count: 4})
	if err := StatsTop(bot, testContext(bot, "/stats messages day", testUser(42))); err != nil {
		t.Fatalf("StatsTop (day): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 ||
		!strings.Contains(texts[0], "за сутки") ||
		!strings.Contains(texts[0], "1. Тест - 7 сообщений") {
		t.Errorf("StatsTop (day) sent %q, want only today's counts", texts)
	}
	client.Calls = nil
	if err := StatsTop(bot, testContext(bot, "/stats messages week", testUser(42))); err != nil {
		t.Fatalf("StatsTop (week): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 ||
		!strings.Contains(texts[0], "за неделю") ||
		!strings.Contains(texts[0], "1. Тест - 11 сообщений") {
		t.Errorf("StatsTop (week) sent %q, want the last 7 days summed", texts)
	}
}

func TestMe(t *testing.T) {
	bot, client := setupTestEnv(t)

	// No argument -> usage message.
	if err := Me(bot, testContext(bot, "/me", testUser(42))); err != nil {
		t.Fatalf("Me (no args): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Пример использования") {
		t.Errorf("Me (no args) sent %q, want usage message", texts)
	}

	// With argument -> original message removed, formatted message sent.
	client.Calls = nil
	if err := Me(bot, testContext(bot, "/me делает что-то", testUser(42))); err != nil {
		t.Fatalf("Me: %v", err)
	}
	if len(client.callsFor("deleteMessage")) != 1 {
		t.Errorf("Me: deleteMessage calls = %d, want 1", len(client.callsFor("deleteMessage")))
	}
	texts = client.sentTexts()
	want := "<code>Тест Пупкин делает что-то</code>"
	if len(texts) != 1 || texts[0] != want {
		t.Errorf("Me sent %q, want %q", texts, want)
	}
}

func TestSay(t *testing.T) {
	bot, client := setupTestEnv(t)

	// Non-admin gets Denied (a random Nope text from DB).
	if err := Say(bot, testContext(bot, "/say hello", testUser(42))); err != nil {
		t.Fatalf("Say (non-admin): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || texts[0] != "нет" {
		t.Errorf("Say (non-admin) sent %q, want the seeded Nope text", texts)
	}

	// Admin without text -> hint.
	client.Calls = nil
	if err := Say(bot, testContext(bot, "/say", testUser(100))); err != nil {
		t.Fatalf("Say (no args): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || texts[0] != "Укажите сообщение." {
		t.Errorf("Say (no args) sent %q", texts)
	}

	// Admin with text -> original removed, text reposted.
	client.Calls = nil
	if err := Say(bot, testContext(bot, "/say hello world", testUser(100))); err != nil {
		t.Fatalf("Say: %v", err)
	}
	if len(client.callsFor("deleteMessage")) != 1 {
		t.Errorf("Say: deleteMessage calls = %d, want 1", len(client.callsFor("deleteMessage")))
	}
	texts = client.sentTexts()
	if len(texts) != 1 || texts[0] != "hello world" {
		t.Errorf("Say sent %q, want [hello world]", texts)
	}
}

func TestSed(t *testing.T) {
	bot, client := setupTestEnv(t)

	// Without reply -> usage message.
	if err := Sed(bot, testContext(bot, "/x/foo/bar y", testUser(42))); err != nil {
		t.Fatalf("Sed (no reply): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Пример использования") {
		t.Errorf("Sed (no reply) sent %q, want usage message", texts)
	}

	// With reply -> pattern from the command applied to the replied message.
	client.Calls = nil
	replyTo := testMessage("x x foo", &targetUser)
	if err := Sed(bot, testReplyContext(bot, "/x/foo/bar y", testUser(42), replyTo)); err != nil {
		t.Fatalf("Sed: %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || texts[0] != "foo foo foo" {
		t.Errorf("Sed sent %q, want [foo foo foo]", texts)
	}
}

func TestKhaleesi(t *testing.T) {
	bot, client := setupTestEnv(t)

	// Without reply -> usage message.
	if err := Khaleesi(bot, testContext(bot, "/khaleesi", testUser(42))); err != nil {
		t.Fatalf("Khaleesi (no reply): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Пример использования") {
		t.Errorf("Khaleesi (no reply) sent %q, want usage message", texts)
	}

	// With reply -> text distorted, requester credited, command message removed.
	client.Calls = nil
	replyTo := testMessage("нет", &targetUser)
	if err := Khaleesi(bot, testReplyContext(bot, "/khaleesi", testUser(42), replyTo)); err != nil {
		t.Fatalf("Khaleesi: %v", err)
	}
	texts = client.sentTexts()
	want := "нит\n\n<i>По просьбе @\u200btestpupkin</i>"
	if len(texts) != 1 || texts[0] != want {
		t.Errorf("Khaleesi sent %q, want [%q]", texts, want)
	}
	if len(client.callsFor("deleteMessage")) != 1 {
		t.Errorf("Khaleesi: deleteMessage calls = %d, want 1", len(client.callsFor("deleteMessage")))
	}

	// Users without a username are credited by an escaped, plain display name.
	client.Calls = nil
	requester := testUser(43)
	requester.Username = ""
	requester.FirstName = "<Имя>"
	requester.LastName = ""
	replyTo = testMessage("<b>нет</b>", &targetUser)
	if err := Khaleesi(bot, testReplyContext(bot, "/khaleesi", requester, replyTo)); err != nil {
		t.Fatalf("Khaleesi (no username): %v", err)
	}
	texts = client.sentTexts()
	want = "&lt;b&gt;нит&lt;/b&gt;\n\n<i>По просьбе &lt;Имя&gt;</i>"
	if len(texts) != 1 || texts[0] != want {
		t.Errorf("Khaleesi (no username) sent %q, want [%q]", texts, want)
	}

	// Mentions in captions and repeated translations remain plain text.
	client.Calls = nil
	replyTo = testMessage("", &targetUser)
	replyTo.Caption = "@target @other_user @\u200btestpupkin нет <tag>"
	if err := Khaleesi(bot, testReplyContext(bot, "/khaleesi", testUser(42), replyTo)); err != nil {
		t.Fatalf("Khaleesi (mentions): %v", err)
	}
	texts = client.sentTexts()
	want = "@\u200btarget @\u200bother_user @\u200btestpupkin нит &lt;tag&gt;\n\n<i>По просьбе @\u200btestpupkin</i>"
	if len(texts) != 1 || texts[0] != want {
		t.Errorf("Khaleesi (mentions) sent %q, want [%q]", texts, want)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 || calls[0].Params["parse_mode"] != gotgbot.ParseModeHTML || calls[0].Params["entities"] != nil {
		t.Errorf("Khaleesi (mentions): expected HTML formatting without explicit mention entities: %+v", calls)
	}
}

func TestGetid(t *testing.T) {
	bot, client := setupTestEnv(t)
	seedUser(targetUser.Id, targetUser.Username, targetUser.FirstName)

	// Non-admin -> Denied.
	if err := Getid(bot, testContext(bot, "/getid", testUser(42))); err != nil {
		t.Fatalf("Getid (non-admin): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || texts[0] != "нет" {
		t.Errorf("Getid (non-admin) sent %q, want Nope text", texts)
	}

	// Admin replying to a message -> info about the replied-to user, sent privately.
	client.Calls = nil
	replyTo := testMessage("цельское сообщение", &targetUser)
	if err := Getid(bot, testReplyContext(bot, "/getid", testUser(100), replyTo)); err != nil {
		t.Fatalf("Getid (reply): %v", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("Getid (reply) made %d sendMessage calls, want 1", len(calls))
	}
	if calls[0].Params["chat_id"] != int64(100) {
		t.Errorf("Getid (reply) chat_id = %v, want private chat of the admin (100)", calls[0].Params["chat_id"])
	}
	if !strings.Contains(calls[0].Params["text"].(string), "UserID: 777") {
		t.Errorf("Getid (reply) text = %q, want info about user 777", calls[0].Params["text"])
	}

	// Admin with @username argument -> looked up in DB.
	client.Calls = nil
	if err := Getid(bot, testContext(bot, "/getid @target", testUser(100))); err != nil {
		t.Fatalf("Getid (arg): %v", err)
	}
	calls = client.callsFor("sendMessage")
	if len(calls) != 1 || !strings.Contains(calls[0].Params["text"].(string), "UserID: 777") {
		t.Errorf("Getid (arg) calls = %v, want info about user 777", calls)
	}

	// Admin without argument -> own info.
	client.Calls = nil
	if err := Getid(bot, testContext(bot, "/getid", testUser(100))); err != nil {
		t.Fatalf("Getid (self): %v", err)
	}
	calls = client.callsFor("sendMessage")
	if len(calls) != 1 || !strings.Contains(calls[0].Params["text"].(string), "UserID: 100") {
		t.Errorf("Getid (self) calls = %v, want info about user 100", calls)
	}
}

func TestSlap(t *testing.T) {
	bot, client := setupTestEnv(t)

	// Regular user slaps the replied-to user.
	replyTo := testMessage("что-то", &targetUser)
	if err := Slap(bot, testReplyContext(bot, "/slap", testUser(42), replyTo)); err != nil {
		t.Fatalf("Slap: %v", err)
	}
	texts := client.sentTexts()
	want := `👋 <b>Тест Пупкин</b> дал леща <a href="tg://user?id=777">Цель Целевич</a>`
	if len(texts) != 1 || texts[0] != want {
		t.Errorf("Slap sent %q, want %q", texts, want)
	}

	// Admin gets the "отцовский" variant.
	client.Calls = nil
	if err := Slap(bot, testReplyContext(bot, "/slap", testUser(100), replyTo)); err != nil {
		t.Fatalf("Slap (admin): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "дал отцовского леща") {
		t.Errorf("Slap (admin) sent %q, want отцовский variant", texts)
	}

	// No target found -> error.
	if err := Slap(bot, testContext(bot, "/slap", testUser(42))); err == nil {
		t.Error("Slap without target: expected error, got nil")
	}
}

func TestRemoveReplyMessage(t *testing.T) {
	bot, client := setupTestEnv(t)

	// Non-admin -> Denied.
	if err := RemoveReplyMessage(bot, testContext(bot, "/remove", testUser(42))); err != nil {
		t.Fatalf("RemoveReplyMessage (non-admin): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || texts[0] != "нет" {
		t.Errorf("RemoveReplyMessage (non-admin) sent %q, want Nope text", texts)
	}

	// Admin -> both the command message and the replied-to message are deleted.
	client.Calls = nil
	replyTo := testMessage("мишень", &targetUser)
	if err := RemoveReplyMessage(bot, testReplyContext(bot, "/remove", testUser(100), replyTo)); err != nil {
		t.Fatalf("RemoveReplyMessage: %v", err)
	}
	if n := len(client.callsFor("deleteMessage")); n != 2 {
		t.Errorf("RemoveReplyMessage: deleteMessage calls = %d, want 2", n)
	}
}

func TestWarnUser(t *testing.T) {
	bot, client := setupTestEnv(t)

	// Non-admin -> Denied.
	if err := WarnUser(bot, testContext(bot, "/warn 777", testUser(42))); err != nil {
		t.Fatalf("WarnUser (non-admin): %v", err)
	}
	if texts := client.sentTexts(); len(texts) != 1 || texts[0] != "нет" {
		t.Errorf("WarnUser (non-admin) sent %q, want Nope text", texts)
	}

	// First warning.
	client.Calls = nil
	replyTo := testMessage("проступок", &targetUser)
	if err := WarnUser(bot, testReplyContext(bot, "/warn", testUser(100), replyTo)); err != nil {
		t.Fatalf("WarnUser (1st): %v", err)
	}
	var warn Warn
	if err := DB.First(&warn, targetUser.Id).Error; err != nil {
		t.Fatalf("WarnUser (1st): warn row not created: %v", err)
	}
	if warn.Amount != 1 {
		t.Errorf("WarnUser (1st): Amount = %d, want 1", warn.Amount)
	}
	texts := client.sentTexts()
	if len(texts) != 2 || !strings.Contains(texts[0], "у тебя 1 предупреждение") || !strings.Contains(texts[1], "1 из 3") {
		t.Errorf("WarnUser (1st) sent %q, want warning to user + report to moderator", texts)
	}
	// The private message spells out what happens next: "Если", not "Есль".
	if len(texts) != 2 || !strings.Contains(texts[0], "Если получишь 3 предупреждения за 2 недели") {
		t.Errorf("WarnUser (1st) warning text = %q, want the 3-warnings notice", texts)
	}

	// Second warning.
	client.Calls = nil
	if err := WarnUser(bot, testReplyContext(bot, "/warn", testUser(100), replyTo)); err != nil {
		t.Fatalf("WarnUser (2nd): %v", err)
	}
	if err := DB.First(&warn, targetUser.Id).Error; err != nil || warn.Amount != 2 {
		t.Errorf("WarnUser (2nd): Amount = %d (err %v), want 2", warn.Amount, err)
	}
	// "в течение недели" - the preposition takes the genitive.
	texts = client.sentTexts()
	if len(texts) != 2 || !strings.Contains(texts[0], "у тебя 2 предупреждения") ||
		!strings.Contains(texts[0], "Если в течение недели") || !strings.Contains(texts[1], "2 из 3") {
		t.Errorf("WarnUser (2nd) sent %q, want warning to user + report to moderator", texts)
	}

	// Third warning -> mute via RestrictChatMember.
	client.Calls = nil
	if err := WarnUser(bot, testReplyContext(bot, "/warn", testUser(100), replyTo)); err != nil {
		t.Fatalf("WarnUser (3rd): %v", err)
	}
	restricts := client.callsFor("restrictChatMember")
	if len(restricts) != 1 {
		t.Fatalf("WarnUser (3rd): restrictChatMember calls = %d, want 1", len(restricts))
	}
	if restricts[0].Params["chat_id"] != int64(-100) || restricts[0].Params["user_id"] != targetUser.Id {
		t.Errorf("WarnUser (3rd): restricted %v/%v, want chat -100 user 777",
			restricts[0].Params["chat_id"], restricts[0].Params["user_id"])
	}
	// The mute lasts a week.
	assertUntilDate(t, restricts[0].Params, time.Now().Add(168*time.Hour))
	// A week of mute is counted into the mute-time stats (stat type 7).
	if got := muteSeconds(t, targetUser.Id); got != 168*3600 {
		t.Errorf("WarnUser (3rd): mute seconds = %d, want %d", got, 168*3600)
	}
	texts = client.sentTexts()
	if len(texts) < 2 || !strings.Contains(texts[0], "ты набрал 3 предупреждения") {
		t.Errorf("WarnUser (3rd) sent %q, want the 3-warnings notice to the user", texts)
	}
	if len(texts) == 0 || !strings.Contains(texts[len(texts)-1], "замьючен") {
		t.Errorf("WarnUser (3rd) sent %q, want mute report", texts)
	}

	// Warnings decay by one per full week since the last one, and /warn applies
	// the decay before counting: a 15 days old counter of 2 is a single warning.
	if err := DB.Save(&Warn{UserID: targetUser.Id, Amount: 2, LastWarn: time.Now().Add(-15 * 24 * time.Hour)}).Error; err != nil {
		t.Fatalf("WarnUser (decay): seed: %v", err)
	}
	client.Calls = nil
	if err := WarnUser(bot, testReplyContext(bot, "/warn", testUser(100), replyTo)); err != nil {
		t.Fatalf("WarnUser (decay): %v", err)
	}
	if err := DB.First(&warn, targetUser.Id).Error; err != nil || warn.Amount != 1 {
		t.Errorf("WarnUser (decay): Amount = %d (err %v), want 1", warn.Amount, err)
	}
	texts = client.sentTexts()
	if len(texts) != 2 || !strings.Contains(texts[0], "у тебя 1 предупреждение") || !strings.Contains(texts[1], "1 из 3") {
		t.Errorf("WarnUser (decay) sent %q, want the counter back at 1", texts)
	}
}

func TestMywarns(t *testing.T) {
	bot, client := setupTestEnv(t)

	// No warnings.
	if err := Mywarns(bot, testContext(bot, "/mywarns", testUser(42))); err != nil {
		t.Fatalf("Mywarns: %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || texts[0] != "У тебя 0 предупреждений." {
		t.Errorf("Mywarns sent %q, want %q", texts, "У тебя 0 предупреждений.")
	}

	// Existing warnings: the noun is declined by ruPlural. The x/text printer on
	// its own would always print the "other" form (see TestRussianPlurals).
	client.Calls = nil
	DB.Create(&Warn{UserID: 42, Amount: 2, LastWarn: time.Now()})
	if err := Mywarns(bot, testContext(bot, "/mywarns", testUser(42))); err != nil {
		t.Fatalf("Mywarns: %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || texts[0] != "У тебя 2 предупреждения." {
		t.Errorf("Mywarns sent %q, want %q", texts, "У тебя 2 предупреждения.")
	}

	// Warnings decay by one per full week since the last one: 3 warnings from
	// 8 days ago are reported as 2.
	client.Calls = nil
	if err := DB.Save(&Warn{UserID: 42, Amount: 3, LastWarn: time.Now().Add(-8 * 24 * time.Hour)}).Error; err != nil {
		t.Fatalf("Mywarns (decay): seed: %v", err)
	}
	if err := Mywarns(bot, testContext(bot, "/mywarns", testUser(42))); err != nil {
		t.Fatalf("Mywarns (decay): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || texts[0] != "У тебя 2 предупреждения." {
		t.Errorf("Mywarns (decay) sent %q, want the decayed amount 2", texts)
	}
}

// TestRussianPlurals documents the plural situation: golang.org/x/text/message
// (the "prt" printer used across the commands) has no Russian plural rules, so
// NewPrinter with language.Russian falls back to the literal format string and
// "2 предупреждений" is never corrected. ruPlural is what declines the nouns.
func TestRussianPlurals(t *testing.T) {
	prt := message.NewPrinter(language.Russian)
	if got := prt.Sprintf("У тебя %d предупреждений.", 2); got != "У тебя 2 предупреждений." {
		t.Errorf("x/text Russian printer changed the string: %q", got)
	}

	cases := []struct {
		count int64
		want  string
	}{
		{0, "предупреждений"},
		{1, "предупреждение"},
		{2, "предупреждения"},
		{4, "предупреждения"},
		{5, "предупреждений"},
		{11, "предупреждений"},
		{14, "предупреждений"},
		{21, "предупреждение"},
		{22, "предупреждения"},
		{101, "предупреждение"},
		{111, "предупреждений"},
	}
	for _, c := range cases {
		if got := ruPlural(c.count, "предупреждение", "предупреждения", "предупреждений"); got != c.want {
			t.Errorf("ruPlural(%d) = %q, want %q", c.count, got, c.want)
		}
	}

	// int counters (Warn.Amount, Duelist.Deaths, mute duration) work as well.
	if got := ruPlural(3, "раз", "раза", "раз"); got != "раза" {
		t.Errorf("ruPlural(int 3) = %q, want %q", got, "раза")
	}
	if got := ruPlural(-1, "смерть", "смерти", "смертей"); got != "смерть" {
		t.Errorf("ruPlural(-1) = %q, want %q", got, "смерть")
	}
}

// TestFormatDuration covers the mute-time formatting of /stats mutetime: units are
// declined and zero units are dropped.
func TestFormatDuration(t *testing.T) {
	cases := []struct {
		seconds int64
		want    string
	}{
		{0, "0 секунд"},
		{1, "1 секунда"},
		{45, "45 секунд"},
		{60, "1 минута"},
		{90, "1 минута 30 секунд"},
		{120, "2 минуты"},
		{3600, "1 час"},
		{3725, "1 час 2 минуты 5 секунд"},
		{86400, "1 день"},
		{90061, "1 день 1 час 1 минута 1 секунда"},
		{90000, "1 день 1 час"},
		{172800, "2 дня"},
		{259200, "3 дня"},
	}
	for _, c := range cases {
		if got := formatDuration(c.seconds); got != c.want {
			t.Errorf("formatDuration(%d) = %q, want %q", c.seconds, got, c.want)
		}
	}
}

func TestDel(t *testing.T) {
	bot, client := setupTestEnv(t)
	owner := gotgbot.User{Id: 555, FirstName: "Владелец", Username: "owner555"}
	seedUser(owner.Id, owner.Username, owner.FirstName)
	DB.Create(&Get{Name: "тестget", Data: "данные", Creator: owner.Id})

	// No argument -> usage message.
	if err := Del(bot, testContext(bot, "/del", testUser(42))); err != nil {
		t.Fatalf("Del (no args): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Пример использования") {
		t.Errorf("Del (no args) sent %q, want usage message", texts)
	}

	// Unknown get.
	client.Calls = nil
	if err := Del(bot, testContext(bot, "/del неттакого", testUser(42))); err != nil {
		t.Fatalf("Del (unknown): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "не найден") {
		t.Errorf("Del (unknown) sent %q, want not-found message", texts)
	}

	// Non-owner non-admin -> ownership message, row kept.
	client.Calls = nil
	if err := Del(bot, testContext(bot, "/del тестget", testUser(42))); err != nil {
		t.Fatalf("Del (non-owner): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "либо администраторы, либо Владелец") {
		t.Errorf("Del (non-owner) sent %q, want ownership message", texts)
	}
	var get Get
	if err := DB.First(&get, "name = ?", "тестget").Error; err != nil {
		t.Errorf("Del (non-owner): get row should still exist: %v", err)
	}

	// Owner deletes it.
	client.Calls = nil
	if err := Del(bot, testContext(bot, "/del тестget", testUser(555))); err != nil {
		t.Fatalf("Del (owner): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "удалён") {
		t.Errorf("Del (owner) sent %q, want deleted message", texts)
	}
	if err := DB.First(&get, "name = ?", "тестget").Error; err == nil {
		t.Error("Del (owner): get row should be gone")
	}

	// Admin can delete anyone's get.
	client.Calls = nil
	if err := Del(bot, testContext(bot, "/del тестget2", testUser(100))); err != nil {
		t.Fatalf("Del (admin): %v", err)
	}
	DB.Create(&Get{Name: "тестget2", Creator: owner.Id})
	if err := Del(bot, testContext(bot, "/del тестget2", testUser(100))); err != nil {
		t.Fatalf("Del (admin): %v", err)
	}
	if err := DB.First(&get, "name = ?", "тестget2").Error; err == nil {
		t.Error("Del (admin): get row should be gone")
	}
}
