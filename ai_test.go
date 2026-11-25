package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// TestAiInit covers the agent build: the model is pointed at the URL from the
// environment and the system prompt comes from the environment too. The URL is
// not validated at build time — a bad one only shows up when the agent is
// called, which is what TestAIStreamError checks.
func TestAiInit(t *testing.T) {
	setupTestEnv(t)

	oldSystem, oldAgent, oldURL := AISystem, AIAgent, Config.AIURL
	t.Cleanup(func() { AISystem, AIAgent, Config.AIURL = oldSystem, oldAgent, oldURL })

	Config.AIURL = "http://127.0.0.1:1/v1"
	Config.AISystem = "система"
	AISystem, AIAgent = "", nil

	if err := AiInit(); err != nil {
		t.Fatalf("AiInit: %v", err)
	}
	if AIAgent == nil {
		t.Errorf("AiInit: AIAgent is nil, want the built agent")
	}
	if AISystem != "система" {
		t.Errorf("AISystem = %q, want the prompt from config", AISystem)
	}

	// Nothing about the endpoint is checked when the agent is built, so an
	// unparsable URL produces an agent that only fails on the first request.
	Config.AIURL = "http://[::1"
	AIAgent = nil
	if err := AiInit(); err != nil {
		t.Errorf("AiInit (broken URL): %v, want no error at build time", err)
	}
	if AIAgent == nil {
		t.Errorf("AiInit (broken URL): AIAgent is nil, want the agent to be built")
	}
}

// TestRestartAI covers /restartai: the refusal for regular users, and the
// report to the admin after the agent is rebuilt.
func TestRestartAI(t *testing.T) {
	bot, client := setupTestEnv(t)

	oldSystem, oldAgent, oldURL := AISystem, AIAgent, Config.AIURL
	t.Cleanup(func() { AISystem, AIAgent, Config.AIURL = oldSystem, oldAgent, oldURL })
	Config.AISystem = "система"
	Config.AIURL = "http://127.0.0.1:1/v1"

	if err := RestartAI(bot, testContext(bot, "/restartai", testUser(42))); err != nil {
		t.Fatalf("RestartAI (regular user): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || texts[0] != "нет" {
		t.Errorf("RestartAI (regular user) sent %q, want the refusal from the pool", texts)
	}

	client.clearCalls()
	if err := RestartAI(bot, testContext(bot, "/restartai", testUser(100))); err != nil {
		t.Fatalf("RestartAI (admin): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || texts[0] != "Агент перезапущен" {
		t.Errorf("RestartAI (admin) sent %q, want the restart report", texts)
	}
	if AIAgent == nil {
		t.Errorf("RestartAI (admin): AIAgent is nil, want the rebuilt agent")
	}

	// A dead endpoint is not noticed here: the command reports success, and the
	// failure only appears on the next /ai request.
	client.clearCalls()
	Config.AIURL = "http://[::1"
	if err := RestartAI(bot, testContext(bot, "/restartai", testUser(100))); err != nil {
		t.Fatalf("RestartAI (broken URL): %v", err)
	}
	if texts := client.sentTexts(); len(texts) != 1 || texts[0] != "Агент перезапущен" {
		t.Errorf("RestartAI (broken URL) sent %q, want the restart report", texts)
	}
}

// TestAIWithoutAgent covers the guard of /ai: the command answers that the agent
// is not initialized, and nothing is sent to the model.
func TestAIWithoutAgent(t *testing.T) {
	bot, client := setupTestEnv(t)

	oldAgent := AIAgent
	t.Cleanup(func() { AIAgent = oldAgent })
	AIAgent = nil

	if err := AI(bot, testContext(bot, "/ai привет", testUser(42))); err != nil {
		t.Fatalf("AI: %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || texts[0] != "Агент не инициализирован" {
		t.Errorf("AI sent %q, want the uninitialized agent note", texts)
	}
	if calls := client.callsFor("setMessageReaction"); len(calls) != 0 {
		t.Errorf("AI: setMessageReaction calls = %d, want none before the agent check", len(calls))
	}
}

// TestAIStream runs /ai against a local OpenAI-compatible endpoint that streams
// an answer: the command replies first, the streamed text is written into that
// reply, and the markdown of the answer arrives as entities.
func TestAIStream(t *testing.T) {
	bot, client := setupTestEnv(t)

	oldSystem, oldAgent, oldURL := AISystem, AIAgent, Config.AIURL
	t.Cleanup(func() { AISystem, AIAgent, Config.AIURL = oldSystem, oldAgent, oldURL })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"content":"**привет**"}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"content":" мир"},"finish_reason":"stop"}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(server.Close)

	Config.AIURL = server.URL + "/v1"
	AISystem = "система"
	if err := AiInit(); err != nil {
		t.Fatalf("AiInit: %v", err)
	}

	if err := AI(bot, testContext(bot, "/ai привет", testUser(42))); err != nil {
		t.Fatalf("AI: %v", err)
	}

	if calls := client.callsFor("setMessageReaction"); len(calls) != 1 {
		t.Errorf("AI: setMessageReaction calls = %d, want the watching reaction", len(calls))
	}

	sent := client.callsFor("sendMessage")
	if len(sent) != 1 {
		t.Fatalf("AI: sendMessage calls = %d, want the placeholder answer", len(sent))
	}
	if text, _ := sent[0].Params["text"].(string); !strings.HasPrefix(text, "Выполняю запрос") {
		t.Errorf("AI placeholder = %q, want the working note", text)
	}

	edits := client.callsFor("editMessageText")
	if len(edits) == 0 {
		t.Fatalf("AI: editMessageText calls = 0, want the streamed answer in the reply")
	}
	last := edits[len(edits)-1]
	text, _ := last.Params["text"].(string)
	if strings.Contains(text, "Выполняю запрос") {
		t.Errorf("AI final edit = %q, want the answer without the working note", text)
	}
	if !strings.Contains(text, "привет") {
		t.Errorf("AI final edit = %q, want the streamed text", text)
	}
	if last.Params["chat_id"] != int64(-100) {
		t.Errorf("AI final edit chat_id = %v, want the chat of the request", last.Params["chat_id"])
	}

	// The markdown of the answer is sent as entities, not as asterisks.
	if _, ok := last.Params["entities"]; !ok {
		t.Errorf("AI final edit has no entities: %+v", last.Params)
	}
	if strings.Contains(text, "**") {
		t.Errorf("AI final edit = %q, want the markdown converted to entities", text)
	}

	// The chat sees the typing indicator while the model is answering.
	if calls := client.callsFor("sendChatAction"); len(calls) == 0 {
		t.Errorf("AI: sendChatAction calls = %d, want the typing indicator", len(calls))
	}
}

// TestAIStreamError runs /ai against an endpoint that refuses the request: the
// working note is already in the chat, nothing is edited into it, and the
// failure goes back to the dispatcher.
func TestAIStreamError(t *testing.T) {
	bot, client := setupTestEnv(t)

	oldSystem, oldAgent, oldURL := AISystem, AIAgent, Config.AIURL
	t.Cleanup(func() { AISystem, AIAgent, Config.AIURL = oldSystem, oldAgent, oldURL })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":{"message":"no such model"}}`)
	}))
	t.Cleanup(server.Close)

	Config.AIURL = server.URL + "/v1"
	AISystem = "система"
	if err := AiInit(); err != nil {
		t.Fatalf("AiInit: %v", err)
	}

	if err := AI(bot, testContext(bot, "/ai привет", testUser(42))); err == nil {
		t.Errorf("AI (endpoint failure): err = nil, want the model error")
	}

	sent := client.callsFor("sendMessage")
	if len(sent) != 1 {
		t.Errorf("AI (endpoint failure): sendMessage calls = %d, want the working note", len(sent))
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 0 {
		t.Errorf("AI (endpoint failure): editMessageText calls = %d, want none", len(calls))
	}
}

func TestAIReplyFailure(t *testing.T) {
	bot, client := setupTestEnv(t)

	oldSystem, oldAgent, oldURL := AISystem, AIAgent, Config.AIURL
	t.Cleanup(func() { AISystem, AIAgent, Config.AIURL = oldSystem, oldAgent, oldURL })

	var modelCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		modelCalls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"content":"ответ"},"finish_reason":"stop"}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(server.Close)

	Config.AIURL = server.URL + "/v1"
	AISystem = "система"
	if err := AiInit(); err != nil {
		t.Fatalf("AiInit: %v", err)
	}

	replyErr := errors.New("bot was blocked")
	client.setError("sendMessage", replyErr)

	err := AI(bot, testContext(bot, "/ai привет", testUser(42)))
	if !errors.Is(err, replyErr) {
		t.Errorf("AI (reply failure): err = %v, want the sendMessage error", err)
	}
	if n := modelCalls.Load(); n != 0 {
		t.Errorf("AI (reply failure): model calls = %d, want none", n)
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 0 {
		t.Errorf("AI (reply failure): editMessageText calls = %d, want none", len(calls))
	}
}

// TestAIWithPhoto runs /ai with a picture attached to the command: the picture
// is read, encoded, and travels to the model as a data URL next to the text of
// the request, and the answer is streamed into the reply as usual.
func TestAIWithPhoto(t *testing.T) {
	bot, client := setupTestEnv(t)

	oldSystem, oldAgent, oldURL := AISystem, AIAgent, Config.AIURL
	t.Cleanup(func() { AISystem, AIAgent, Config.AIURL = oldSystem, oldAgent, oldURL })

	var mu sync.Mutex
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		data, _ := io.ReadAll(r.Body)
		mu.Lock()
		body = string(data)
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"content":"на картинке"}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"content":" тест"},"finish_reason":"stop"}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(server.Close)

	Config.AIURL = server.URL + "/v1"
	AISystem = "система"
	if err := AiInit(); err != nil {
		t.Fatalf("AiInit: %v", err)
	}

	input := ffmpegMedia(t, "photo")
	data, err := os.ReadFile(input)
	if err != nil {
		t.Fatalf("generated photo: %v", err)
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	client.setResult("getFile", json.RawMessage(fmt.Sprintf(`{"file_path":%q,"file_size":%d}`, input, len(data))))

	msg := testMessage("/ai что на картинке", testUser(42))
	msg.Photo = []gotgbot.PhotoSize{{FileId: "ph1", FileSize: int64(len(data))}}
	if err := AI(bot, ext.NewContext(bot, &gotgbot.Update{UpdateId: 1, Message: msg}, nil)); err != nil {
		t.Fatalf("AI (photo): %v", err)
	}

	edits := client.callsFor("editMessageText")
	if len(edits) == 0 {
		t.Fatalf("AI (photo): editMessageText calls = 0, want the streamed answer")
	}
	text, _ := edits[len(edits)-1].Params["text"].(string)
	if !strings.Contains(text, "на картинке тест") {
		t.Errorf("AI (photo) final edit = %q, want the streamed text", text)
	}

	// The model gets the bytes of the picture, not a path to a file it cannot
	// reach, and the text of the command next to it.
	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(body, "data:image/jpeg;base64,"+b64) {
		t.Errorf("AI (photo): the model request has no photo in it: %.400v", body)
	}
	if !strings.Contains(body, "что на картинке") {
		t.Errorf("AI (photo): the model request has no command text in it: %.400v", body)
	}
}

// TestGetImageForLLM covers the picture handed to the model: the file is read
// from disk, encoded as base64, and the mime type comes from its extension.
func TestGetImageForLLM(t *testing.T) {
	_, client := setupTestEnv(t)

	input := ffmpegMedia(t, "photo")
	data, err := os.ReadFile(input)
	if err != nil {
		t.Fatalf("generated photo: %v", err)
	}
	client.setResult("getFile", json.RawMessage(fmt.Sprintf(`{"file_path":%q,"file_size":%d}`, input, len(data))))

	msg := &gotgbot.Message{Photo: []gotgbot.PhotoSize{{FileId: "ph1", FileSize: int64(len(data))}}}

	b64, mimeType, err := getImageForLLM(msg)
	if err != nil {
		t.Fatalf("getImageForLLM: %v", err)
	}
	if mimeType != "image/jpeg" {
		t.Errorf("getImageForLLM mime = %q, want image/jpeg for a .jpg file", mimeType)
	}
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("getImageForLLM returned no base64: %v", err)
	}
	if string(decoded) != string(data) {
		t.Errorf("getImageForLLM = %d bytes, want the bytes of the photo", len(decoded))
	}
	// The file the picture was read from is removed behind the handler.
	if _, err := os.Stat(input); !os.IsNotExist(err) {
		t.Errorf("getImageForLLM: %v, want the source file removed", err)
	}
}

// TestGetImageForLLMRemote covers the picture the bot does not have on disk:
// it is downloaded from the Telegram file server over TLS, encoded the same
// way, and the downloaded copy is removed behind the helper.
func TestGetImageForLLMRemote(t *testing.T) {
	_, client := setupTestEnv(t)

	source := ffmpegMedia(t, "photo")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("generated photo: %v", err)
	}

	// A path under the temp dir that does not exist: the helper downloads it.
	remote := filepath.Join(t.TempDir(), "photos", "photo.jpg")
	client.setResult("getFile", json.RawMessage(fmt.Sprintf(`{"file_path":%q,"file_size":%d}`, remote, len(data))))

	oldToken := Config.Token
	t.Cleanup(func() { Config.Token = oldToken })
	Config.Token = "TESTTOKEN"

	var mu sync.Mutex
	var paths []string
	stubTLSProxy(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "api.telegram.org" || !strings.Contains(r.URL.Path, "/file/botTESTTOKEN/") {
			http.Error(w, "unexpected request "+r.Host+r.URL.Path, http.StatusNotFound)
			return
		}
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		w.Write(data)
	})

	msg := &gotgbot.Message{Photo: []gotgbot.PhotoSize{{FileId: "ph1", FileSize: int64(len(data))}}}
	b64, mimeType, err := getImageForLLM(msg)
	if err != nil {
		t.Fatalf("getImageForLLM (remote file): %v", err)
	}
	if mimeType != "image/jpeg" {
		t.Errorf("getImageForLLM (remote file) mime = %q, want image/jpeg", mimeType)
	}
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("getImageForLLM (remote file) returned no base64: %v", err)
	}
	if string(decoded) != string(data) {
		t.Errorf("getImageForLLM (remote file) = %d bytes, want the bytes of the photo", len(decoded))
	}

	mu.Lock()
	defer mu.Unlock()
	if len(paths) == 0 {
		t.Errorf("getImageForLLM (remote file): the Telegram file server was never asked, want a download")
	}

	// The downloaded copy is cleaned up behind the helper.
	if _, err := os.Stat(remote); !os.IsNotExist(err) {
		t.Errorf("getImageForLLM (remote file): %v, want the downloaded file removed", err)
	}
}

// TestGetImageForLLMConcurrentSameFile asks about one and the same picture
// from four conversations at once. Each request downloads its own copy under a
// unique name: the name built from the current second made the requests share
// one file, and the first to finish removed the copy the others were reading.
func TestGetImageForLLMConcurrentSameFile(t *testing.T) {
	_, client := setupTestEnv(t)

	source := ffmpegMedia(t, "photo")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("generated photo: %v", err)
	}

	remote := filepath.Join(t.TempDir(), "photos", "photo.jpg")
	client.setResult("getFile", json.RawMessage(fmt.Sprintf(`{"file_path":%q,"file_size":%d}`, remote, len(data))))

	oldToken := Config.Token
	t.Cleanup(func() { Config.Token = oldToken })
	Config.Token = "TESTTOKEN"

	stubTLSProxy(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "api.telegram.org" || !strings.Contains(r.URL.Path, "/file/botTESTTOKEN/") {
			http.Error(w, "unexpected request "+r.Host+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Write(data)
	})

	msg := &gotgbot.Message{Photo: []gotgbot.PhotoSize{{FileId: "ph1", FileSize: int64(len(data))}}}

	const runs = 4
	errs := make([]error, runs)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			b64, mimeType, err := getImageForLLM(msg)
			if err != nil {
				errs[i] = err
				return
			}
			if mimeType != "image/jpeg" {
				errs[i] = fmt.Errorf("mime = %q, want image/jpeg", mimeType)
				return
			}
			decoded, err := base64.StdEncoding.DecodeString(b64)
			if err != nil || string(decoded) != string(data) {
				errs[i] = fmt.Errorf("decoded %d bytes, want the bytes of the photo", len(decoded))
			}
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("getImageForLLM request %v of %v: %v", i+1, runs, err)
		}
	}
}

// TestSetAISystem covers /setaisystem: the refusal for regular users, and the
// prompt the admin sets on top of the one from the environment.
func TestSetAISystem(t *testing.T) {
	bot, client := setupTestEnv(t)

	oldSystem := AISystem
	t.Cleanup(func() { AISystem = oldSystem })
	Config.AISystem = "база"

	// A regular user gets the refusal, and the prompt stays as it was.
	AISystem = "база"
	if err := SetAISystem(bot, testContext(bot, "/setaisystem новый промпт", testUser(42))); err != nil {
		t.Fatalf("SetAISystem (regular user): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || texts[0] != "нет" {
		t.Errorf("SetAISystem (regular user) sent %q, want the refusal from the pool", texts)
	}
	if AISystem != "база" {
		t.Errorf("SetAISystem (regular user): AISystem = %q, want it untouched", AISystem)
	}

	// The admin prompt is appended to the prompt from the environment.
	client.clearCalls()
	if err := SetAISystem(bot, testContext(bot, "/setaisystem новый промпт", testUser(100))); err != nil {
		t.Fatalf("SetAISystem (admin): %v", err)
	}
	if AISystem != "база\nновый промпт" {
		t.Errorf("SetAISystem (admin): AISystem = %q, want %q", AISystem, "база\nновый промпт")
	}
	texts = client.sentTexts()
	if len(texts) != 1 || texts[0] != "Системный промпт изменен на:\nновый промпт" {
		t.Errorf("SetAISystem (admin) sent %q, want the report of the new prompt", texts)
	}

	// Without arguments the prompt ends with an empty line.
	client.clearCalls()
	if err := SetAISystem(bot, testContext(bot, "/setaisystem", testUser(100))); err != nil {
		t.Fatalf("SetAISystem (no arguments): %v", err)
	}
	if AISystem != "база\n" {
		t.Errorf("SetAISystem (no arguments): AISystem = %q, want %q", AISystem, "база\n")
	}
}
