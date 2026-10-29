package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	cmc "github.com/miguelmota/go-coinmarketcap/pro/v1"
)

// TestGoogle covers the search link builder (it performs no HTTP request).
func TestGoogle(t *testing.T) {
	bot, client := setupTestEnv(t)

	// Usage message without a query.
	if err := Google(bot, testContext(bot, "/google", testUser(42))); err != nil {
		t.Fatalf("Google (usage): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "/google {запрос}") {
		t.Errorf("Google (usage) sent %q, want usage message", texts)
	}

	// The query is lowercased and URL escaped.
	client.Calls = nil
	if err := Google(bot, testContext(bot, "/google Привет Мир", testUser(42))); err != nil {
		t.Fatalf("Google: %v", err)
	}
	want := "https://www.google.com/search?q=" + url.QueryEscape("привет мир")
	texts = client.sentTexts()
	if len(texts) != 1 || texts[0] != want {
		t.Errorf("Google sent %q, want %q", texts, want)
	}

	// Replying anchors the link to the replied-to message instead.
	client.Calls = nil
	replyTo := testMessage("запрос", &targetUser)
	replyTo.MessageId = 55
	if err := Google(bot, testReplyContext(bot, "/google что поискать", testUser(42), replyTo)); err != nil {
		t.Fatalf("Google (reply): %v", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("Google (reply): sendMessage calls = %d, want 1", len(calls))
	}
	params, ok := calls[0].Params["reply_parameters"].(*gotgbot.ReplyParameters)
	if !ok {
		t.Fatalf("Google (reply): reply_parameters param missing: %v", calls[0].Params["reply_parameters"])
	}
	if params.MessageId != replyTo.MessageId || !params.AllowSendingWithoutReply {
		t.Errorf("Google (reply): reply_parameters = %+v, want message_id %d with AllowSendingWithoutReply", params, replyTo.MessageId)
	}
}

// TestDownload covers the parts of the yt-dlp handler that do not need the
// yt-dlp binary: usage, the "no links found" error, and the reaction handling.
func TestDownload(t *testing.T) {
	bot, client := setupTestEnv(t)

	// Usage message without a link or a reply.
	if err := Download(bot, testContext(bot, "/download", testUser(42))); err != nil {
		t.Fatalf("Download (usage): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "/download {ссылка") {
		t.Errorf("Download (usage) sent %q, want usage message", texts)
	}

	// A plain-text URL is not an entity, so no link is found and the message
	// gets the "watching" reaction.
	client.clearCalls()
	err := Download(bot, testContext(bot, "/download https://www.youtube.com/watch?v=123", testUser(42)))
	if err == nil || !strings.Contains(err.Error(), "ссылок не найдено") {
		t.Errorf("Download (no entities): err = %v, want the missing link error", err)
	}
	if n := len(client.callsFor("setMessageReaction")); n != 1 {
		t.Errorf("Download (no entities): setMessageReaction calls = %d, want 1", n)
	}
	if n := len(client.callsFor("deleteMessage")); n != 0 {
		t.Errorf("Download (no entities): deleteMessage calls = %d, want 0", n)
	}

	// The " remove" variant deletes the command instead of reacting.
	client.clearCalls()
	err = Download(bot, testContext(bot, "/download remove", testUser(42)))
	if err == nil {
		t.Error("Download (remove): expected the missing link error, got nil")
	}
	if n := len(client.callsFor("deleteMessage")); n != 1 {
		t.Errorf("Download (remove): deleteMessage calls = %d, want 1", n)
	}
	if n := len(client.callsFor("setMessageReaction")); n != 0 {
		t.Errorf("Download (remove): setMessageReaction calls = %d, want 0", n)
	}
}

// TestDownloadLocalSource runs /download end to end with the real yt-dlp: the
// HLS fixture on the loopback address offers a video-only and an audio-only
// variant, the format filter of the handler picks both, ffmpeg merges them into
// one mp4, and the video is sent with the size yt-dlp reported.
func TestDownloadLocalSource(t *testing.T) {
	bot, client := setupTestEnv(t)

	source := ytdlpFixtureServer(t, "video") + "/master.m3u8"

	client.clearCalls()
	// The handler closes and removes the file it sends, so its contents are
	// read at the moment of the send.
	var sent []byte
	client.Observe = func(method string, params map[string]any) {
		if method != "sendVideo" {
			return
		}
		if file, ok := params["video"].(*gotgbot.FileReader); ok {
			sent, _ = io.ReadAll(file.Data)
		}
	}
	msg := testMessage("/download "+source, testUser(42))
	// The handler names the download after the chat and the message, so the test
	// uses ids of its own: parallel runs of the suite must not share one temp file.
	chat, msgId := int64(-2000-os.Getpid()), int64(102)
	msg.Chat.Id, msg.MessageId = chat, msgId
	msg.Entities = []gotgbot.MessageEntity{{Type: "url", Offset: 9, Length: int64(len(source)), Url: source}}
	if err := Download(bot, ext.NewContext(bot, &gotgbot.Update{UpdateId: 1, Message: msg}, nil)); err != nil {
		t.Fatalf("Download (local source): %v", err)
	}

	calls := client.callsFor("sendVideo")
	if len(calls) != 1 {
		t.Fatalf("Download (local source): sendVideo calls = %d, want 1", len(calls))
	}
	file, ok := calls[0].Params["video"].(*gotgbot.FileReader)
	if !ok {
		t.Fatalf("Download (local source): video = %T, want the downloaded file", calls[0].Params["video"])
	}
	if file.Name != "master.mp4" {
		t.Errorf("Download (local source) file name = %q, want the title yt-dlp reported plus the mp4 extension", file.Name)
	}
	if len(sent) == 0 {
		t.Errorf("Download (local source) sent %d bytes, want a downloaded video file", len(sent))
	}
	sentPath := filepath.Join(t.TempDir(), "sent.mp4")
	if err := os.WriteFile(sentPath, sent, 0o644); err != nil {
		t.Fatalf("Download (local source): %v", err)
	}
	values := ffprobeStreamValues(t, sentPath, "codec_type")
	if !slices.Contains(values, "video") || !slices.Contains(values, "audio") {
		t.Errorf("Download (local source) streams = %v, want the video-only and the audio-only variant merged into one file", values)
	}
	if calls[0].Params["width"] != int64(128) || calls[0].Params["height"] != int64(96) {
		t.Errorf("Download (local source) size = %v x %v, want 128 x 96", calls[0].Params["width"], calls[0].Params["height"])
	}
	if _, err := os.Stat(filepath.Join(os.TempDir(), fmt.Sprintf("%v_%v.mp4", chat, msgId))); !os.IsNotExist(err) {
		t.Errorf("Download (local source): the temp file is left behind: %v", err)
	}
	if n := len(client.callsFor("sendChatAction")); n < 1 {
		t.Errorf("Download (local source): sendChatAction calls = %d, want at least 1", n)
	}
}

// TestOnReaction covers the reaction -> stats counter handler.
func TestOnReaction(t *testing.T) {
	bot, _ := setupTestEnv(t)

	reactionUpdate := func(newReaction []gotgbot.ReactionType) *ext.Context {
		return ext.NewContext(bot, &gotgbot.Update{UpdateId: 1, MessageReaction: &gotgbot.MessageReactionUpdated{
			Chat:        gotgbot.Chat{Id: Config.Chat, Type: "group"},
			MessageId:   55,
			Date:        time.Now().Unix(),
			NewReaction: newReaction,
		}}, nil)
	}

	// An update without a reaction is ignored.
	if err := OnReaction(bot, ext.NewContext(bot, &gotgbot.Update{UpdateId: 1}, nil)); err != nil {
		t.Fatalf("OnReaction (no reaction): %v", err)
	}
	// Removing a reaction (empty NewReaction) is ignored too.
	if err := OnReaction(bot, reactionUpdate(nil)); err != nil {
		t.Fatalf("OnReaction (removed reaction): %v", err)
	}
	var stats Stats
	if err := DB.First(&stats, "stat_type = ? AND context_id = ?", 6, 55).Error; err == nil {
		t.Errorf("OnReaction: stats row created for an ignored reaction: %+v", stats)
	}

	// A real reaction counts once per message.
	if err := OnReaction(bot, reactionUpdate([]gotgbot.ReactionType{gotgbot.ReactionTypeEmoji{Emoji: "👍"}})); err != nil {
		t.Fatalf("OnReaction: %v", err)
	}
	var reactionStats Stats
	if err := DB.First(&reactionStats, "stat_type = ? AND context_id = ?", 6, 55).Error; err != nil {
		t.Fatalf("OnReaction: stats row not created: %v", err)
	}
	if reactionStats.Count != 1 {
		t.Errorf("OnReaction: stats count = %d, want 1", reactionStats.Count)
	}
	if err := OnReaction(bot, reactionUpdate([]gotgbot.ReactionType{gotgbot.ReactionTypeEmoji{Emoji: "👍"}})); err != nil {
		t.Fatalf("OnReaction (second): %v", err)
	}
	var secondStats Stats
	if err := DB.First(&secondStats, "stat_type = ? AND context_id = ?", 6, 55).Error; err != nil {
		t.Fatalf("OnReaction (second): stats row lost: %v", err)
	}
	if secondStats.Count != 2 {
		t.Errorf("OnReaction (second): stats count = %d, want 2", secondStats.Count)
	}
}

// TestRemoveJoinMessageAndJoinUser covers cleaning up join messages in the
// reserve chat and unbanning everyone who joined.
func TestRemoveJoinMessageAndJoinUser(t *testing.T) {
	bot, client := setupTestEnv(t)

	msg := testMessage("вошли в чат", testUser(42))
	msg.Chat = gotgbot.Chat{Id: Config.ReserveChat, Type: "supergroup"}
	msg.NewChatMembers = []gotgbot.User{{Id: 777, FirstName: "Первый"}, {Id: 778, FirstName: "Второй"}}
	if err := RemoveJoinMessageAndJoinUser(bot, ext.NewContext(bot, &gotgbot.Update{UpdateId: 1, Message: msg}, nil)); err != nil {
		t.Fatalf("RemoveJoinMessageAndJoinUser: %v", err)
	}
	if n := len(client.callsFor("deleteMessage")); n != 1 {
		t.Errorf("RemoveJoinMessageAndJoinUser: deleteMessage calls = %d, want 1", n)
	}
	calls := client.callsFor("unbanChatMember")
	if len(calls) != 2 {
		t.Fatalf("RemoveJoinMessageAndJoinUser: unbanChatMember calls = %d, want 2", len(calls))
	}
	for i, want := range []int64{777, 778} {
		if calls[i].Params["user_id"] != want {
			t.Errorf("RemoveJoinMessageAndJoinUser: unban %d user_id = %v, want %d", i, calls[i].Params["user_id"], want)
		}
	}
}

// stubProxy points httpClient() at a test server. Go sends the absolute URL of a
// plain http:// request to an HTTP proxy, so the handler below can answer for the
// service URLs the commands have hardcoded.
func stubProxy(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	old := HTTPClientProxy
	t.Cleanup(func() { HTTPClientProxy = old })

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	proxyURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("stubProxy: %v", err)
	}
	HTTPClientProxy = func(*http.Request) (*url.URL, error) { return proxyURL, nil }
}

// TestTldr covers the static hint about the TLDR button.
func TestTldr(t *testing.T) {
	bot, client := setupTestEnv(t)

	if err := TLDR(bot, testContext(bot, "/tldr", testUser(42))); err != nil {
		t.Fatalf("TLDR: %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || texts[0] != "Юзайте кнопку TLDR напротив сообщения" {
		t.Errorf("TLDR sent %q, want the hint about the TLDR button", texts)
	}
}

// TestAdvice covers /advice against a stubbed advice API. The URL is hardcoded, so
// the request is intercepted with the proxy setting from the environment.
func TestAdvice(t *testing.T) {
	bot, client := setupTestEnv(t)

	stubProxy(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.RequestURI, "fucking-great-advice.ru/api/random") {
			http.Error(w, "unexpected target "+r.RequestURI, http.StatusNotFound)
			return
		}
		fmt.Fprint(w, `{"id":42,"text":"Проверяй тесты перед пушем.","sound":"meow"}`)
	})

	if err := Advice(bot, testContext(bot, "/advice", testUser(42))); err != nil {
		t.Fatalf("Advice: %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || texts[0] != "Проверяй тесты перед пушем." {
		t.Errorf("Advice sent %q, want the advice text", texts)
	}

	// A non-JSON answer (a captcha page, an error page from the proxy) is an error.
	stubProxy(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html>captcha</html>")
	})
	client.Calls = nil
	if err := Advice(bot, testContext(bot, "/advice", testUser(42))); err == nil {
		t.Errorf("Advice: err = nil, want a decode error")
	}
	if texts := client.sentTexts(); len(texts) != 0 {
		t.Errorf("Advice sent %q on a broken answer, want nothing", texts)
	}
}

// TestAnekdot covers the parser of the anekdot.ru random page. The page embeds the
// text in a JSON.parse('[\"…\",\"…"]') blob; the old code indexed the split result
// straight away and panicked when the marker was missing.
func TestAnekdot(t *testing.T) {
	// A <br> between Cyrillic letters splits a sentence, so it becomes a space.
	page := `x JSON.parse('[\"Первый анекдот<br>продолжение фразы и \\\"кавычки\\\"\",\"next']);`
	got, err := parseAnekdot(page)
	if err != nil {
		t.Fatalf("parseAnekdot: %v", err)
	}
	if want := "Первый анекдот продолжение фразы и \"кавычки\""; got != want {
		t.Errorf("parseAnekdot = %q, want %q", got, want)
	}

	// Any other <br> stays a line break.
	got, err = parseAnekdot(`JSON.parse('[\"First line<br>second line\",\"next'])`)
	if err != nil {
		t.Fatalf("parseAnekdot (latin): %v", err)
	}
	if got != "First line\nsecond line" {
		t.Errorf("parseAnekdot (latin) = %q, want %q", got, "First line\nsecond line")
	}

	// A page without the marker (blocked request, changed format) is an error.
	if _, err := parseAnekdot("<html>503 Service Unavailable</html>"); err == nil {
		t.Errorf("parseAnekdot: err = nil on a page without the joke, want an error")
	}
}

// TestCur covers the parts of /cur that need no CoinMarketCap request: the
// not-configured reply and the argument checks.
func TestCur(t *testing.T) {
	bot, client := setupTestEnv(t)

	// No ZAVTRA_CURRENCY_KEY in the environment.
	if err := Cur(bot, testContext(bot, "/cur 1 USD RUB", testUser(42))); err != nil {
		t.Fatalf("Cur (no key): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || texts[0] != "Конвертация валют не настроена" {
		t.Errorf("Cur (no key) sent %q, want the not-configured reply", texts)
	}

	// With a key the argument count is checked first. /cur alone used to read
	// Args()[1] before that check and panicked.
	oldKey := Config.CurrencyKey
	t.Cleanup(func() { Config.CurrencyKey = oldKey })
	Config.CurrencyKey = "test-key"

	for _, args := range []string{"/cur", "/cur 1", "/cur 1 USD RUB extra"} {
		client.Calls = nil
		if err := Cur(bot, testContext(bot, args, testUser(42))); err != nil {
			t.Fatalf("Cur (usage %q): %v", args, err)
		}
		texts := client.sentTexts()
		if len(texts) != 1 || !strings.Contains(texts[0], "/cur 1 USD RUB") {
			t.Errorf("Cur (usage %q) sent %q, want the usage message", args, texts)
		}
	}

	client.Calls = nil
	if err := Cur(bot, testContext(bot, "/cur abc USD RUB", testUser(42))); err == nil {
		t.Errorf("Cur (bad amount): err = nil, want a ParseFloat error")
	}
}

// TestCurSymbolMaps covers the currency lookup. The fiat/crypto maps are seeded
// here, so no CoinMarketCap request is made.
func TestCurSymbolMaps(t *testing.T) {
	oldCrypto, oldFiat, oldCustom := CryptoMap, FiatMap, CustomMap
	t.Cleanup(func() { CryptoMap, FiatMap, CustomMap = oldCrypto, oldFiat, oldCustom })

	FiatMap = []*cmc.FiatMapListing{
		{ID: 9911, Name: "Russian Ruble", Symbol: "RUB"},
		{ID: 9913, Name: "US Dollar", Symbol: "USD"},
		{ID: 9916, Name: "Belarusian Ruble", Symbol: "BYN"},
		{ID: 9934, Name: "Turkish Lira", Symbol: "TRY"},
	}
	CryptoMap = []*cmc.MapListing{{ID: 1, Name: "Bitcoin", Symbol: "BTC"}}
	CustomMap = []CustomMapStruct{{symbol: "COC", name: "Cup Of Coffee", amount: 300.0}}

	for _, c := range []struct{ in, want string }{
		{"RUB", "9911"},
		{"usd", "9913"}, // the symbol is uppercased before the lookup
		{"byr", "9916"}, // the old Belarusian ruble resolves to BYN
		{"TL", "9934"},  // the old Turkish lira resolves to TRY
		{"coc", "9911"}, // custom symbols are worth rubles
		{"btc", "1"},    // crypto is searched after fiat
	} {
		got, err := GetSymbolId(c.in)
		if err != nil {
			t.Errorf("GetSymbolId(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("GetSymbolId(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	if _, err := GetSymbolId("ZZZ"); err == nil || !strings.Contains(err.Error(), "распознать") {
		t.Errorf("GetSymbolId(ZZZ): err = %v, want the unknown currency message", err)
	}

	for _, c := range []struct{ id, want string }{
		{"9911", "Russian Ruble"},
		{"1", "Bitcoin"},
		{"777", ""},
	} {
		if got := GetIdName(c.id); got != c.want {
			t.Errorf("GetIdName(%q) = %q, want %q", c.id, got, c.want)
		}
	}

	// 9911 is the test currency of the API and is refused; the custom symbol
	// resolves to it, so /cur 1 COC RUB must not reach the network.
	bot, client := setupTestEnv(t)
	oldKey := Config.CurrencyKey
	t.Cleanup(func() { Config.CurrencyKey = oldKey })
	Config.CurrencyKey = "test-key"

	if err := Cur(bot, testContext(bot, "/cur 1 COC RUB", testUser(42))); err == nil ||
		!strings.Contains(err.Error(), "тестовую валюту") {
		t.Errorf("Cur (test currency): err = %v, want the test currency refusal", err)
	}
	if texts := client.sentTexts(); len(texts) != 0 {
		t.Errorf("Cur (test currency) sent %q, want nothing", texts)
	}
}

// TestAnekdotRequest runs the whole /anekdot command against a stubbed
// anekdot.ru page: the page is fetched over TLS, parsed, and the joke is sent
// to the chat. A page without the joke marker is reported as an error and
// nothing is sent.
func TestAnekdotRequest(t *testing.T) {
	bot, client := setupTestEnv(t)

	var page string
	stubTLSProxy(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "www.anekdot.ru" || r.URL.Path != "/rss/randomu.html" {
			http.Error(w, "unexpected target "+r.Host+r.URL.Path, http.StatusNotFound)
			return
		}
		fmt.Fprint(w, page)
	})

	page = `<html>JSON.parse('[\"Анекдот про тесты<br>и про продакшн\",\"comments']);</html>`
	if err := Anekdot(bot, testContext(bot, "/anekdot", testUser(42))); err != nil {
		t.Fatalf("Anekdot: %v", err)
	}
	want := "Анекдот про тесты и про продакшн"
	if texts := client.sentTexts(); len(texts) != 1 || texts[0] != want {
		t.Errorf("Anekdot sent %q, want %q", texts, want)
	}

	client.clearCalls()
	page = `<html>503 Service Unavailable</html>`
	if err := Anekdot(bot, testContext(bot, "/anekdot", testUser(42))); err == nil ||
		!strings.Contains(err.Error(), "нет текста анекдота") {
		t.Errorf("Anekdot (no joke on the page): err = %v, want the missing joke error", err)
	}
	if texts := client.sentTexts(); len(texts) != 0 {
		t.Errorf("Anekdot (no joke on the page) sent %q, want nothing", texts)
	}
}

// TestCurConversion runs /cur end to end: the fiat and crypto maps are loaded
// from CoinMarketCap, the price is converted, and the result is a reply to the
// command. The client of the API builds its own http.Client, so the requests
// are intercepted by the proxy of the test binary (see stubEnvRoute).
func TestCurConversion(t *testing.T) {
	bot, client := setupTestEnv(t)

	oldKey := Config.CurrencyKey
	oldCrypto, oldFiat, oldCustom := CryptoMap, FiatMap, CustomMap
	t.Cleanup(func() { Config.CurrencyKey, CryptoMap, FiatMap, CustomMap = oldKey, oldCrypto, oldFiat, oldCustom })

	// Empty maps force GetSymbolId to load them from the API.
	Config.CurrencyKey = "test-key"
	CryptoMap, FiatMap, CustomMap = nil, nil, nil

	var mu sync.Mutex
	var seen []string
	// The proxy handler runs on its own goroutines, so the recorded requests
	// are read and cleared under the same mutex.
	seenRequests := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen...)
	}
	clearRequests := func() {
		mu.Lock()
		seen = nil
		mu.Unlock()
	}
	record := func(r *http.Request) {
		mu.Lock()
		seen = append(seen, r.URL.Path+"?"+r.URL.RawQuery)
		mu.Unlock()
	}
	stubEnvRoute(t, "pro-api.coinmarketcap.com", "/v1/", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if r.Header.Get("X-CMC_PRO_API_KEY") != "test-key" {
			http.Error(w, `{"error_message":"bad key"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/cryptocurrency/map":
			fmt.Fprint(w, `{"status":{"error_code":0},"data":[{"id":1,"name":"Bitcoin","symbol":"BTC"},{"id":1027,"name":"Ethereum","symbol":"ETH"}]}`)
		case "/v1/fiat/map":
			fmt.Fprint(w, `{"status":{"error_code":0},"data":[{"id":9913,"name":"US Dollar","symbol":"USD","sign":"$"},{"id":9914,"name":"Euro","symbol":"EUR","sign":"€"}]}`)
		case "/v1/tools/price-conversion":
			fmt.Fprint(w, `{"status":{"error_code":0},"data":{"id":9913,"name":"US Dollar","symbol":"USD","amount":1,"quote":{"9914":{"price":0.92}}}}`)
		default:
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
		}
	})

	if err := Cur(bot, testContext(bot, "/cur 1 USD EUR", testUser(42))); err != nil {
		t.Fatalf("Cur: %v", err)
	}
	want := "1.00 US Dollar = 0.92 Euro"
	if texts := client.sentTexts(); len(texts) != 1 || texts[0] != want {
		t.Errorf("Cur sent %q, want %q", texts, want)
	}

	// The maps stay in the globals, so the next lookup needs no request.
	if len(CryptoMap) != 2 || len(FiatMap) != 2 {
		t.Errorf("Cur: CryptoMap = %d entries, FiatMap = %d, want 2 and 2", len(CryptoMap), len(FiatMap))
	}

	requests := seenRequests()
	for _, endpoint := range []string{"/v1/cryptocurrency/map", "/v1/fiat/map", "/v1/tools/price-conversion"} {
		found := false
		for _, entry := range requests {
			if strings.HasPrefix(entry, endpoint+"?") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Cur: the API never saw %v, requests: %v", endpoint, requests)
		}
	}
	for _, entry := range requests {
		if strings.HasPrefix(entry, "/v1/tools/price-conversion") &&
			!strings.Contains(entry, "amount=1.000000") {
			t.Errorf("Cur price request = %q, want amount=1.000000&id=9913&convert_id=9914", entry)
		}
	}

	// An unknown symbol is refused before any conversion request.
	client.clearCalls()
	if err := Cur(bot, testContext(bot, "/cur 1 ZZZ EUR", testUser(42))); err == nil ||
		!strings.Contains(err.Error(), "распознать") {
		t.Errorf("Cur (unknown symbol): err = %v, want the unknown currency error", err)
	}

	// A failing map request is the error of GenerateMaps, not a panic. The
	// client of the API throws its own status error away and reports the empty
	// body as a JSON error.
	clearRequests()
	CryptoMap, FiatMap, CustomMap = nil, nil, nil
	stubEnvRoute(t, "pro-api.coinmarketcap.com", "/v1/cryptocurrency/map", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		http.Error(w, `{"error_message":"rate limit"}`, http.StatusTooManyRequests)
	})
	if err := GenerateMaps(); err == nil {
		t.Errorf("GenerateMaps (API failure): err = nil, want an error")
	}
	if requests := seenRequests(); len(requests) != 1 {
		t.Errorf("GenerateMaps (API failure): API requests = %v, want one map request", requests)
	}

	// Without a currency key the maps are not loaded at all.
	Config.CurrencyKey = ""
	CryptoMap, FiatMap, CustomMap = nil, nil, nil
	if err := GenerateMaps(); err != nil {
		t.Errorf("GenerateMaps (no key): err = %v, want nil", err)
	}
	if requests := seenRequests(); len(requests) != 1 {
		t.Errorf("GenerateMaps (no key): API requests = %v, want no new request", requests)
	}
}
