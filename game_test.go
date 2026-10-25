package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// resetDuelState clears the duel globals (busy map, cached Message) for a
// test and restores them afterwards.
func resetDuelState(t *testing.T) {
	t.Helper()
	oldMessage, oldBusy := Message, busy
	t.Cleanup(func() { Message, busy = oldMessage, oldBusy })
	Message = nil
	busy = make(map[string]bool)
}

// seedDuelInvitation models the state created by Request for callback tests.
func seedDuelInvitation(msg *gotgbot.Message) {
	duelMu.Lock()
	defer duelMu.Unlock()
	copy := *msg
	copy.Date = time.Now().Unix()
	Message = &copy
	busy["russianroulettePending"] = true
}

// ---------- Пидор Дня ----------

func TestPidoreg(t *testing.T) {
	bot, client := setupTestEnv(t)

	if err := Pidoreg(bot, testContext(bot, "/pidoreg", testUser(42))); err != nil {
		t.Fatalf("Pidoreg: %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Ты теперь участвуешь") {
		t.Errorf("Pidoreg sent %q, want registration confirmation", texts)
	}
	var pidor PidorList
	if err := DB.First(&pidor, 42).Error; err != nil {
		t.Errorf("Pidoreg: user not saved to pidor list: %v", err)
	}

	// Re-registration is rejected.
	client.Calls = nil
	if err := Pidoreg(bot, testContext(bot, "/pidoreg", testUser(42))); err != nil {
		t.Fatalf("Pidoreg (dup): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || texts[0] != "Эй, ты уже в игре!" {
		t.Errorf("Pidoreg (dup) sent %q, want %q", texts, "Эй, ты уже в игре!")
	}
}

func TestPidorme(t *testing.T) {
	bot, client := setupTestEnv(t)

	// No stats at all.
	if err := Pidorme(bot, testContext(bot, "/pidorme", testUser(42))); err != nil {
		t.Fatalf("Pidorme: %v", err)
	}
	texts := client.sentTexts()
	want := "В этом году ты был пидором дня — 0 раз\nЗа всё время ты был пидором дня — 0 раз!"
	if len(texts) != 1 || texts[0] != want {
		t.Errorf("Pidorme sent %q, want %q", texts, want)
	}

	// One win this year.
	client.Calls = nil
	DB.Create(&PidorStats{UserID: 42, Date: time.Now()})
	if err := Pidorme(bot, testContext(bot, "/pidorme", testUser(42))); err != nil {
		t.Fatalf("Pidorme: %v", err)
	}
	texts = client.sentTexts()
	want = "В этом году ты был пидором дня — 1 раз\nЗа всё время ты был пидором дня — 1 раз!"
	if len(texts) != 1 || texts[0] != want {
		t.Errorf("Pidorme sent %q, want %q", texts, want)
	}
}

func TestPidorlist(t *testing.T) {
	bot, client := setupTestEnv(t)

	// Non-admin -> Denied.
	if err := Pidorlist(bot, testContext(bot, "/pidorlist", testUser(42))); err != nil {
		t.Fatalf("Pidorlist (non-admin): %v", err)
	}
	if texts := client.sentTexts(); len(texts) != 1 || texts[0] != "нет" {
		t.Errorf("Pidorlist (non-admin) sent %q, want Nope text", texts)
	}

	// Admin gets the numbered list.
	client.Calls = nil
	DB.Create(&PidorList{Id: 777, Username: "target", FirstName: "Цель"})
	DB.Create(&PidorList{Id: 888, Username: "second", FirstName: "Второй"})
	if err := Pidorlist(bot, testContext(bot, "/pidorlist", testUser(100))); err != nil {
		t.Fatalf("Pidorlist: %v", err)
	}
	texts := client.sentTexts()
	want := "1. @target (777)\n2. @second (888)\n"
	if len(texts) != 1 || texts[0] != want {
		t.Errorf("Pidorlist sent %q, want %q", texts, want)
	}
}

func TestPidordel(t *testing.T) {
	bot, client := setupTestEnv(t)

	// Non-admin -> Denied.
	if err := Pidordel(bot, testContext(bot, "/pidordel 777", testUser(42))); err != nil {
		t.Fatalf("Pidordel (non-admin): %v", err)
	}
	if texts := client.sentTexts(); len(texts) != 1 || texts[0] != "нет" {
		t.Errorf("Pidordel (non-admin) sent %q, want Nope text", texts)
	}

	// Admin removes a registered player (via reply).
	client.Calls = nil
	DB.Create(&PidorList{Id: 777, Username: "target", FirstName: "Цель"})
	replyTo := testMessage("сообщение", &targetUser)
	if err := Pidordel(bot, testReplyContext(bot, "/pidordel", testUser(100), replyTo)); err != nil {
		t.Fatalf("Pidordel: %v", err)
	}
	texts := client.sentTexts()
	want := "Пользователь <a href=\"tg://user?id=777\">Цель Целевич</a> удалён из игры <b>Пидор Дня</b>!"
	if len(texts) != 1 || texts[0] != want {
		t.Errorf("Pidordel sent %q, want %q", texts, want)
	}
	var pidor PidorList
	if err := DB.First(&pidor, 777).Error; err == nil {
		t.Error("Pidordel: pidor row should be gone")
	}

	// Admin removes an unregistered player: used to panic on nil result.Error.
	client.Calls = nil
	if err := Pidordel(bot, testReplyContext(bot, "/pidordel", testUser(100), replyTo)); err != nil {
		t.Fatalf("Pidordel (unregistered): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Не удалось удалить пользователя") {
		t.Errorf("Pidordel (unregistered) sent %q, want failure message", texts)
	}
}

func TestPidorstats(t *testing.T) {
	bot, client := setupTestEnv(t)
	DB.Create(&PidorList{Id: 777, Username: "target", FirstName: "Цель"})
	for i := 0; i < 3; i++ {
		DB.Create(&PidorStats{UserID: 777, Date: time.Now()})
	}

	// No argument -> current year top.
	if err := Pidorstats(bot, testContext(bot, "/pidorstats", testUser(42))); err != nil {
		t.Fatalf("Pidorstats: %v", err)
	}
	texts := client.sentTexts()
	year := strconv.Itoa(time.Now().Year())
	if len(texts) != 1 || !strings.Contains(texts[0], "Топ-10 пидоров за "+year+" год") ||
		!strings.Contains(texts[0], "1. target - 3 раза") || !strings.Contains(texts[0], "Всего участников — 1") {
		t.Errorf("Pidorstats sent %q, want current-year top with target - 3", texts)
	}

	// Past year argument (used to always fail: Atoi was applied to the whole message text).
	client.Calls = nil
	if err := Pidorstats(bot, testContext(bot, "/pidorstats 2019", testUser(42))); err != nil {
		t.Fatalf("Pidorstats (2019): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Топ-10 пидоров за 2019 год") {
		t.Errorf("Pidorstats (2019) sent %q, want 2019 header", texts)
	}

	// 2077 easter egg -> video.
	client.Calls = nil
	if err := Pidorstats(bot, testContext(bot, "/pidorstats 2077", testUser(42))); err != nil {
		t.Fatalf("Pidorstats (2077): %v", err)
	}
	if n := len(client.callsFor("sendVideo")); n != 1 {
		t.Errorf("Pidorstats (2077): sendVideo calls = %d, want 1", n)
	}

	// Unparseable year -> error.
	if err := Pidorstats(bot, testContext(bot, "/pidorstats abc", testUser(42))); err == nil {
		t.Error("Pidorstats (abc): expected error, got nil")
	}
}

func TestPidorall(t *testing.T) {
	bot, client := setupTestEnv(t)
	DB.Create(&PidorList{Id: 777, Username: "target", FirstName: "Цель"})
	// Old dates: Pidorall has no date filter, Pidorstats would exclude these.
	DB.Create(&PidorStats{UserID: 777, Date: time.Date(2019, 5, 5, 12, 0, 0, 0, time.Local)})
	DB.Create(&PidorStats{UserID: 777, Date: time.Date(2020, 5, 5, 12, 0, 0, 0, time.Local)})

	if err := Pidorall(bot, testContext(bot, "/pidorall", testUser(42))); err != nil {
		t.Fatalf("Pidorall: %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Топ-10 пидоров за всё время") ||
		!strings.Contains(texts[0], "1. target - 2 раза") || !strings.Contains(texts[0], "Всего участников — 1") {
		t.Errorf("Pidorall sent %q, want all-time top with target - 2", texts)
	}
}

func TestPidorules(t *testing.T) {
	bot, client := setupTestEnv(t)
	if err := Pidorules(bot, testContext(bot, "/pidorules", testUser(42))); err != nil {
		t.Fatalf("Pidorules: %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Правила игры <b>Пидор Дня</b>") {
		t.Errorf("Pidorules sent %q, want rules text", texts)
	}
}

func TestPidorRemoveToday(t *testing.T) {
	bot, client := setupTestEnv(t)

	// Non-admin -> Denied.
	if err := PidorRemoveToday(bot, testContext(bot, "/pidorremove", testUser(42))); err != nil {
		t.Fatalf("PidorRemoveToday (non-admin): %v", err)
	}
	if texts := client.sentTexts(); len(texts) != 1 || texts[0] != "нет" {
		t.Errorf("PidorRemoveToday (non-admin) sent %q, want Nope text", texts)
	}

	// No draw today.
	client.Calls = nil
	if err := PidorRemoveToday(bot, testContext(bot, "/pidorremove", testUser(100))); err != nil {
		t.Fatalf("PidorRemoveToday: %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || texts[0] != "Сегодня пидора дня ещё не было." {
		t.Errorf("PidorRemoveToday sent %q, want %q", texts, "Сегодня пидора дня ещё не было.")
	}

	// Draw today -> row removed.
	client.Calls = nil
	DB.Create(&PidorStats{UserID: 777, Date: time.Now()})
	if err := PidorRemoveToday(bot, testContext(bot, "/pidorremove", testUser(100))); err != nil {
		t.Fatalf("PidorRemoveToday: %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || texts[0] != "Текущий пидор дня больше не пидор!" {
		t.Errorf("PidorRemoveToday sent %q, want removal confirmation", texts)
	}
	var count int64
	DB.Model(&PidorStats{}).Count(&count)
	if count != 0 {
		t.Errorf("PidorRemoveToday: %d pidor stats rows left, want 0", count)
	}
}

func TestPidor(t *testing.T) {
	bot, client := setupTestEnv(t)

	// Private chat -> no-op.
	private := testMessage("/pidor", testUser(42))
	private.Chat.Type = "private"
	if err := Pidor(bot, ext.NewContext(bot, &gotgbot.Update{UpdateId: 1, Message: private}, nil)); err != nil {
		t.Fatalf("Pidor (private): %v", err)
	}
	if n := len(client.Calls); n != 0 {
		t.Errorf("Pidor (private): %d API calls made, want 0", n)
	}

	// First draw of the day: picks a random registered player and posts the sequence.
	DB.Create(&PidorList{Id: 777, Username: "target", FirstName: "Цель"})
	if err := Pidor(bot, testContext(bot, "/pidor", testUser(42))); err != nil {
		t.Fatalf("Pidor: %v", err)
	}
	if n := len(client.callsFor("sendMessage")); n != 1 {
		t.Errorf("Pidor: sendMessage calls = %d, want 1 (edits happen in background goroutines)", n)
	}
	var statsCount int64
	DB.Model(&PidorStats{}).Count(&statsCount)
	if statsCount != 1 {
		t.Errorf("Pidor: pidor stats rows = %d, want 1", statsCount)
	}
	var stats PidorStats
	DB.Model(&PidorStats{}).First(&stats)
	if stats.UserID != 777 {
		t.Errorf("Pidor: drawn user = %d, want 777", stats.UserID)
	}

	// Second call the same day -> reports the existing draw.
	client.Calls = nil
	if err := Pidor(bot, testContext(bot, "/pidor", testUser(42))); err != nil {
		t.Fatalf("Pidor (second): %v", err)
	}
	texts := client.sentTexts()
	want := "Согласно моей информации, по результатам сегодняшнего розыгрыша пидор дня - target!"
	if len(texts) != 1 || texts[0] != want {
		t.Errorf("Pidor (second) sent %q, want %q", texts, want)
	}
}

func TestPidorDrawnUserLeftChat(t *testing.T) {
	bot, client := setupTestEnv(t)
	DB.Create(&PidorList{Id: 777, Username: "target", FirstName: "Цель"})
	client.Results = map[string]json.RawMessage{
		"getChatMember": json.RawMessage(`{"status":"left","user":{"id":777,"is_bot":false,"first_name":"Цель"}}`),
	}

	if err := Pidor(bot, testContext(bot, "/pidor", testUser(42))); err != nil {
		t.Fatalf("Pidor: %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "вышел из этого чата") {
		t.Errorf("Pidor sent %q, want left-chat message", texts)
	}
	var pidor PidorList
	if err := DB.First(&pidor, 777).Error; err == nil {
		t.Error("Pidor: left user should be removed from the game")
	}
}

func TestPidorGetChatMemberError(t *testing.T) {
	bot, client := setupTestEnv(t)
	DB.Create(&PidorList{Id: 777, Username: "target", FirstName: "Цель"})
	client.Results = map[string]json.RawMessage{
		"getChatMember": json.RawMessage(`not json`), // forces a parse error in gotgbot
	}

	if err := Pidor(bot, testContext(bot, "/pidor", testUser(42))); err != nil {
		t.Fatalf("Pidor: %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "что-то не так") {
		t.Errorf("Pidor sent %q, want broken-user message", texts)
	}
	var pidor PidorList
	if err := DB.First(&pidor, 777).Error; err == nil {
		t.Error("Pidor: broken user should be removed from the game")
	}
}

// ---------- Дуэль (русская рулетка) ----------

func TestDuelRequest(t *testing.T) {
	bot, client := setupTestEnv(t)
	resetDuelState(t)

	// Usage message when misused.
	if err := Request(bot, testContext(bot, "/russianroulette", testUser(42))); err != nil {
		t.Fatalf("Request (usage): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Пример использования") {
		t.Errorf("Request (usage) sent %q, want usage message", texts)
	}

	// Self-challenge rejected.
	client.Calls = nil
	own := testMessage("своё", testUser(42))
	if err := Request(bot, testReplyContext(bot, "/russianroulette", testUser(42), own)); err != nil {
		t.Fatalf("Request (self): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "нельзя вызвать на дуэль самого себя") {
		t.Errorf("Request (self) sent %q, want self-challenge rejection", texts)
	}

	// Bot target rejected.
	client.Calls = nil
	botMsg := testMessage("ботовское", &gotgbot.User{Id: 999, IsBot: true, FirstName: "Bot"})
	if err := Request(bot, testReplyContext(bot, "/russianroulette", testUser(42), botMsg)); err != nil {
		t.Fatalf("Request (bot): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || texts[0] != "Бота нельзя вызвать на дуэль." {
		t.Errorf("Request (bot) sent %q, want bot rejection", texts)
	}

	// Successful challenge: command removed, duel message with buttons sent.
	client.Calls = nil
	replyTo := testMessage("мишень", &targetUser)
	if err := Request(bot, testReplyContext(bot, "/russianroulette", testUser(42), replyTo)); err != nil {
		t.Fatalf("Request: %v", err)
	}
	if n := len(client.callsFor("deleteMessage")); n != 1 {
		t.Errorf("Request: deleteMessage calls = %d, want 1", n)
	}
	sends := client.callsFor("sendMessage")
	if len(sends) != 1 {
		t.Fatalf("Request: sendMessage calls = %d, want 1", len(sends))
	}
	want := "<a href=\"tg://user?id=777\">Цель Целевич</a>! <a href=\"tg://user?id=42\">Тест Пупкин</a> вызывает тебя на дуэль!"
	if sends[0].Params["text"] != want {
		t.Errorf("Request text = %q, want %q", sends[0].Params["text"], want)
	}
	if sends[0].Params["reply_markup"] == nil {
		t.Error("Request: duel message has no inline keyboard")
	}
	if !busy["russianroulettePending"] {
		t.Error("Request: duel should be left pending")
	}
}

func TestDuelRequestTimeout(t *testing.T) {
	bot, client := setupTestEnv(t)
	resetDuelState(t)

	// A pending duel older than 60s is cancelled with an edit.
	Message = testMessage("приглашение", testUser(42))
	Message.Entities = []gotgbot.MessageEntity{{Type: "text_mention", User: &targetUser}}
	Message.Date = 0
	busy["russianroulettePending"] = true

	if err := Request(bot, testContext(bot, "/russianroulette", testUser(42))); err != nil {
		t.Fatalf("Request (timeout): %v", err)
	}
	edits := client.callsFor("editMessageText")
	if len(edits) != 1 || edits[0].Params["text"] != "Цель Целевич не пришел на дуэль." {
		t.Errorf("Request (timeout) edits = %v, want timeout cancellation", edits)
	}
	if busy["russianroulettePending"] || busy["russianrouletteInProgress"] {
		t.Errorf("Request (timeout): busy flags not cleared: %v", busy)
	}
}

func TestDuelRequestBusyStates(t *testing.T) {
	bot, client := setupTestEnv(t)
	resetDuelState(t)

	// Bot marked dead -> refuses to run.
	Message = testMessage("сообщение", testUser(42))
	Message.Date = time.Now().Unix()
	busy["bot_is_dead"] = true
	if err := Request(bot, testContext(bot, "/russianroulette", testUser(42))); err != nil {
		t.Fatalf("Request (dead): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "немного умер") {
		t.Errorf("Request (dead) sent %q, want dead-bot message", texts)
	}

	// Any busy flag set -> "command busy".
	client.Calls = nil
	busy["bot_is_dead"] = false
	busy["russianroulette"] = true
	if err := Request(bot, testContext(bot, "/russianroulette", testUser(42))); err != nil {
		t.Fatalf("Request (busy): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || texts[0] != "Команда занята. Попробуйте позже." {
		t.Errorf("Request (busy) sent %q, want busy message", texts)
	}
}

func TestDuelDeny(t *testing.T) {
	bot, client := setupTestEnv(t)
	resetDuelState(t)

	player := gotgbot.User{Id: 42, FirstName: "Тест", LastName: "Пупкин", Username: "testpupkin"}
	msg := testMessage("вызов", &player)
	msg.Entities = []gotgbot.MessageEntity{{Type: "text_mention", User: &targetUser}, {Type: "text_mention", User: &player}}
	seedDuelInvitation(msg)
	busy["russianroulettePending"] = true

	// Someone else presses deny -> answered with a Nope, duel stays pending.
	ctx := testCallbackContext(bot, msg, &player)
	if err := Deny(bot, ctx); err != nil {
		t.Fatalf("Deny (wrong presser): %v", err)
	}
	answers := client.callsFor("answerCallbackQuery")
	if len(answers) != 1 || answers[0].Params["text"] != "нет" {
		t.Errorf("Deny (wrong presser) answers = %v, want Nope answer", answers)
	}
	if !busy["russianroulettePending"] {
		t.Error("Deny (wrong presser): duel should stay pending")
	}

	// The challenged user presses deny -> duel cancelled, message edited.
	client.Calls = nil
	ctx = testCallbackContext(bot, msg, &targetUser)
	if err := Deny(bot, ctx); err != nil {
		t.Fatalf("Deny: %v", err)
	}
	answers = client.callsFor("answerCallbackQuery")
	if len(answers) != 1 || answers[0].Params["text"] != nil {
		t.Errorf("Deny answers = %v, want plain answer without text", answers)
	}
	if busy["russianroulettePending"] || busy["russianrouletteInProgress"] || busy["russianroulette"] {
		t.Errorf("Deny: busy flags not cleared: %v", busy)
	}
	edits := client.callsFor("editMessageText")
	if len(edits) != 1 || edits[0].Params["text"] != "Цель Целевич отказался от дуэли." {
		t.Errorf("Deny edits = %v, want refusal edit", edits)
	}
}

func TestDuelstats(t *testing.T) {
	bot, client := setupTestEnv(t)

	// No row.
	if err := Duelstats(bot, testContext(bot, "/duelstats", testUser(42))); err != nil {
		t.Fatalf("Duelstats: %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || texts[0] != "У тебя нет статистики." {
		t.Errorf("Duelstats sent %q, want %q", texts, "У тебя нет статистики.")
	}

	// Existing row (x/text prints the literal "other" plural form).
	client.Calls = nil
	DB.Create(&Duelist{UserID: 42, Kills: 2, Deaths: 1})
	if err := Duelstats(bot, testContext(bot, "/duelstats", testUser(42))); err != nil {
		t.Fatalf("Duelstats: %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || texts[0] != "У тебя 2 победы, 1 смерть и 0 спасений" {
		t.Errorf("Duelstats sent %q, want %q", texts, "У тебя 2 победы, 1 смерть и 0 спасений")
	}
}

// TestDuelAcceptFull runs the whole sequence with controlled randomness and
// no animation delay. Both duelists are regular members.
func TestDuelAcceptFull(t *testing.T) {
	bot, client := setupTestEnv(t)
	resetDuelState(t)

	player := gotgbot.User{Id: 42, FirstName: "Тест", LastName: "Пупкин", Username: "testpupkin"}
	msg := testMessage("вызов", &player)
	msg.Entities = []gotgbot.MessageEntity{{Type: "text_mention", User: &targetUser}, {Type: "text_mention", User: &player}}
	seedDuelInvitation(msg)

	// The challenged user (victim) accepts.
	ctx := testCallbackContext(bot, msg, &targetUser)
	if err := acceptDuelWith(bot, ctx, func(min, max int) int { return min }, func(time.Duration) {}); err != nil {
		t.Fatalf("Accept: %v", err)
	}

	answers := client.callsFor("answerCallbackQuery")
	if len(answers) != 1 {
		t.Errorf("Accept: answerCallbackQuery calls = %d, want 1", len(answers))
	}
	edits := client.callsFor("editMessageText")
	if len(edits) < 6 {
		t.Errorf("Accept: editMessageText calls = %d, want at least 6", len(edits))
	}
	final := edits[len(edits)-1].Params["text"].(string)
	if !strings.Contains(final, "Победитель дуэли") || !strings.Contains(final, "отправился на респавн") {
		t.Errorf("Accept final edit = %q, want winner + respawn text", final)
	}
	assertMinutesDeclined(t, final)
	restricts := client.callsFor("restrictChatMember")
	if len(restricts) != 1 {
		t.Fatalf("Accept: restrictChatMember calls = %d, want 1", len(restricts))
	}

	// Exactly one duelist died and the other got the kill.
	var duelists []Duelist
	DB.Find(&duelists)
	if len(duelists) != 2 {
		t.Fatalf("Accept: duelist rows = %d, want 2 (%v)", len(duelists), duelists)
	}
	var deaths, kills int
	for _, d := range duelists {
		deaths += d.Deaths
		kills += d.Kills
		if d.UserID != 42 && d.UserID != 777 {
			t.Errorf("Accept: unexpected duelist %v", d)
		}
	}
	if deaths != 1 || kills != 1 {
		t.Errorf("Accept: total deaths = %d kills = %d, want 1 and 1 (duelists: %v)", deaths, kills, duelists)
	}
	// The duelist who died owns the mute time (stat type 7), the winner none.
	want := announcedSeconds(t, final)
	if total := muteSeconds(t, 42) + muteSeconds(t, 777); total != want {
		t.Errorf("Accept: total mute seconds = %d, want %d", total, want)
	}
	if muteSeconds(t, 42) != 0 && muteSeconds(t, 777) != 0 {
		t.Error("Accept: only the duelist who died should gain mute time")
	}
	// The death is duplicated into the stats (stat type 8) for the loser only.
	if total := statTotal(t, 8, 42) + statTotal(t, 8, 777); total != 1 {
		t.Errorf("Accept: total death stats = %d, want 1", total)
	}
	if statTotal(t, 8, 42) != 0 && statTotal(t, 8, 777) != 0 {
		t.Error("Accept: only the duelist who died should gain death stats")
	}
	if busy["russianrouletteInProgress"] {
		t.Error("Accept: duel should not stay in progress")
	}
	events, err := latestMuteEvents(Config.Chat)
	if err != nil || len(events) != 1 || events[0].Source != "duel" || !events[0].Muted ||
		events[0].UserID != restricts[0].Params["user_id"] || events[0].ActorID == events[0].UserID {
		t.Errorf("Accept: mute origin = %+v, err %v", events, err)
	}
}

// TestDuelAcceptNotParticipant covers the accept pressed by anyone who is not
// the challenged user: the callback is answered with a refusal from the pool,
// and the duel never starts.
func TestDuelAcceptNotParticipant(t *testing.T) {
	bot, client := setupTestEnv(t)
	resetDuelState(t)

	player := gotgbot.User{Id: 42, FirstName: "Тест", LastName: "Пупкин", Username: "testpupkin"}
	msg := testMessage("вызов", &player)
	msg.Entities = []gotgbot.MessageEntity{{Type: "text_mention", User: &targetUser}, {Type: "text_mention", User: &player}}
	seedDuelInvitation(msg)
	busy["russianroulettePending"] = true

	if err := Accept(bot, testCallbackContext(bot, msg, testUser(100))); err != nil {
		t.Fatalf("Accept (not a participant): %v", err)
	}

	answers := client.callsFor("answerCallbackQuery")
	if len(answers) != 1 || answers[0].Params["text"] != "нет" {
		t.Errorf("Accept (not a participant) answers = %v, want the Nope answer", answers)
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 0 {
		t.Errorf("Accept (not a participant): editMessageText calls = %d, want the duel not started", len(calls))
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
		t.Errorf("Accept (not a participant): restrictChatMember calls = %d, want nobody restricted", len(calls))
	}
	var duelists []Duelist
	DB.Find(&duelists)
	if len(duelists) != 0 {
		t.Errorf("Accept (not a participant): duelist rows = %v, want none", duelists)
	}
	if busy["russianrouletteInProgress"] {
		t.Error("Accept (not a participant): duel should not be in progress")
	}
	if !busy["russianroulettePending"] {
		t.Error("Accept (not a participant): the pending duel should stay pending")
	}
}

// ---------- Ставки ----------

func TestBet(t *testing.T) {
	bot, client := setupTestEnv(t)

	// Too few arguments.
	if err := Bet(bot, testContext(bot, "/bet 01.01.2027", testUser(42))); err != nil {
		t.Fatalf("Bet (usage): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Пример использования") {
		t.Errorf("Bet (usage) sent %q, want usage message", texts)
	}

	// Unparseable date.
	client.Calls = nil
	if err := Bet(bot, testContext(bot, "/bet 31.31.2027 foo bar", testUser(42))); err != nil {
		t.Fatalf("Bet (bad date): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Ошибка парсинга даты") {
		t.Errorf("Bet (bad date) sent %q, want parse error", texts)
	}

	// Past date.
	client.Calls = nil
	if err := Bet(bot, testContext(bot, "/bet 01.01.2020 foo bar", testUser(42))); err != nil {
		t.Fatalf("Bet (past date): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "минимальная дата") {
		t.Errorf("Bet (past date) sent %q, want minimum date message", texts)
	}

	// Valid bet.
	client.Calls = nil
	if err := Bet(bot, testContext(bot, "/bet 01.01.2027 foo bar", testUser(42))); err != nil {
		t.Fatalf("Bet: %v", err)
	}
	timestamp, err := time.Parse("02.01.2006", "01.01.2027")
	if err != nil {
		t.Fatalf("parse test date: %v", err)
	}
	want := "Ставка добавлена.\nДата: <code>" + timestamp.Local().Format("02.01.2006") + "</code>.\nТекст: <code>foo bar</code>."
	texts = client.sentTexts()
	if len(texts) != 1 || texts[0] != want {
		t.Errorf("Bet sent %q, want %q", texts, want)
	}
	var count int64
	DB.Model(&Bets{}).Where("user_id = ? AND text = ?", 42, "foo bar").Count(&count)
	if count != 1 {
		t.Errorf("Bet: bets rows = %d, want 1", count)
	}

	// Duplicate bet.
	client.Calls = nil
	if err := Bet(bot, testContext(bot, "/bet 01.01.2027 foo bar", testUser(42))); err != nil {
		t.Fatalf("Bet (dup): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || texts[0] != "Такая ставка уже добавлена" {
		t.Errorf("Bet (dup) sent %q, want duplicate rejection", texts)
	}
}

func TestDelBet(t *testing.T) {
	bot, client := setupTestEnv(t)

	// Too few arguments.
	if err := DelBet(bot, testContext(bot, "/delbet 01.01.2027", testUser(42))); err != nil {
		t.Fatalf("DelBet (usage): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Пример использования") {
		t.Errorf("DelBet (usage) sent %q, want usage message", texts)
	}

	// Missing bet.
	client.Calls = nil
	if err := DelBet(bot, testContext(bot, "/delbet 01.01.2027 foo bar", testUser(42))); err != nil {
		t.Fatalf("DelBet (missing): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || texts[0] != "Твоя ставка не найдена по указанным параметрам." {
		t.Errorf("DelBet (missing) sent %q, want not-found message", texts)
	}

	// Existing bet.
	client.Calls = nil
	timestamp, err := time.Parse("02.01.2006", "01.01.2027")
	if err != nil {
		t.Fatalf("parse test date: %v", err)
	}
	DB.Create(&Bets{UserID: 42, Text: "foo bar", Timestamp: timestamp.Unix()})
	if err := DelBet(bot, testContext(bot, "/delbet 01.01.2027 foo bar", testUser(42))); err != nil {
		t.Fatalf("DelBet: %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Ставка удалена") {
		t.Errorf("DelBet sent %q, want removal confirmation", texts)
	}
	var count int64
	DB.Model(&Bets{}).Count(&count)
	if count != 0 {
		t.Errorf("DelBet: bets rows = %d, want 0", count)
	}
}

func TestAllBets(t *testing.T) {
	bot, client := setupTestEnv(t)
	seedUser(42, "testpupkin", "Тест")

	// No bets in the default window -> empty reply.
	if err := AllBets(bot, testContext(bot, "/allbets", testUser(42))); err != nil {
		t.Fatalf("AllBets (empty): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || texts[0] != "" {
		t.Errorf("AllBets (empty) sent %q, want empty list", texts)
	}

	future, err := time.Parse("02.01.2006", "01.01.2027")
	if err != nil {
		t.Fatalf("parse test date: %v", err)
	}
	DB.Create(&Bets{UserID: 42, Text: "foo bar", Timestamp: future.Unix()})
	past := time.Now().Add(-48 * time.Hour)
	DB.Create(&Bets{UserID: 42, Text: "old bet", Timestamp: past.Unix()})

	// Default window starts today -> past bet hidden.
	client.Calls = nil
	if err := AllBets(bot, testContext(bot, "/allbets", testUser(42))); err != nil {
		t.Fatalf("AllBets: %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "foo bar") || strings.Contains(texts[0], "old bet") {
		t.Errorf("AllBets sent %q, want only the future bet", texts)
	}

	// "all" -> past bets included (used to be a no-op).
	client.Calls = nil
	if err := AllBets(bot, testContext(bot, "/allbets all", testUser(42))); err != nil {
		t.Fatalf("AllBets (all): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "foo bar") || !strings.Contains(texts[0], "old bet") {
		t.Errorf("AllBets (all) sent %q, want both bets", texts)
	}
}
