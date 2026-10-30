package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func setupDistortScenario(t *testing.T) (*gotgbot.Bot, *fakeBotClient) {
	t.Helper()
	bot, client := setupTestEnv(t)
	oldBusy, oldCache, oldDir := DistortBusy, DistortCache, DistortTempDir
	t.Cleanup(func() { DistortBusy, DistortCache, DistortTempDir = oldBusy, oldCache, oldDir })
	DistortBusy, DistortCache = false, map[string]string{}
	// Exercise argument boundaries as well as ordinary temporary paths.
	DistortTempDir = filepath.Join(t.TempDir(), "media jobs")
	if err := os.Mkdir(DistortTempDir, 0o700); err != nil {
		t.Fatal(err)
	}
	client.setMemberStatus(42, "creator")
	client.setResult("sendDocument", json.RawMessage(`{"message_id":1,"document":{"file_id":"distorted"}}`))
	return bot, client
}

func assertDistortCleanup(t *testing.T, fileID string) {
	t.Helper()
	if DistortBusy {
		t.Fatal("failed job left the handler busy")
	}
	workdir := filepath.Join(DistortTempDir, "telegram-go-chatbot-distort", fileID)
	if _, err := os.Stat(workdir); !os.IsNotExist(err) {
		t.Fatalf("work directory was left behind: %v", err)
	}
}

func TestDistortVisualMedia(t *testing.T) {
	requireMediaTool(t, "convert")
	for _, kind := range []string{"photo", "video_only", "video"} {
		t.Run(kind, func(t *testing.T) {
			bot, client := setupDistortScenario(t)
			input := ffmpegMedia(t, kind)
			fakeMediaFile(t, client, input)
			reply := &gotgbot.Message{Video: &gotgbot.Video{FileId: "visual"}}
			if kind == "photo" {
				reply = &gotgbot.Message{Photo: []gotgbot.PhotoSize{{FileId: "visual"}}}
			}
			var sent []byte
			client.Observe = func(method string, params map[string]any) {
				if method == "sendDocument" {
					file := params["document"].(*gotgbot.FileReader)
					sent, _ = io.ReadAll(file.Data)
				}
			}
			if err := Distort(bot, testReplyContext(bot, "/distort", testUser(42), reply)); err != nil {
				t.Fatal(err)
			}
			if len(client.callsFor("sendDocument")) != 1 || len(sent) == 0 || DistortCache["visual"] != "distorted" {
				t.Fatal("visual result was not sent and cached")
			}
			output := filepath.Join(t.TempDir(), "result.mp4")
			if err := os.WriteFile(output, sent, 0o600); err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(ffprobeStreamValues(t, output, "codec_name"), "h264") {
				t.Fatal("result is not a playable H.264 video")
			}
			if kind == "video" && !slices.Contains(ffprobeStreamValues(t, output, "codec_type"), "audio") {
				t.Fatal("video distortion lost its audio track")
			}
			assertDistortCleanup(t, "visual")
		})
	}
}

// Local-mode Bot API paths can belong to another container. Download the
// photo through the API's configured transport before handing it to FFmpeg.
func TestDistortRemotePhoto(t *testing.T) {
	requireMediaTool(t, "convert")
	for _, absolute := range []bool{false, true} {
		t.Run(fmt.Sprint(absolute), func(t *testing.T) {
			_, _ = setupDistortScenario(t)
			photo, err := os.ReadFile(ffmpegMedia(t, "photo"))
			if err != nil {
				t.Fatal(err)
			}
			path := "distort-remote/photos/input.jpg"
			if absolute {
				path = "/api/container/1/" + path
			}
			var sent []byte
			var downloads int
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Media-Transport") != "configured" {
					http.Error(w, "missing API transport", http.StatusForbidden)
					return
				}
				switch r.URL.Path {
				case "/botTESTTOKEN/getFile":
					fmt.Fprintf(w, `{"ok":true,"result":{"file_path":%q,"file_size":%d}}`, path, len(photo))
				case "/botTESTTOKEN/getChatMember":
					fmt.Fprint(w, `{"ok":true,"result":{"status":"creator","user":{"id":42,"is_bot":false,"first_name":"member"}}}`)
				case "/file/botTESTTOKEN/distort-remote/photos/input.jpg":
					downloads++
					w.Write(photo)
				case "/botTESTTOKEN/sendDocument":
					file, _, err := r.FormFile("document")
					if err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
					defer file.Close()
					sent, _ = io.ReadAll(file)
					fmt.Fprint(w, `{"ok":true,"result":{"message_id":1,"document":{"file_id":"distorted"}}}`)
				case "/botTESTTOKEN/sendChatAction":
					fmt.Fprint(w, `{"ok":true,"result":true}`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client := *server.Client()
			transport := client.Transport
			client.Transport = quoteAvatarRoundTrip(func(r *http.Request) (*http.Response, error) {
				r.Header.Set("X-Media-Transport", "configured")
				return transport.RoundTrip(r)
			})
			bot := &gotgbot.Bot{User: gotgbot.User{Id: 1}, Token: "TESTTOKEN", BotClient: &gotgbot.BaseBotClient{
				Client: client, DefaultRequestOpts: &gotgbot.RequestOpts{APIURL: server.URL},
			}}
			Bot = bot
			t.Setenv("TMPDIR", t.TempDir())
			reply := &gotgbot.Message{Photo: []gotgbot.PhotoSize{{FileId: "remote"}}}
			if err := Distort(bot, testReplyContext(bot, "/distort", testUser(42), reply)); err != nil {
				t.Fatal(err)
			}
			if downloads != 1 || len(sent) == 0 || DistortCache["remote"] != "distorted" {
				t.Fatal("remote photo was not downloaded, distorted, sent and cached")
			}
			output := filepath.Join(t.TempDir(), "result.mp4")
			if err := os.WriteFile(output, sent, 0o600); err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(ffprobeStreamValues(t, output, "codec_name"), "h264") {
				t.Fatal("remote photo did not produce a playable H.264 video")
			}
			assertDistortCleanup(t, "remote")
			entries, err := os.ReadDir(os.TempDir())
			if err != nil || len(entries) != 0 {
				t.Fatalf("downloaded input was not cleaned up: %v, %v", entries, err)
			}
		})
	}
}

func TestDistortPhotoFromSharedBotAPIVolume(t *testing.T) {
	requireMediaTool(t, "convert")
	bot, client := setupDistortScenario(t)
	Config.Token = "1:unitsecret"
	Config.BotApiDataDir = t.TempDir()
	dir := filepath.Join(Config.BotApiDataDir, Config.Token, "photos")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	photo, err := os.ReadFile(ffmpegMedia(t, "photo"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "photo.jpg"), photo, 0o600); err != nil {
		t.Fatal(err)
	}
	client.setResult("getFile", json.RawMessage(`{"file_path":"/api/container/1/photos/photo.jpg"}`))
	reply := &gotgbot.Message{Photo: []gotgbot.PhotoSize{{FileId: "shared"}}}
	if err := Distort(bot, testReplyContext(bot, "/distort", testUser(42), reply)); err != nil {
		t.Fatal(err)
	}
	if len(client.callsFor("sendDocument")) != 1 || DistortCache["shared"] != "distorted" {
		t.Fatal("photo from shared Bot API volume was not distorted")
	}
	assertDistortCleanup(t, "shared")
}

func TestDistortUploadFailureAndMissingResult(t *testing.T) {
	for _, kind := range []string{"failure", "missing audio"} {
		t.Run(kind, func(t *testing.T) {
			bot, client := setupDistortScenario(t)
			fakeMediaFile(t, client, ffmpegMedia(t, "audio"))
			if kind == "failure" {
				client.setError("sendAudio", errors.New("upload failed"))
			} else {
				client.setResult("sendAudio", json.RawMessage(`{"message_id":1}`))
			}
			if err := Distort(bot, testReplyContext(bot, "/distort", testUser(42), &gotgbot.Message{Audio: &gotgbot.Audio{FileId: "audio"}})); err == nil {
				t.Fatal("missing upload result was accepted")
			}
			if _, ok := DistortCache["audio"]; ok {
				t.Fatal("failed upload poisoned the cache")
			}
			assertDistortCleanup(t, "audio")
		})
	}
}

func TestDistortWorkerFailureJoinsAndCleans(t *testing.T) {
	bot, client := setupDistortScenario(t)
	input := ffmpegMedia(t, "video_only")
	fakeMediaFile(t, client, input)
	// Fail only frame processing; ffmpeg and ffprobe remain real tools.
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "convert"), []byte("#!/bin/sh\nexit 7\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	err := Distort(bot, testReplyContext(bot, "/distort", testUser(42), &gotgbot.Message{Video: &gotgbot.Video{FileId: "failedframes"}}))
	if err == nil || !strings.Contains(err.Error(), "exit status 7") {
		t.Fatalf("frame worker failure was lost: %v", err)
	}
	if len(client.callsFor("sendDocument")) != 0 {
		t.Fatal("partial frame result was sent")
	}
	assertDistortCleanup(t, "failedframes")
}

func TestDistortCorruptMediaAndMissingVideoTrack(t *testing.T) {
	for _, kind := range []string{"corrupt", "audio labelled video"} {
		t.Run(kind, func(t *testing.T) {
			bot, client := setupDistortScenario(t)
			input := ffmpegMedia(t, "audio")
			if kind == "corrupt" {
				if err := os.WriteFile(input, []byte("not media"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			fakeMediaFile(t, client, input)
			err := Distort(bot, testReplyContext(bot, "/distort", testUser(42), &gotgbot.Message{Video: &gotgbot.Video{FileId: "broken"}}))
			if err == nil {
				t.Fatal("invalid media was accepted")
			}
			if kind != "corrupt" && !strings.Contains(err.Error(), "видео-дорожка") {
				t.Fatalf("unexpected missing-track error: %v", err)
			}
			if len(client.callsFor("sendDocument")) != 0 {
				t.Fatal("sent invalid media")
			}
			assertDistortCleanup(t, "broken")
		})
	}
}

func TestDistortFrameLimit(t *testing.T) {
	bot, client := setupDistortScenario(t)
	requireMediaTool(t, "ffmpeg")
	requireMediaTool(t, "ffprobe")
	client.setMemberStatus(42, "member")
	input := filepath.Join(t.TempDir(), "manyframes.mp4")
	if output, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "fatal", "-f", "lavfi", "-i", "testsrc=duration=1:size=16x16:rate=1001", "-an", "-y", input).CombinedOutput(); err != nil {
		t.Fatalf("frame-limit fixture: %v\n%s", err, output)
	}
	fakeMediaFile(t, client, input)
	if err := Distort(bot, testReplyContext(bot, "/distort", testUser(42), &gotgbot.Message{Video: &gotgbot.Video{FileId: "long"}})); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(client.sentTexts(), ""), "Максимум 1000 фреймов") || len(client.callsFor("sendDocument")) != 0 {
		t.Fatal("frame limit was not enforced")
	}
	assertDistortCleanup(t, "long")
}
