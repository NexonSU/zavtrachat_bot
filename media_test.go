package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// fakeMediaFile points the getFile answer at a real generated media file, so
// GetMedia finds the file locally and the media commands run end to end.
func fakeMediaFile(t *testing.T, client *fakeBotClient, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("generated media: %v", err)
	}
	client.setResult("getFile", json.RawMessage(fmt.Sprintf(`{"file_path":%q,"file_size":%d}`, path, info.Size())))
}

func TestFFmpegConvertSharedSource(t *testing.T) {
	bot, client := setupTestEnv(t)
	source := ffmpegMedia(t, "video")
	media := Media{FilePath: source, FileName: filepath.Base(source)}
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			errs <- FFmpegConvert(testContext(bot, "/convert gif", testUser(42)), media, "gif")
		}()
	}
	for range 2 {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
	if len(client.callsFor("sendAnimation")) != 2 {
		t.Fatal("concurrent conversions did not both send a result")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("conversion deleted the shared Telegram source: %v", err)
	}
}

// TestConvert covers /convert: the usage notes and the real conversions of the
// replied-to file. Without an argument the target is the media type itself, a
// video sticker is offered as an animation.
func TestConvert(t *testing.T) {
	bot, client := setupTestEnv(t)

	// A command without a replied-to message explains itself.
	if err := Convert(bot, testContext(bot, "/convert", testUser(42))); err != nil {
		t.Fatalf("Convert (no reply): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Пример использования") {
		t.Errorf("Convert (no reply) sent %q, want the usage note", texts)
	}

	// A reply without a media file is refused.
	client.clearCalls()
	if err := Convert(bot, testReplyContext(bot, "/convert", testUser(42), &gotgbot.Message{Text: "текст"})); err != nil {
		t.Fatalf("Convert (no media): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Какого-либо медиа файла нет") {
		t.Errorf("Convert (no media) sent %q, want the missing media note", texts)
	}

	for _, c := range []struct {
		name, kind, arg, method string
		reply                   *gotgbot.Message
	}{
		{"audio to ogg", "audio", "ogg", "sendVoice", &gotgbot.Message{Audio: &gotgbot.Audio{FileId: "aud1"}}},
		{"video to gif", "video", "gif", "sendAnimation", &gotgbot.Message{Video: &gotgbot.Video{FileId: "vid1"}}},
		{"video without argument stays a video", "video", "", "sendVideo", &gotgbot.Message{Video: &gotgbot.Video{FileId: "vid1"}}},
		{"video sticker to gif", "video", "gif", "sendAnimation", &gotgbot.Message{Sticker: &gotgbot.Sticker{FileId: "stk1", IsVideo: true}}},
		{"photo to jpg", "photo", "", "sendDocument", &gotgbot.Message{Photo: []gotgbot.PhotoSize{{FileId: "pho1", FileSize: 10}}}},
	} {
		client.clearCalls()
		input := ffmpegMedia(t, c.kind)
		fakeMediaFile(t, client, input)

		if err := Convert(bot, testReplyContext(bot, "/convert "+c.arg, testUser(42), c.reply)); err != nil {
			t.Errorf("Convert (%s): %v", c.name, err)
			continue
		}
		calls := client.callsFor(c.method)
		if len(calls) != 1 {
			t.Errorf("Convert (%s): %s calls = %d, want 1", c.name, c.method, len(calls))
		}
	}
}

// TestInvert covers /invert: the usage notes, the media types the reverse
// filters refuse, and the reversed audio and video conversions.
func TestInvert(t *testing.T) {
	bot, client := setupTestEnv(t)

	if err := Invert(bot, testContext(bot, "/invert", testUser(42))); err != nil {
		t.Fatalf("Invert (no reply): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "в ответ на какое-либо сообщение с видео") {
		t.Errorf("Invert (no reply) sent %q, want the usage note", texts)
	}

	client.clearCalls()
	if err := Invert(bot, testReplyContext(bot, "/invert", testUser(42), &gotgbot.Message{Text: "текст"})); err != nil {
		t.Fatalf("Invert (no media): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Какого-либо видео нет") {
		t.Errorf("Invert (no media) sent %q, want the missing media note", texts)
	}

	// A photo has no reverse filter, and neither has a static sticker.
	for _, c := range []struct {
		name  string
		reply *gotgbot.Message
	}{
		{"photo", &gotgbot.Message{Photo: []gotgbot.PhotoSize{{FileId: "pho1", FileSize: 10}}}},
		{"static sticker", &gotgbot.Message{Sticker: &gotgbot.Sticker{FileId: "stk1"}}},
	} {
		client.clearCalls()
		if err := Invert(bot, testReplyContext(bot, "/invert", testUser(42), c.reply)); err != nil {
			t.Fatalf("Invert (%s): %v", c.name, err)
		}
		texts = client.sentTexts()
		if len(texts) != 1 || !strings.Contains(texts[0], "Неподдерживаемая операция") {
			t.Errorf("Invert (%s) sent %q, want the unsupported-operation note", c.name, texts)
		}
	}

	for _, c := range []struct {
		name, kind, arg, method string
		reply                   *gotgbot.Message
	}{
		{"audio to reversed audio", "audio", "mp3", "sendAudio", &gotgbot.Message{Audio: &gotgbot.Audio{FileId: "aud1"}}},
		{"video to reversed video", "video", "mp4", "sendVideo", &gotgbot.Message{Video: &gotgbot.Video{FileId: "vid1"}}},
	} {
		client.clearCalls()
		input := ffmpegMedia(t, c.kind)
		fakeMediaFile(t, client, input)

		if err := Invert(bot, testReplyContext(bot, "/invert "+c.arg, testUser(42), c.reply)); err != nil {
			t.Errorf("Invert (%s): %v", c.name, err)
			continue
		}
		if calls := client.callsFor(c.method); len(calls) != 1 {
			t.Errorf("Invert (%s): %s calls = %d, want 1", c.name, c.method, len(calls))
		}
	}
}

// TestDistort covers /distort: the usage notes, the media types it refuses, the
// busy flag, the per-file cache, the work directory guard, and the distorted
// audio result.
func TestDistort(t *testing.T) {
	bot, client := setupTestEnv(t)

	oldBusy, oldCache := DistortBusy, DistortCache
	t.Cleanup(func() { DistortBusy, DistortCache = oldBusy, oldCache })
	DistortBusy = false
	DistortCache = map[string]string{}

	// The work directories live under the system temp dir, which every process
	// on the machine shares: point them at the temp dir of this test, so a
	// second test process cannot wipe a work directory this one is converting.
	oldTempDir := DistortTempDir
	t.Cleanup(func() { DistortTempDir = oldTempDir })
	DistortTempDir = t.TempDir()

	// The fake API must answer with a document, /distort stores its file id.
	client.setResult("sendDocument", json.RawMessage(`{"message_id":1,"document":{"file_id":"doc1"}}`))
	client.setResult("sendAudio", json.RawMessage(`{"message_id":1,"audio":{"file_id":"distorted1"}}`))

	if err := Distort(bot, testContext(bot, "/distort", testUser(42))); err != nil {
		t.Fatalf("Distort (no reply): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "в ответ на какое-либо сообщение с видео") {
		t.Errorf("Distort (no reply) sent %q, want the usage note", texts)
	}

	client.clearCalls()
	if err := Distort(bot, testReplyContext(bot, "/distort", testUser(42), &gotgbot.Message{Text: "текст"})); err != nil {
		t.Fatalf("Distort (no media): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Какого-либо видео нет") {
		t.Errorf("Distort (no media) sent %q, want the missing media note", texts)
	}

	// Documents and video notes are not distorted.
	client.clearCalls()
	note := &gotgbot.Message{VideoNote: &gotgbot.VideoNote{FileId: "vn1"}}
	if err := Distort(bot, testReplyContext(bot, "/distort", testUser(42), note)); err != nil {
		t.Fatalf("Distort (video note): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Неподдерживаемая операция") {
		t.Errorf("Distort (video note) sent %q, want the unsupported-operation note", texts)
	}

	// One job at a time: while the flag is set the command answers it is busy.
	client.clearCalls()
	DistortBusy = true
	input := ffmpegMedia(t, "audio")
	fakeMediaFile(t, client, input)
	if err := Distort(bot, testReplyContext(bot, "/distort", testUser(42),
		&gotgbot.Message{Audio: &gotgbot.Audio{FileId: "aud1"}})); err != nil {
		t.Fatalf("Distort (busy): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Команда занята") {
		t.Errorf("Distort (busy) sent %q, want the busy note", texts)
	}
	DistortBusy = false

	// A cached result is sent straight away, without touching ffmpeg. The
	// member status is pinned to creator so the result always goes into the
	// chat: the private-message branch depends on the hour of the day.
	client.clearCalls()
	client.setMemberStatus(42, "creator")
	DistortCache["aud1"] = "doc1"
	if err := Distort(bot, testReplyContext(bot, "/distort", testUser(42),
		&gotgbot.Message{Audio: &gotgbot.Audio{FileId: "aud1"}})); err != nil {
		t.Fatalf("Distort (cache): %v", err)
	}
	calls := client.callsFor("sendDocument")
	if len(calls) != 1 {
		t.Fatalf("Distort (cache): sendDocument calls = %d, want 1", len(calls))
	}
	// The file id sits in an unexported field of the FileReader value, so the
	// printed form is what can be checked here.
	if got := fmt.Sprintf("%+v", calls[0].Params["document"]); !strings.Contains(got, "doc1") {
		t.Errorf("Distort (cache): document = %v, want the cached file id", calls[0].Params["document"])
	}
	if calls[0].Params["chat_id"] != int64(-100) {
		t.Errorf("Distort (cache): chat_id = %v, want the chat of the request", calls[0].Params["chat_id"])
	}
	if texts := client.sentTexts(); len(texts) != 0 {
		t.Errorf("Distort (cache) sent %q, want only the cached document", texts)
	}
	delete(DistortCache, "aud1")

	// The work directory is created per file id, and a failure to create it is
	// reported as the file being in progress already. A file id with a slash in
	// it makes that plain Mkdir fail, which is the cheapest way to reach the
	// guard: the handler wipes its parent directory when it exists, so a marker
	// file left there would not survive to the check.
	client.clearCalls()
	parent := fmt.Sprintf("%v/telegram-go-chatbot-distort", DistortTempDir)
	os.RemoveAll(parent)
	if err := Distort(bot, testReplyContext(bot, "/distort", testUser(42),
		&gotgbot.Message{Audio: &gotgbot.Audio{FileId: "aud/1"}})); err != nil {
		t.Fatalf("Distort (work dir): %v", err)
	}
	texts = client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Обработка файла уже выполняется") {
		t.Errorf("Distort (work dir) sent %q, want the in-progress note", texts)
	}
	if DistortBusy {
		t.Errorf("Distort (work dir): the busy flag is left set")
	}
	os.RemoveAll(parent)

	// The audio path distorts the file with the vibrato filter and sends it.
	client.clearCalls()
	input = ffmpegMedia(t, "audio")
	fakeMediaFile(t, client, input)
	if err := Distort(bot, testReplyContext(bot, "/distort", testUser(42),
		&gotgbot.Message{Audio: &gotgbot.Audio{FileId: "aud1"}})); err != nil {
		t.Fatalf("Distort (audio): %v", err)
	}
	if calls := client.callsFor("sendAudio"); len(calls) != 1 {
		t.Errorf("Distort (audio): sendAudio calls = %d, want 1", len(calls))
	}
	if DistortCache["aud1"] != "distorted1" {
		t.Errorf("Distort (audio): cache = %q, want the file id of the result", DistortCache["aud1"])
	}
	if DistortBusy {
		t.Errorf("Distort (audio): the busy flag is left set")
	}
}

// TestMp3 covers /mp3 without yt-dlp: the usage note, the missing link and the
// download failure. The reaction on the requested message is sent first.
func TestMp3(t *testing.T) {
	bot, client := setupTestEnv(t)

	// Nothing to download from.
	if err := Mp3(bot, testContext(bot, "/mp3", testUser(42))); err != nil {
		t.Fatalf("Mp3 (no reply): %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.Contains(texts[0], "Пример использования") {
		t.Errorf("Mp3 (no reply) sent %q, want the usage note", texts)
	}

	// An argument without a link in it is reported as an error, after the
	// "watching" reaction is put on the message.
	client.clearCalls()
	if err := Mp3(bot, testContext(bot, "/mp3 не ссылка", testUser(42))); err == nil ||
		!strings.Contains(err.Error(), "ссылок не найдено") {
		t.Errorf("Mp3 (no link): err = %v, want the missing link error", err)
	}
	if calls := client.callsFor("setMessageReaction"); len(calls) != 1 {
		t.Errorf("Mp3 (no link): setMessageReaction calls = %d, want 1", len(calls))
	}

	// A link yt-dlp cannot handle ends with the downloader error, and no audio
	// is sent.
	client.clearCalls()
	msg := testMessage("/mp3 https://example.invalid/watch", testUser(42))
	msg.Entities = []gotgbot.MessageEntity{{Type: "url", Offset: 5, Length: int64(len("https://example.invalid/watch")), Url: "https://example.invalid/watch"}}
	if err := Mp3(bot, ext.NewContext(bot, &gotgbot.Update{UpdateId: 1, Message: msg}, nil)); err == nil {
		t.Errorf("Mp3 (bad link): err = nil, want the downloader error")
	}
	if calls := client.callsFor("sendAudio"); len(calls) != 0 {
		t.Errorf("Mp3 (bad link): sendAudio calls = %d, want 0", len(calls))
	}
}

// TestMp3LocalSource runs /mp3 end to end with the real yt-dlp: the m4a
// fixture served on the loopback address is picked by the format filter of the
// handler, ffmpeg extracts the mp3, and the audio is sent with the title
// yt-dlp reported. The temp file of the download is removed behind the handler.
func TestMp3LocalSource(t *testing.T) {
	bot, client := setupTestEnv(t)

	source := ytdlpFixtureServer(t, "audio") + "/audio.m4a"

	client.clearCalls()
	// The handler closes and removes the file it sends, so its contents are
	// read at the moment of the send.
	var sent []byte
	client.Observe = func(method string, params map[string]any) {
		if method != "sendAudio" {
			return
		}
		if file, ok := params["audio"].(*gotgbot.FileReader); ok {
			sent, _ = io.ReadAll(file.Data)
		}
	}
	msg := testMessage("/mp3 "+source, testUser(42))
	// The handler names the download after the chat and the message, so the test
	// uses ids of its own: parallel runs of the suite must not share one temp file.
	chat, msgId := int64(-1000-os.Getpid()), int64(101)
	msg.Chat.Id, msg.MessageId = chat, msgId
	msg.Entities = []gotgbot.MessageEntity{{Type: "url", Offset: 5, Length: int64(len(source)), Url: source}}
	if err := Mp3(bot, ext.NewContext(bot, &gotgbot.Update{UpdateId: 1, Message: msg}, nil)); err != nil {
		t.Fatalf("Mp3 (local source): %v", err)
	}

	calls := client.callsFor("sendAudio")
	if len(calls) != 1 {
		t.Fatalf("Mp3 (local source): sendAudio calls = %d, want 1", len(calls))
	}
	file, ok := calls[0].Params["audio"].(*gotgbot.FileReader)
	if !ok {
		t.Fatalf("Mp3 (local source): audio = %T, want the converted file", calls[0].Params["audio"])
	}
	if file.Name != "audio.mp3" {
		t.Errorf("Mp3 (local source) file name = %q, want the title yt-dlp reported plus the mp3 extension", file.Name)
	}
	if len(sent) == 0 {
		t.Errorf("Mp3 (local source) sent %d bytes, want a converted audio file", len(sent))
	}
	sentPath := filepath.Join(t.TempDir(), "sent.mp3")
	if err := os.WriteFile(sentPath, sent, 0o644); err != nil {
		t.Fatalf("Mp3 (local source): %v", err)
	}
	if values := ffprobeStreamValues(t, sentPath, "codec_name"); !slices.Contains(values, "mp3") {
		t.Errorf("Mp3 (local source) streams = %v, want the audio converted to mp3", values)
	}
	if calls[0].Params["title"] != "audio" {
		t.Errorf("Mp3 (local source) title = %v, want the title yt-dlp reported", calls[0].Params["title"])
	}
	if _, err := os.Stat(filepath.Join(os.TempDir(), fmt.Sprintf("%v_%v.mp3", chat, msgId))); !os.IsNotExist(err) {
		t.Errorf("Mp3 (local source): the temp file is left behind: %v", err)
	}
	if n := len(client.callsFor("sendChatAction")); n < 1 {
		t.Errorf("Mp3 (local source): sendChatAction calls = %d, want at least 1", n)
	}
}

// TestFFmpegRemoteMedia covers the branch of FFmpegConvert that every real
// conversion takes: the Bot API answers with a file_path the process does not
// have, so the file is downloaded from the Telegram file server over TLS,
// converted, and the downloaded input is removed afterwards.
func TestFFmpegRemoteMedia(t *testing.T) {
	bot, client := setupTestEnv(t)

	source := ffmpegMedia(t, "video")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("generated media: %v", err)
	}

	// A path under the temp dir that does not exist yet: FFmpegConvert creates
	// the directory and downloads the file into it.
	remote := filepath.Join(t.TempDir(), "remote", "video.mp4")
	client.setResult("getFile", json.RawMessage(fmt.Sprintf(`{"file_path":%q,"file_size":%d}`, remote, len(data))))

	oldToken := Config.Token
	t.Cleanup(func() { Config.Token = oldToken })
	Config.Token = "TESTTOKEN"

	var mu sync.Mutex
	var paths []string
	stubTLSProxy(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "api.telegram.org" {
			http.Error(w, "unexpected host "+r.Host, http.StatusNotFound)
			return
		}
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		if !strings.Contains(r.URL.Path, "/file/botTESTTOKEN/") {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Write(data)
	})

	reply := &gotgbot.Message{Video: &gotgbot.Video{FileId: "vid1"}}
	if err := Convert(bot, testReplyContext(bot, "/convert gif", testUser(42), reply)); err != nil {
		t.Fatalf("Convert (remote file): %v", err)
	}
	if calls := client.callsFor("sendAnimation"); len(calls) != 1 {
		t.Errorf("Convert (remote file): sendAnimation calls = %d, want 1", len(calls))
	}

	mu.Lock()
	defer mu.Unlock()
	if len(paths) == 0 {
		t.Errorf("Convert (remote file): the Telegram file server was never asked, want a download")
	}
	for _, p := range paths {
		if !strings.Contains(p, remote) {
			t.Errorf("Convert (remote file): download path = %q, want it to end at %v", p, remote)
		}
	}

	// The downloaded input is cleaned up like the conversion result.
	if _, err := os.Stat(remote); !os.IsNotExist(err) {
		t.Errorf("Convert (remote file): the downloaded input %v was left behind (stat: %v)", remote, err)
	}
}

// TestFFmpegRemoteMediaFailure covers the two ways the download of a media file
// ends in an error: the file server refuses the request, and the path the Bot
// API returned is unstatable for a reason other than "missing" (its parent is a
// regular file), which is not treated as a file to download.
func TestFFmpegRemoteMediaFailure(t *testing.T) {
	bot, client := setupTestEnv(t)

	oldToken := Config.Token
	t.Cleanup(func() { Config.Token = oldToken })
	Config.Token = "TESTTOKEN"

	reply := &gotgbot.Message{Video: &gotgbot.Video{FileId: "vid1"}}

	// The file server refuses: the conversion stops with the download error.
	remote := filepath.Join(t.TempDir(), "remote", "video.mp4")
	client.setResult("getFile", json.RawMessage(fmt.Sprintf(`{"file_path":%q,"file_size":1}`, remote)))
	stubTLSProxy(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "file reference has expired", http.StatusNotFound)
	})

	if err := Convert(bot, testReplyContext(bot, "/convert gif", testUser(42), reply)); err == nil {
		t.Errorf("Convert (expired file): err = nil, want the download error")
	}
	if calls := client.callsFor("sendAnimation"); len(calls) != 0 {
		t.Errorf("Convert (expired file): sendAnimation calls = %d, want 0", len(calls))
	}

	// The parent of the path is a regular file: os.Stat reports ENOTDIR, which
	// is not os.ErrNotExist, so nothing is downloaded and nothing is converted.
	notadir := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(notadir, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write stub file: %v", err)
	}
	client.setResult("getFile", json.RawMessage(fmt.Sprintf(`{"file_path":%q,"file_size":1}`, filepath.Join(notadir, "video.mp4"))))
	client.clearCalls()

	if err := Convert(bot, testReplyContext(bot, "/convert gif", testUser(42), reply)); err == nil {
		t.Errorf("Convert (unstatable path): err = nil, want the stat error")
	}
	if calls := client.callsFor("sendAnimation"); len(calls) != 0 {
		t.Errorf("Convert (unstatable path): sendAnimation calls = %d, want 0", len(calls))
	}
}

// TestLoopAnimation covers the /loop conversion itself: the looped animation
// for the default target and for an explicit argument, and the arguments the
// command refuses before ffmpeg is started. The typing goroutine keeps sending
// chat actions while ffmpeg works, so the action count is a lower bound.
func TestLoopAnimation(t *testing.T) {
	bot, client := setupTestEnv(t)

	for _, c := range []struct {
		name string
		cmd  string
	}{
		{"animation reply", "/loop"},
		{"animation argument", "/loop animation"},
	} {
		client.clearCalls()
		fakeMediaFile(t, client, ffmpegMedia(t, "video_only"))

		if err := Loop(bot, testReplyContext(bot, c.cmd, testUser(42),
			&gotgbot.Message{Animation: &gotgbot.Animation{FileId: "anim1"}})); err != nil {
			t.Errorf("Loop (%s): %v", c.name, err)
			continue
		}
		if calls := client.callsFor("sendAnimation"); len(calls) != 1 {
			t.Errorf("Loop (%s): sendAnimation calls = %d, want 1", c.name, len(calls))
		}
		if calls := client.callsFor("sendChatAction"); len(calls) < 1 {
			t.Errorf("Loop (%s): sendChatAction calls = %d, want at least 1", c.name, len(calls))
		}
	}

	// Only animations are looped: naming another target is refused, and
	// nothing is sent.
	for _, arg := range []string{"video", "mp3", "photo"} {
		client.clearCalls()
		fakeMediaFile(t, client, ffmpegMedia(t, "video_only"))

		if err := Loop(bot, testReplyContext(bot, "/loop "+arg, testUser(42),
			&gotgbot.Message{Animation: &gotgbot.Animation{FileId: "anim1"}})); err != nil {
			t.Errorf("Loop (%s): %v", arg, err)
			continue
		}
		texts := client.sentTexts()
		if len(texts) != 1 || texts[0] != "Неподдерживаемая операция" {
			t.Errorf("Loop (%s) sent %q, want the unsupported operation note", arg, texts)
		}
		if calls := client.callsFor("sendAnimation"); len(calls) != 0 {
			t.Errorf("Loop (%s): sendAnimation calls = %d, want 0", arg, len(calls))
		}
	}
}
