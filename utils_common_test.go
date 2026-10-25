package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestUserFullName(t *testing.T) {
	tests := []struct {
		name string
		user gotgbot.User
		want string
	}{
		{"only first name", gotgbot.User{FirstName: "Иван"}, "Иван"},
		{"first and last", gotgbot.User{FirstName: "Иван", LastName: "Пупкин"}, "Иван Пупкин"},
		{"empty user", gotgbot.User{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UserFullName(&tt.user); got != tt.want {
				t.Errorf("UserFullName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUserName(t *testing.T) {
	withUsername := gotgbot.User{Id: 1, FirstName: "Иван", LastName: "Пупкин", Username: "vpupkin"}
	if got := UserName(&withUsername); got != "vpupkin" {
		t.Errorf("UserName() = %q, want %q", got, "vpupkin")
	}

	withoutUsername := gotgbot.User{Id: 1, FirstName: "Иван", LastName: "Пупкин"}
	if got := UserName(&withoutUsername); got != "Иван Пупкин" {
		t.Errorf("UserName() = %q, want fallback to full name %q", got, "Иван Пупкин")
	}
}

func TestMentionUser(t *testing.T) {
	user := gotgbot.User{Id: 42, FirstName: "Иван", LastName: "Пупкин"}
	want := `<a href="tg://user?id=42">Иван Пупкин</a>`
	if got := MentionUser(&user); got != want {
		t.Errorf("MentionUser() = %q, want %q", got, want)
	}
}

func TestRandInt(t *testing.T) {
	tests := []struct {
		name     string
		min, max int
	}{
		{"positive range", 1, 5},
		{"single value", 7, 7},
		{"negative range", -3, 3},
		{"cross zero", -10, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen := make(map[int]bool)
			for i := 0; i < 1000; i++ {
				v := RandInt(tt.min, tt.max)
				if v < tt.min || v > tt.max {
					t.Fatalf("RandInt(%d, %d) = %d, out of range", tt.min, tt.max, v)
				}
				seen[v] = true
			}
			// For small ranges all values should appear at least once in 1000 draws.
			if tt.max-tt.min < 10 && len(seen) != tt.max-tt.min+1 {
				t.Errorf("RandInt(%d, %d): only %d of %d values seen in 1000 draws",
					tt.min, tt.max, len(seen), tt.max-tt.min+1)
			}
		})
	}
}

func TestStringInSlice(t *testing.T) {
	list := []string{"a", "b", "c"}
	if !StringInSlice("b", list) {
		t.Error("StringInSlice(\"b\", list) = false, want true")
	}
	if StringInSlice("d", list) {
		t.Error("StringInSlice(\"d\", list) = true, want false")
	}
	if StringInSlice("a", nil) {
		t.Error("StringInSlice(\"a\", nil) = true, want false")
	}
}

func TestIsAdmin(t *testing.T) {
	oldConfig := Config
	defer func() { Config = oldConfig }()
	Config = &Configuration{
		Admins: []int64{100, 200},
		Moders: []int64{300},
	}

	if !IsAdmin(100) {
		t.Error("IsAdmin(100) = false, want true")
	}
	if IsAdmin(300) {
		t.Error("IsAdmin(300) = true, want false (moder is not admin)")
	}
	if IsAdmin(999) {
		t.Error("IsAdmin(999) = true, want false")
	}
}

func TestIsAdminOrModer(t *testing.T) {
	oldConfig := Config
	defer func() { Config = oldConfig }()
	Config = &Configuration{
		Admins: []int64{100, 200},
		Moders: []int64{300},
	}

	for _, id := range []int64{100, 200, 300} {
		if !IsAdminOrModer(id) {
			t.Errorf("IsAdminOrModer(%d) = false, want true", id)
		}
	}
	if IsAdminOrModer(999) {
		t.Error("IsAdminOrModer(999) = true, want false")
	}
}

func TestRestrictionTimeMessage(t *testing.T) {
	now := time.Now().Unix()

	// Restriction in effect until the future timestamp -> message with date.
	got := RestrictionTimeMessage(now + 3600)
	if !strings.HasPrefix(got, " до ") {
		t.Errorf("RestrictionTimeMessage(future) = %q, want it to start with \" до \"", got)
	}

	// Already expired (or expiring within 30s) -> empty message.
	if got := RestrictionTimeMessage(now - 10); got != "" {
		t.Errorf("RestrictionTimeMessage(past) = %q, want empty", got)
	}
	if got := RestrictionTimeMessage(now + 10); got != "" {
		t.Errorf("RestrictionTimeMessage(now+10) = %q, want empty (within 30s grace)", got)
	}
}

func TestGetStartOfDay(t *testing.T) {
	got := GetStartOfDay()

	now := time.Now().Local()
	want := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).Unix()

	if got != want {
		t.Errorf("GetStartOfDay() = %d, want local midnight %d", got, want)
	}
}

type marshalInner struct {
	InnerSet   string `json:"inner_set"`
	InnerZero  string `json:"inner_zero"`
	InnerOmit  string `json:"-"`
	innerField string
}

type marshalOuter struct {
	Name    string `json:"name"`
	Zero    int    `json:"zero"`
	Tagged  string `json:"tagged_field,omitempty,string"`
	Skipped string `json:"-"`
	private string
	Nested  marshalInner   `json:"nested"`
	NoTag   bool           `json:""`
	Items   []marshalInner `json:"items"`
}

func TestMarshalOmitEmptyAll(t *testing.T) {
	v := marshalOuter{
		Name:    "test",
		Tagged:  "tag",
		Skipped: "gone",
		private: "gone",
		Nested:  marshalInner{InnerSet: "in", InnerOmit: "gone", innerField: "gone"},
		NoTag:   true,
	}

	data, err := MarshalOmitEmptyAll(v)
	if err != nil {
		t.Fatalf("MarshalOmitEmptyAll() error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	// Present fields.
	if got["name"] != "test" {
		t.Errorf(`got["name"] = %v, want "test"`, got["name"])
	}
	if got["tagged_field"] != "tag" {
		t.Errorf(`got["tagged_field"] = %v, want "tag"`, got["tagged_field"])
	}
	if got["NoTag"] != true {
		t.Errorf(`got["NoTag"] = %v, want true (empty json tag falls back to field name)`, got["NoTag"])
	}

	// Zero, json:"-" and unexported fields must be dropped.
	for _, key := range []string{"zero", "Skipped", "private", "gone"} {
		if _, ok := got[key]; ok {
			t.Errorf("key %q should be omitted, got: %s", key, data)
		}
	}

	// Nested struct: recursively stripped, present as an object.
	nested, ok := got["nested"].(map[string]any)
	if !ok {
		t.Fatalf(`got["nested"] = %v, want object`, got["nested"])
	}
	if nested["inner_set"] != "in" {
		t.Errorf(`nested["inner_set"] = %v, want "in"`, nested["inner_set"])
	}
	for _, key := range []string{"inner_zero", "InnerOmit", "innerField"} {
		if _, ok := nested[key]; ok {
			t.Errorf("nested key %q should be omitted, got: %s", key, data)
		}
	}
}

func TestMarshalOmitEmptyAll_PointerAndNonStruct(t *testing.T) {
	// Pointer to struct is handled like the struct itself.
	v := marshalOuter{Name: "ptr"}
	data, err := MarshalOmitEmptyAll(&v)
	if err != nil {
		t.Fatalf("MarshalOmitEmptyAll(ptr) error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if got["name"] != "ptr" {
		t.Errorf(`got["name"] = %v, want "ptr"`, got["name"])
	}

	// Non-struct falls back to standard marshaling.
	data, err = MarshalOmitEmptyAll([]string{"a", "b"})
	if err != nil {
		t.Fatalf("MarshalOmitEmptyAll(slice) error: %v", err)
	}
	if string(data) != `["a","b"]` {
		t.Errorf("MarshalOmitEmptyAll(slice) = %s, want [\"a\",\"b\"]", data)
	}
}

func TestIsContainsMedia(t *testing.T) {
	cases := []struct {
		name string
		msg  gotgbot.Message
		want bool
	}{
		{"text", gotgbot.Message{Text: "привет"}, false},
		{"photo", gotgbot.Message{Photo: []gotgbot.PhotoSize{{FileId: "p"}}}, true},
		{"voice", gotgbot.Message{Voice: &gotgbot.Voice{FileId: "v"}}, true},
		{"audio", gotgbot.Message{Audio: &gotgbot.Audio{FileId: "a"}}, true},
		{"animation", gotgbot.Message{Animation: &gotgbot.Animation{FileId: "an"}}, true},
		{"sticker", gotgbot.Message{Sticker: &gotgbot.Sticker{FileId: "s"}}, true},
		{"document", gotgbot.Message{Document: &gotgbot.Document{FileId: "d"}}, true},
		{"video", gotgbot.Message{Video: &gotgbot.Video{FileId: "vi"}}, true},
		{"video note", gotgbot.Message{VideoNote: &gotgbot.VideoNote{FileId: "vn"}}, true},
		// Attachments that are not files are not media for the bot.
		{"contact", gotgbot.Message{Contact: &gotgbot.Contact{}}, false},
		{"location", gotgbot.Message{Location: &gotgbot.Location{}}, false},
		{"poll", gotgbot.Message{Poll: &gotgbot.Poll{}}, false},
	}

	for _, c := range cases {
		if got := IsContainsMedia(&c.msg); got != c.want {
			t.Errorf("IsContainsMedia(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDownloadFile(t *testing.T) {
	stubProxy(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.RequestURI, "/ok.bin") {
			fmt.Fprint(w, "media bytes")
			return
		}
		http.Error(w, "file not found", http.StatusNotFound)
	})

	dir := t.TempDir()

	path := filepath.Join(dir, "ok.bin")
	if err := DownloadFile(path, "http://example.invalid/ok.bin"); err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("DownloadFile: read back: %v", err)
	}
	if string(body) != "media bytes" {
		t.Errorf("DownloadFile wrote %q, want the response body", body)
	}

	// An expired Bot API file link must not be written out as a media file.
	missing := filepath.Join(dir, "missing.bin")
	if err := DownloadFile(missing, "http://example.invalid/missing.bin"); err == nil ||
		!strings.Contains(err.Error(), "404") {
		t.Errorf("DownloadFile (404): err = %v, want the status error", err)
	}
	// The file is created before the request, so a failed download leaves it empty.
	if info, err := os.Stat(missing); err != nil || info.Size() != 0 {
		t.Errorf("DownloadFile (404): file = %v, want an empty file left behind", info)
	}
}

// ffmpegMedia writes a tiny media file with the system ffmpeg and returns its path.
func requireMediaTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		if os.Getenv("ZAVTRA_REQUIRE_MEDIA_TESTS") == "1" {
			t.Fatalf("required media tool %s is not installed: %v", name, err)
		}
		t.Skipf("%s is not installed", name)
	}
}

func ffmpegMedia(t *testing.T, kind string) string {
	t.Helper()
	requireMediaTool(t, "ffmpeg")
	requireMediaTool(t, "ffprobe")

	ext, inputs := "mp3", []string{"-f", "lavfi", "-i", "sine=frequency=440:duration=1"}
	switch kind {
	case "photo":
		ext, inputs = "jpg", []string{"-f", "lavfi", "-i", "testsrc=size=128x96:rate=1", "-frames:v", "1"}
	case "video":
		ext, inputs = "mp4", []string{"-f", "lavfi", "-i", "testsrc=duration=1:size=128x96:rate=10",
			"-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-shortest"}
	case "video_only":
		ext, inputs = "mp4", []string{"-f", "lavfi", "-i", "testsrc=duration=1:size=128x96:rate=10", "-an"}
	case "long_video":
		ext, inputs = "mp4", []string{"-f", "lavfi", "-i", "testsrc=duration=121:size=128x96:rate=10",
			"-f", "lavfi", "-i", "sine=frequency=440:duration=121"}
	}

	path := filepath.Join(t.TempDir(), "input_"+kind+"."+ext)
	cmd := exec.Command("ffmpeg", append([]string{"-hide_banner", "-loglevel", "fatal"}, append(inputs, []string{"-y", path}...)...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v\n%s", err, out)
	}
	return path
}

// ytdlpFixtureServer serves a tiny media fixture over plain HTTP on the
// loopback address, so the yt-dlp handlers run end to end without leaving the
// machine. The "audio" fixture is one m4a file: the format filter of /mp3
// picks it. The "video" fixture is an HLS stream with a video-only and an
// audio-only variant: the format filter of /download picks both and ffmpeg
// merges them into one mp4.
func ytdlpFixtureServer(t *testing.T, kind string) string {
	t.Helper()

	requireMediaTool(t, "yt-dlp")
	requireMediaTool(t, "ffmpeg")

	fixtures := map[string]struct {
		out  string
		args []string
	}{
		"audio": {"audio.m4a", []string{"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
			"-c:a", "aac", "-f", "mp4"}},
		"video": {"av%v.m3u8", []string{"-f", "lavfi", "-i", "testsrc=duration=1:size=128x96:rate=10",
			"-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-map", "0:v", "-map", "1:a",
			"-c:v", "libx264", "-c:a", "aac", "-var_stream_map", "v:0 a:0", "-f", "hls",
			"-hls_time", "1", "-hls_playlist_type", "vod", "-hls_segment_type", "fmp4",
			"-master_pl_name", "master.m3u8", "-hls_segment_filename", "seg%v%d.m4s"}},
	}
	fixture, ok := fixtures[kind]
	if !ok {
		t.Fatalf("no yt-dlp fixture %q", kind)
	}

	dir := t.TempDir()
	cmd := exec.Command("ffmpeg", append([]string{"-hide_banner", "-loglevel", "fatal"},
		append(fixture.args, "-y", fixture.out)...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		if os.Getenv("ZAVTRA_REQUIRE_MEDIA_TESTS") == "1" {
			t.Fatalf("required %v fixture failed: %v\n%s", kind, err, out)
		}
		t.Skipf("ffmpeg cannot build the %v fixture: %v\n%s", kind, err, out)
	}

	server := httptest.NewServer(http.FileServer(http.Dir(dir)))
	t.Cleanup(server.Close)
	return server.URL
}

// ffprobeStreamValues lists one ffprobe field (codec_name, codec_type, …) of
// every stream of a file, so a test can check what a conversion produced.
func ffprobeStreamValues(t *testing.T, path string, entry string) []string {
	t.Helper()

	requireMediaTool(t, "ffprobe")
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream="+entry,
		"-of", "csv=p=0", path).CombinedOutput()
	if err != nil {
		t.Fatalf("ffprobe stream=%s of %v: %v\n%s", entry, path, err, out)
	}
	var values []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			values = append(values, strings.TrimSpace(line))
		}
	}
	return values
}

// sentFilePaths lists the local files the fake client was asked to send with
// the given method. FFmpegConvert opens its result file and hands the handle
// over, so the reader name is the path that was sent. Tests check the paths of
// their own conversions instead of globbing the temp dir: result names are
// unique per conversion, so a conversion running in another process is a
// different file that this test has no business inspecting.
func sentFilePaths(client *fakeBotClient, method string) []string {
	var paths []string
	for _, call := range client.callsFor(method) {
		file, ok := call.Params[strings.ToLower(strings.TrimPrefix(method, "send"))].(*gotgbot.FileReader)
		if !ok {
			continue
		}
		if path, ok := file.Data.(interface{ Name() string }); ok {
			paths = append(paths, path.Name())
		}
	}
	return paths
}

// TestFFmpegConvert runs the real ffmpeg/ffprobe on generated 1-second files.
// The conversions the bot offers fall back between tracks: an audio file asked
// to become a video is sent as audio, a video without audio asked to become an
// mp3 is sent as an animation.
func TestFFmpegConvert(t *testing.T) {
	bot, client := setupTestEnv(t)

	var converted []string

	for _, c := range []struct {
		name, kind, targetType, method string
	}{
		{"audio to mp3", "audio", "mp3", "sendAudio"},
		{"audio to ogg", "audio", "ogg", "sendVoice"},
		{"video to mp4", "video", "mp4", "sendVideo"},
		{"video to gif", "video", "gif", "sendAnimation"},
		{"video to jpg", "video", "jpg", "sendDocument"},
		{"audio asked for a video becomes audio", "audio", "video", "sendAudio"},
		{"video without audio asked for mp4 becomes an animation", "video_only", "mp4", "sendAnimation"},
	} {
		input := ffmpegMedia(t, c.kind)
		media := Media{FileName: filepath.Base(input), FilePath: input}

		if err := FFmpegConvert(testContext(bot, "/convert", testUser(42)), media, c.targetType); err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		calls := client.callsFor(c.method)
		if len(calls) != 1 {
			t.Errorf("%s: %s calls = %d, want 1", c.name, c.method, len(calls))
			continue
		}
		if calls[0].Params["chat_id"] != int64(-100) {
			t.Errorf("%s: %s chat_id = %v, want -100", c.name, c.method, calls[0].Params["chat_id"])
		}
		converted = append(converted, sentFilePaths(client, c.method)...)
		client.Calls = nil
	}

	// A video without an audio track cannot be converted to audio formats.
	videoOnly := ffmpegMedia(t, "video_only")
	if err := FFmpegConvert(testContext(bot, "/convert", testUser(42)),
		Media{FileName: filepath.Base(videoOnly), FilePath: videoOnly}, "mp3"); err == nil ||
		!strings.Contains(err.Error(), "аудио-дорожка") {
		t.Errorf("FFmpegConvert (video only to mp3): err = %v, want the missing audio track", err)
	}

	// An audio file has no video track for the animation targets.
	audio := ffmpegMedia(t, "audio")
	if err := FFmpegConvert(testContext(bot, "/convert", testUser(42)),
		Media{FileName: filepath.Base(audio), FilePath: audio}, "gif"); err == nil ||
		!strings.Contains(err.Error(), "видео-дорожка") {
		t.Errorf("FFmpegConvert (audio to gif): err = %v, want the missing video track", err)
	}

	// Reverse and loop effects are refused on long videos.
	long := ffmpegMedia(t, "long_video")
	if err := FFmpegConvert(testContext(bot, "/convert", testUser(42)),
		Media{FileName: filepath.Base(long), FilePath: long}, "reverse"); err == nil ||
		!strings.Contains(err.Error(), "слишком длинное") {
		t.Errorf("FFmpegConvert (2 minutes to reverse): err = %v, want the length refusal", err)
	}

	// An unknown target type is reported, not sent as a document.
	video := ffmpegMedia(t, "video")
	if err := FFmpegConvert(testContext(bot, "/convert", testUser(42)),
		Media{FileName: filepath.Base(video), FilePath: video}, "xyz"); err == nil ||
		!strings.Contains(err.Error(), "not supported") {
		t.Errorf("FFmpegConvert (xyz): err = %v, want the unsupported target", err)
	}

	// The converted files are removed after sending.
	if len(converted) != 7 {
		t.Errorf("converted files = %d, want the %d conversions of the loop", len(converted), 7)
	}
	for _, path := range converted {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("converted file left after sending: %v", path)
		}
	}
}

// TestFFmpegConvertConcurrentSameName converts four copies of one file at the
// same time, the way four conversations converting one viral file do. Every
// conversion needs its own result name: a name built from the input name and
// the current second made them write one and the same file, and the first to
// finish removed the file the others were about to send.
func TestFFmpegConvertConcurrentSameName(t *testing.T) {
	bot, client := setupTestEnv(t)

	source := ffmpegMedia(t, "video_only")
	name := filepath.Base(source)
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("generated video: %v", err)
	}

	const runs = 4
	media := make([]Media, runs)
	for i := range media {
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("copy the generated video: %v", err)
		}
		media[i] = Media{FileName: name, FilePath: path}
	}
	client.clearCalls()

	errs := make([]error, runs)
	var wg sync.WaitGroup
	for i := range media {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = FFmpegConvert(testContext(bot, "/convert", testUser(42)), media[i], "mp4")
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("conversion %v of %v: %v", i+1, runs, err)
		}
	}

	// Every conversion must have sent a result file of its own.
	sent := client.callsFor("sendAnimation")
	if len(sent) != runs {
		t.Fatalf("sendAnimation calls = %d, want %d", len(sent), runs)
	}
	paths := sentFilePaths(client, "sendAnimation")
	if len(paths) != runs {
		t.Fatalf("result files = %v, want %v opened result files", paths, runs)
	}
	unique := map[string]int{}
	for _, path := range paths {
		unique[path]++
	}
	if len(unique) != runs {
		t.Errorf("result files = %v, want %v distinct names", paths, runs)
	}

	for path := range unique {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("converted file left after sending: %v", path)
		}
	}
}

// TestGetMedia covers the media picker: the largest photo size, the fields
// copied from the message, the Bot API file path, the file cached in
// bot_api_datadir and the getFile failure.
func TestGetMedia(t *testing.T) {
	_, client := setupTestEnv(t)

	oldDataDir, oldToken := Config.BotApiDataDir, Config.Token
	t.Cleanup(func() { Config.BotApiDataDir, Config.Token = oldDataDir, oldToken })
	Config.BotApiDataDir = t.TempDir()
	Config.Token = "tok"

	client.setResult("getFile", json.RawMessage(`{"file_path":"photos/p.jpg","file_size":10}`))

	msg := &gotgbot.Message{
		Photo: []gotgbot.PhotoSize{
			{FileId: "small", FileSize: 10},
			{FileId: "large", FileSize: 100, Width: 8, Height: 6},
			{FileId: "mid", FileSize: 50},
		},
		Caption:               "подпись",
		HasMediaSpoiler:       true,
		ShowCaptionAboveMedia: true,
		MediaGroupId:          "g1",
	}

	media, err := GetMedia(msg)
	if err != nil {
		t.Fatalf("GetMedia (photo): %v", err)
	}
	if media.Type != "photo" || media.FileID != "large" {
		t.Errorf("GetMedia (photo) = %v/%v, want the largest photo size", media.Type, media.FileID)
	}
	if media.Width != 8 || media.Height != 6 || media.FileSize != 100 {
		t.Errorf("GetMedia (photo) = %vx%v, %v bytes, want 8x6 and 100 bytes", media.Width, media.Height, media.FileSize)
	}
	if media.Caption != "подпись" || !media.HasSpoiler || !media.ShowCaptionAboveMedia || media.MediaGroupId != "g1" {
		t.Errorf("GetMedia (photo) caption fields = %+v, want the caption fields of the message", media)
	}
	if media.FilePath != "photos/p.jpg" || media.FileName != "p.jpg" {
		t.Errorf("GetMedia (photo) = %q/%q, want the Bot API file path and its base name", media.FilePath, media.FileName)
	}
	if media.FileURL == "" {
		t.Errorf("GetMedia (photo): FileURL is empty")
	}

	// A file cached under bot_api_datadir/<token> wins over the API path.
	local := filepath.Join(Config.BotApiDataDir, Config.Token, "photos", "p.jpg")
	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		t.Fatalf("create datadir: %v", err)
	}
	if err := os.WriteFile(local, []byte("cached"), 0o644); err != nil {
		t.Fatalf("write the cached file: %v", err)
	}
	media, err = GetMedia(msg)
	if err != nil {
		t.Fatalf("GetMedia (cached): %v", err)
	}
	if media.FilePath != local {
		t.Errorf("GetMedia (cached) FilePath = %q, want the datadir file %q", media.FilePath, local)
	}

	for _, c := range []struct {
		name, wantType, wantID string
		msg                    gotgbot.Message
	}{
		{"voice", "voice", "v", gotgbot.Message{Voice: &gotgbot.Voice{FileId: "v", Duration: 3}}},
		{"audio", "audio", "a", gotgbot.Message{Audio: &gotgbot.Audio{FileId: "a", Duration: 4}}},
		{"animation", "animation", "an", gotgbot.Message{Animation: &gotgbot.Animation{FileId: "an", Duration: 5}}},
		{"sticker", "sticker", "s", gotgbot.Message{Sticker: &gotgbot.Sticker{FileId: "s"}}},
		{"video", "video", "vi", gotgbot.Message{Video: &gotgbot.Video{FileId: "vi", Duration: 6}}},
		{"video note", "video_note", "vn", gotgbot.Message{VideoNote: &gotgbot.VideoNote{FileId: "vn", Duration: 7}}},
		{"document", "document", "d", gotgbot.Message{Document: &gotgbot.Document{FileId: "d"}}},
	} {
		m, err := GetMedia(&c.msg)
		if err != nil {
			t.Errorf("GetMedia (%s): %v", c.name, err)
			continue
		}
		if m.Type != c.wantType || m.FileID != c.wantID {
			t.Errorf("GetMedia (%s) = %v/%v, want %v/%v", c.name, m.Type, m.FileID, c.wantType, c.wantID)
		}
	}

	// The file path comes from the Bot API, so its failure is fatal.
	client.setError("getFile", errors.New("file is gone"))
	if _, err := GetMedia(msg); err == nil {
		t.Errorf("GetMedia (getFile failure): err = nil, want the API error")
	}
}

// TestReplyAndRemove covers the self-destructing answer: the note is appended to
// the text, and the deletion is scheduled only for a message with a real id. The
// 30 second timer itself is not waited out by the test.
func TestReplyAndRemove(t *testing.T) {
	bot, client := setupTestEnv(t)

	ctx := testContext(bot, "/foo", testUser(42))
	ctx.Message.MessageId = 0
	if err := ReplyAndRemove("готово", *ctx); err != nil {
		t.Fatalf("ReplyAndRemove: %v", err)
	}
	texts := client.sentTexts()
	if len(texts) != 1 || !strings.HasSuffix(texts[0], "готово\n\nЭто сообщение самоуничтожится через 30 секунд.") {
		t.Errorf("ReplyAndRemove sent %q, want the text with the self-destruct note", texts)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Errorf("ReplyAndRemove: deleteMessage calls = %d, want none for a message without an id", len(calls))
	}
}

// TestSysAdminReply covers where the operational notices go: an ephemeral
// message for the sysadmin in the main chat, the sysadmin's private chat when
// no main chat is configured (ephemeral messages do not exist there), and
// nothing at all when there is no sysadmin or no bot to send with.
func TestSysAdminReply(t *testing.T) {
	_, client := setupTestEnv(t)

	if err := SysAdminReply("готово"); err != nil {
		t.Fatalf("SysAdminReply: %v", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("SysAdminReply sendMessage calls = %d, want 1", len(calls))
	}
	if calls[0].Params["chat_id"] != int64(Config.Chat) {
		t.Errorf("SysAdminReply chat_id = %v, want the main chat %d", calls[0].Params["chat_id"], Config.Chat)
	}
	eph, ok := calls[0].Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
	if !ok || eph.ReceiverUserId != int64(Config.SysAdmin) {
		t.Errorf("SysAdminReply ephemeral_message_parameters = %v, want the sysadmin %d", calls[0].Params["ephemeral_message_parameters"], Config.SysAdmin)
	}
	if texts := client.sentTexts(); len(texts) != 1 || texts[0] != "готово" {
		t.Errorf("SysAdminReply sent %q, want the text unchanged", texts)
	}

	// Without a main chat the notice goes to the private chat, where the
	// ephemeral receiver is not part of the request.
	client.Calls = nil
	Config.Chat = 0
	if err := SysAdminReply("личкой"); err != nil {
		t.Fatalf("SysAdminReply (no main chat): %v", err)
	}
	calls = client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("SysAdminReply (no main chat) sendMessage calls = %d, want 1", len(calls))
	}
	if calls[0].Params["chat_id"] != int64(Config.SysAdmin) {
		t.Errorf("SysAdminReply (no main chat) chat_id = %v, want the sysadmin %d", calls[0].Params["chat_id"], Config.SysAdmin)
	}
	if _, ok := calls[0].Params["ephemeral_message_parameters"]; ok {
		t.Errorf("SysAdminReply (no main chat) sent ephemeral_message_parameters, want a plain private message")
	}

	// A chat that refuses the message answers with the API error: the caller
	// decides what to do with it, ErrorReporting must not feed on it.
	client.Calls = nil
	client.setError("sendMessage", errors.New("Bad Request: BOT_NOT_ADMIN"))
	if err := SysAdminReply("готово"); err == nil {
		t.Errorf("SysAdminReply (send failure): err = nil, want the API error")
	}

	// No sysadmin, no bot: nothing is sent.
	client.Calls = nil
	client.Errors = nil
	Config.SysAdmin = 0
	if err := SysAdminReply("готово"); err != nil {
		t.Errorf("SysAdminReply (no sysadmin): %v", err)
	}
	Config.SysAdmin = 100
	Bot = nil
	if err := SysAdminReply("готово"); err != nil {
		t.Errorf("SysAdminReply (no bot): %v", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 0 {
		t.Errorf("SysAdminReply: sendMessage calls = %d, want none without a sysadmin or a bot", len(calls))
	}
}

// TestStatsCounters covers the counters behind the statistics: statsIncrease
// creates the row of a day and increments it, getWordID stores one row per
// stemmed word, and the refusal and blessing pools answer from the database.
func TestStatsCounters(t *testing.T) {
	setupTestEnv(t)

	day := GetStartOfDay()

	// The first call of a day creates the row with a count of one.
	statsIncrease(1, day, 42)
	var row Stats
	if err := DB.Where("stat_type = ? AND day_timestamp = ? AND context_id = ?", 1, day, 42).First(&row).Error; err != nil {
		t.Fatalf("statsIncrease: %v", err)
	}
	if row.Count != 1 {
		t.Errorf("statsIncrease: Count = %v, want 1", row.Count)
	}
	if row.LastUpdate <= 0 {
		t.Errorf("statsIncrease: LastUpdate = %v, want the current time", row.LastUpdate)
	}

	// The next calls increment the same row.
	statsIncrease(1, day, 42)
	statsIncrease(1, day, 42)
	row = Stats{}
	if err := DB.Where("stat_type = ? AND day_timestamp = ? AND context_id = ?", 1, day, 42).First(&row).Error; err != nil {
		t.Fatalf("statsIncrease: %v", err)
	}
	if row.Count != 3 {
		t.Errorf("statsIncrease: Count = %v, want 3", row.Count)
	}

	// Another context is a row of its own.
	statsIncrease(1, day, 43)
	row = Stats{}
	if err := DB.Where("stat_type = ? AND day_timestamp = ? AND context_id = ?", 1, day, 43).First(&row).Error; err != nil {
		t.Fatalf("statsIncrease: %v", err)
	}
	if row.Count != 1 {
		t.Errorf("statsIncrease (other context): Count = %v, want 1", row.Count)
	}
	row = Stats{}
	if err := DB.Where("stat_type = ? AND day_timestamp = ? AND context_id = ?", 1, day, 42).First(&row).Error; err != nil {
		t.Fatalf("statsIncrease: %v", err)
	}
	if row.Count != 3 {
		t.Errorf("statsIncrease: Count = %v, want the first context to stay at 3", row.Count)
	}

	// statsAdd increases the counter by an arbitrary amount: mute time (stat
	// type 7) counts seconds, so one call can add minutes at once.
	statsAdd(7, day, 42, 600)
	statsAdd(7, day, 42, 60)
	row = Stats{}
	if err := DB.Where("stat_type = ? AND day_timestamp = ? AND context_id = ?", 7, day, 42).First(&row).Error; err != nil {
		t.Fatalf("statsAdd: %v", err)
	}
	if row.Count != 660 {
		t.Errorf("statsAdd: Count = %v, want 660", row.Count)
	}

	// Words are stored once per stem, and the stem of the same word is stable.
	first := getWordID("покупал")
	again := getWordID("покупал")
	if first == 0 || first != again {
		t.Errorf("getWordID = %v and %v, want the same row for the same word", first, again)
	}
	other := getWordID("кот")
	if other == 0 || other == first {
		t.Errorf("getWordID = %v and %v, want separate rows for different words", first, other)
	}
	var words []StatsWords
	if err := DB.Find(&words).Error; err != nil {
		t.Fatalf("read stats_words: %v", err)
	}
	if len(words) != 2 {
		t.Fatalf("stats_words rows = %d, want 2", len(words))
	}
	if words[0].Word != "покупал" || words[0].ShortWord == "" {
		t.Errorf("stats_words[0] = %+v, want the word with its stem", words[0])
	}

	// The pools are seeded by the test harness.
	if got := GetNope(); got != "нет" {
		t.Errorf("GetNope = %q, want the seeded refusal", got)
	}
	if got := GetBless(); got != "да" {
		t.Errorf("GetBless = %q, want the seeded blessing", got)
	}
}

// TestGetUserFromDB covers the user lookup behind /getid and friends: by
// @username in any case, by numeric id, and the answers for an empty string, a
// non-number, a zero id and an unknown user.
func TestGetUserFromDB(t *testing.T) {
	setupTestEnv(t)
	seedUser(42, "testpupkin", "Тест")
	seedUser(77, "Vavka", "Вавка")

	// By username: the lookup lowercases both sides.
	user, err := GetUserFromDB("@testpupkin")
	if err != nil || user.Id != 42 {
		t.Errorf("GetUserFromDB(@testpupkin) = %+v, %v, want the user 42", user, err)
	}
	user, err = GetUserFromDB("@TESTPUPKIN")
	if err != nil || user.Id != 42 {
		t.Errorf("GetUserFromDB(@TESTPUPKIN) = %+v, %v, want the user 42", user, err)
	}

	// By id: the row is read back with its username.
	user, err = GetUserFromDB("77")
	if err != nil || user.Username != "Vavka" {
		t.Errorf("GetUserFromDB(77) = %+v, %v, want the user with the username Vavka", user, err)
	}

	// Nothing to look for, a word instead of an id, and an id of zero.
	if _, err := GetUserFromDB(""); err == nil {
		t.Errorf("GetUserFromDB (empty): err = nil, want the not-found error")
	}
	if _, err := GetUserFromDB("abc"); err == nil {
		t.Errorf("GetUserFromDB (not a number): err = nil, want the parse error")
	}
	if _, err := GetUserFromDB("0"); err == nil {
		t.Errorf("GetUserFromDB (zero): err = nil, want the not-found error")
	}

	// A number nobody has: the database reports no rows.
	if _, err := GetUserFromDB("99999"); err == nil {
		t.Errorf("GetUserFromDB (unknown id): err = nil, want the missing row error")
	}
}
