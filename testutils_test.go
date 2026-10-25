package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---------- Fake Bot API client ----------

// fakeCall records a single Bot API request made by a handler.
type fakeCall struct {
	Method string
	Params map[string]any
}

// fakeBotClient implements gotgbot.BotClient: it records every API call and
// returns canned "result" JSON (gotgbot unwraps the {"ok":..,"result":..}
// envelope inside BaseBotClient, so fakes return only the result part).
// Some handlers fire background goroutines that keep calling the API after
// the handler returns, so access is mutex-guarded.
type fakeBotClient struct {
	mu           sync.Mutex
	Calls        []fakeCall
	Results      map[string]json.RawMessage // per-test canned results, override defaults
	Errors       map[string]error           // per-test API failures, checked before the results
	MemberStatus map[int64]string           // per-user getChatMember status overrides
	// Observe is called with every API call while its parameters are still
	// usable: a handler closes and removes the file it has just sent, so a test
	// that wants to look inside a sent file has to do it here. The observer must
	// not call the bot, the client is locked while it runs.
	Observe func(method string, params map[string]any)
}

// sentMessageResult is the answer every "send…" method returns by default, so
// handlers that read the id or the chat of the message they just sent see a
// real one instead of a zero value.
const sentMessageResult = `{"message_id":555,"chat":{"id":-100,"type":"group","title":"test chat"}}`

// defaultFakeResults covers methods used by the bot: bool-returning methods
// get "true", object-returning methods get "{}" (empty object) by default.
var defaultFakeResults = map[string]json.RawMessage{
	"sendMessage":            json.RawMessage(sentMessageResult),
	"sendPhoto":              json.RawMessage(sentMessageResult),
	"sendAudio":              json.RawMessage(sentMessageResult),
	"sendDocument":           json.RawMessage(sentMessageResult),
	"sendVideo":              json.RawMessage(sentMessageResult),
	"sendAnimation":          json.RawMessage(sentMessageResult),
	"sendVoice":              json.RawMessage(sentMessageResult),
	"sendVideoNote":          json.RawMessage(sentMessageResult),
	"sendSticker":            json.RawMessage(sentMessageResult),
	"sendMediaGroup":         json.RawMessage(`[` + sentMessageResult + `]`),
	"deleteMessage":          json.RawMessage(`true`),
	"deleteEphemeralMessage": json.RawMessage(`true`),
	"banChatMember":          json.RawMessage(`true`),
	"unbanChatMember":        json.RawMessage(`true`),
	"restrictChatMember":     json.RawMessage(`true`),
	"setMyCommands":          json.RawMessage(`true`),
	"deleteMyCommands":       json.RawMessage(`true`),
	"answerCallbackQuery":    json.RawMessage(`true`),
	"answerInlineQuery":      json.RawMessage(`true`),
	"forwardMessages":        json.RawMessage(`[]`),
	"copyMessages":           json.RawMessage(`[]`),
	"copyMessage":            json.RawMessage(`{"message_id":999}`),
	"getFile":                json.RawMessage(`{"file_path":"fake/file","file_size":1}`),
}

func (c *fakeBotClient) RequestWithContext(ctx context.Context, token string, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	stored := make(map[string]any, len(params))
	for k, v := range params {
		stored[k] = v
	}
	// The whole lookup below reads the recorded calls and the canned answers,
	// which tests mutate from the test goroutine while handler goroutines are
	// still calling the API.
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Calls = append(c.Calls, fakeCall{Method: method, Params: stored})

	if c.Observe != nil {
		c.Observe(method, stored)
	}

	if c.Errors != nil {
		if err, ok := c.Errors[method]; ok {
			return nil, err
		}
	}

	if c.Results != nil {
		if res, ok := c.Results[method]; ok {
			return res, nil
		}
	}
	// getChatMember echoes the requested user back, by default as a plain
	// "member", so handlers that use ChatMember.GetUser().Id see consistent
	// data. MemberStatus can pin specific users to other statuses.
	if method == "getChatMember" {
		status := "member"
		if c.MemberStatus != nil {
			if id, ok := params["user_id"].(int64); ok {
				if s, ok := c.MemberStatus[id]; ok {
					status = s
				}
			}
		}
		return json.RawMessage(fmt.Sprintf(`{"status":%q,"user":{"id":%v,"is_bot":false,"first_name":"member"}}`, status, params["user_id"])), nil
	}
	if res, ok := defaultFakeResults[method]; ok {
		return res, nil
	}
	return json.RawMessage(`{}`), nil
}

func (c *fakeBotClient) GetAPIURL(opts *gotgbot.RequestOpts) string { return "http://127.0.0.1:8081" }

func (c *fakeBotClient) FileURL(token string, tgFilePath string, opts *gotgbot.RequestOpts) string {
	return "http://127.0.0.1:8081/file/" + tgFilePath
}

// callsFor returns all recorded calls of the given API method.
func (c *fakeBotClient) callsFor(method string) []fakeCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []fakeCall
	for _, call := range c.Calls {
		if call.Method == method {
			out = append(out, call)
		}
	}
	return out
}

// clearCalls drops the recorded calls. Handlers that keep calling the API from
// a background goroutine after they return write the log under the same mutex,
// so clearing the log has to go through it too.
func (c *fakeBotClient) clearCalls() {
	c.mu.Lock()
	c.Calls = nil
	c.mu.Unlock()
}

// setResult pins the canned answer of an API method, under the client mutex for
// the same reason as clearCalls.
func (c *fakeBotClient) setResult(method string, result json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Results == nil {
		c.Results = make(map[string]json.RawMessage)
	}
	c.Results[method] = result
}

// setError makes one API method fail, under the client mutex for the same
// reason as clearCalls.
func (c *fakeBotClient) setError(method string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Errors == nil {
		c.Errors = make(map[string]error)
	}
	if err == nil {
		delete(c.Errors, method)
		return
	}
	c.Errors[method] = err
}

// setMemberStatus pins the getChatMember answer for one user.
func (c *fakeBotClient) setMemberStatus(userID int64, status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.MemberStatus == nil {
		c.MemberStatus = make(map[int64]string)
	}
	c.MemberStatus[userID] = status
}

// sentTexts returns the "text" param of every sendMessage call.
func (c *fakeBotClient) sentTexts() []string {
	var out []string
	for _, call := range c.callsFor("sendMessage") {
		if text, ok := call.Params["text"].(string); ok {
			out = append(out, text)
		}
	}
	return out
}

// waitForCalls blocks until the given method has been recorded want times.
// Polling through the client mutex also gives the race detector a
// happens-before edge with background goroutines that record calls (handlers
// like ForwardChannelPost run their API calls from a timer goroutine).
func waitForCalls(t *testing.T, client *fakeBotClient, method string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(client.callsFor(method)) >= want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d %v calls, got %d", want, method, len(client.callsFor(method)))
}

// ---------- Test environment ----------

// setupTestEnv replaces the Bot/DB/Config globals with test doubles and
// restores them on cleanup. The DB is an in-memory sqlite sharing the code's
// gorm models (the only Postgres-specific SQL in the codebase is RANDOM(),
// which sqlite also supports).
// memDBCounter gives every test its own named in-memory sqlite database.
// A named shared-cache DB can be accessed by several pooled connections at
// once, which handlers need: some (Pidorstats, AllBets) run extra queries
// while iterating an open Rows() cursor - a single-connection :memory: DB
// would deadlock there.
var memDBCounter int

func setupTestEnv(t *testing.T) (*gotgbot.Bot, *fakeBotClient) {
	t.Helper()

	oldBot, oldDB, oldConfig, oldLogger := Bot, DB, Config, Logger
	t.Cleanup(func() { Bot, DB, Config, Logger = oldBot, oldDB, oldConfig, oldLogger })

	// main() builds the slog logger; handlers that log need it in tests too.
	Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	client := &fakeBotClient{}
	bot := &gotgbot.Bot{User: gotgbot.User{Id: 1, IsBot: true, Username: "testbot"}, Token: "TESTTOKEN", BotClient: client}
	Bot = bot

	memDBCounter++
	dsn := fmt.Sprintf("file:memdb%d?mode=memory&cache=shared&_pragma=busy_timeout(10000)", memDBCounter)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("test db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(8)

	if err := db.AutoMigrate(&gotgbot.User{}, &Get{}, &AntiSpam{}, &Warn{}, &PidorStats{}, &PidorList{}, &Duelist{}, &Bless{}, &Salvation{}, &Nope{}, &Stats{}, &StatsWords{}, &Bets{}, &MuteEvent{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	DB = *db

	Config = &Configuration{
		Chat:           -100,
		ReserveChat:    -200,
		CommentChat:    -300,
		Admins:         []int64{100},
		Moders:         []int64{300},
		SysAdmin:       100,
		AntiSpamWindow: 30,
		AntiSpamCount:  3,
	}

	// Seed pools used by Denied()/GetNope()/GetBless().
	DB.Create(&Nope{Text: "нет"})
	DB.Create(&Bless{Text: "да"})

	return bot, client
}

// ---------- Update/message builders ----------

func testUser(id int64) *gotgbot.User {
	return &gotgbot.User{Id: id, FirstName: "Тест", LastName: "Пупкин", Username: "testpupkin"}
}

func testMessage(text string, from *gotgbot.User) *gotgbot.Message {
	return &gotgbot.Message{
		MessageId: 100,
		Date:      time.Now().Unix(),
		Chat:      gotgbot.Chat{Id: -100, Type: "group", Title: "test chat"},
		From:      from,
		Text:      text,
	}
}

// testContext builds an ext.Context for a plain group message from the given user.
func testContext(bot *gotgbot.Bot, text string, from *gotgbot.User) *ext.Context {
	return ext.NewContext(bot, &gotgbot.Update{UpdateId: 1, Message: testMessage(text, from)}, nil)
}

// testReplyContext builds an ext.Context for a message replying to replyTo.
func testReplyContext(bot *gotgbot.Bot, text string, from *gotgbot.User, replyTo *gotgbot.Message) *ext.Context {
	msg := testMessage(text, from)
	msg.ReplyToMessage = replyTo
	return ext.NewContext(bot, &gotgbot.Update{UpdateId: 1, Message: msg}, nil)
}

// testCallbackContext builds an ext.Context for a callback query pressed on
// msg by from (the duel accept/deny buttons).
func testCallbackContext(bot *gotgbot.Bot, msg *gotgbot.Message, from *gotgbot.User) *ext.Context {
	cb := &gotgbot.CallbackQuery{Id: "123", From: *from, Message: *msg}
	return ext.NewContext(bot, &gotgbot.Update{UpdateId: 1, CallbackQuery: cb}, nil)
}

// testInlineContext builds an ext.Context for an inline query from from.
func testInlineContext(bot *gotgbot.Bot, from *gotgbot.User, query string) *ext.Context {
	iq := &gotgbot.InlineQuery{Id: "1", From: *from, Query: query}
	return ext.NewContext(bot, &gotgbot.Update{UpdateId: 1, InlineQuery: iq}, nil)
}

// seedUser inserts a user into the users table (used by GetUserFromDB lookups).
func seedUser(id int64, username, firstName string) {
	if err := DB.Create(&gotgbot.User{Id: id, Username: username, FirstName: firstName}).Error; err != nil {
		panic("seedUser: " + err.Error())
	}
}

// statTotal sums the stats rows of a stat type for one context (user id,
// message id or word id), 0 when there are none.
func statTotal(t *testing.T, statType int64, contextID int64) int64 {
	t.Helper()
	var total int64
	if err := DB.Model(&Stats{}).Where("stat_type = ? AND context_id = ?", statType, contextID).
		Select("COALESCE(SUM(count), 0)").Scan(&total).Error; err != nil {
		t.Fatalf("statTotal(%v, %v): %v", statType, contextID, err)
	}
	return total
}

// muteSeconds sums the stat type 7 (mute time in seconds) rows of a user,
// 0 when the user has never been muted.
func muteSeconds(t *testing.T, userID int64) int64 {
	t.Helper()
	return statTotal(t, 7, userID)
}

// minutesRE matches a count written out with the declined minute noun,
// e.g. "12 минут", "1 минуту".
var minutesRE = regexp.MustCompile(`(\d+) (минуту|минуты|минут)`)

// assertMinutesDeclined checks that every "N минут" in text spells the noun out
// in the form ruPlural picks for N, i.e. not the "мин." abbreviation and not a
// fixed form. Texts with a respawn/mute duration must contain such a phrase.
func assertMinutesDeclined(t *testing.T, text string) {
	t.Helper()
	hits := minutesRE.FindAllStringSubmatch(text, -1)
	if len(hits) == 0 {
		t.Errorf("нет склоняемого «минута» с числом в тексте %q", text)
		return
	}
	for _, m := range hits {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			t.Errorf("нечитаемое число в %q: %v", m[0], err)
			continue
		}
		if want := ruPlural(n, "минуту", "минуты", "минут"); m[2] != want {
			t.Errorf("«%s %s», want «%s %s»; текст: %q", m[1], m[2], m[1], want, text)
		}
	}
}
