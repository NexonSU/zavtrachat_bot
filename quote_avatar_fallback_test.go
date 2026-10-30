package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func quoteAvatarPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, quoteTestAvatar()); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestQuoteAvatarFallsBackToCurrentChatPhoto(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprint(failed), func(t *testing.T) {
			bot, client := setupTestEnv(t)
			client.setResult("getUserProfilePhotos", json.RawMessage(`{"total_count":0,"photos":[]}`))
			if failed {
				client.setError("getUserProfilePhotos", errors.New("profile request failed"))
			}
			client.setResult("getChat", json.RawMessage(`{"id":77,"type":"private","photo":{"small_file_id":"small","big_file_id":"big"}}`))
			path := filepath.Join(t.TempDir(), "avatar.png")
			if err := os.WriteFile(path, quoteAvatarPNG(t), 0600); err != nil {
				t.Fatal(err)
			}
			fakeMediaFile(t, client, path)
			img, status := quoteAvatarWithStatus(context.Background(), bot, quoteContent{UserID: 77})
			if img == nil || status != "" {
				t.Fatalf("fallback failed: %s", status)
			}
			calls := client.callsFor("getChat")
			if len(calls) != 1 || calls[0].Params["chat_id"] != int64(77) {
				t.Fatal("fallback queried a different author")
			}
		})
	}
}

func TestQuoteAvatarTriesOtherPhotoSizes(t *testing.T) {
	bot, client := setupTestEnv(t)
	client.setResult("getUserProfilePhotos", json.RawMessage(`{"total_count":1,"photos":[[{"file_id":"small","width":160,"height":160},{"file_id":"big","width":640,"height":640},{"file_id":"big","width":640,"height":640}]]}`))
	path := filepath.Join(t.TempDir(), "avatar.png")
	os.WriteFile(path, quoteAvatarPNG(t), 0600)
	fakeMediaFile(t, client, path)
	client.Observe = func(method string, p map[string]any) {
		if method == "getFile" {
			if p["file_id"] == "big" {
				client.Errors = map[string]error{"getFile": errors.New("unavailable image size")}
			} else {
				delete(client.Errors, "getFile")
			}
		}
	}
	img, status := quoteAvatarWithStatus(context.Background(), bot, quoteContent{UserID: 77})
	if img == nil || status != "" {
		t.Fatal(status)
	}
	calls := client.callsFor("getFile")
	if len(calls) != 2 || calls[0].Params["file_id"] != "big" || calls[1].Params["file_id"] != "small" {
		t.Fatalf("sizes not tried/deduplicated: %+v", calls)
	}
}

func TestQuoteAvatarMapsLocalBotAPIVolume(t *testing.T) {
	bot, client := setupTestEnv(t)
	Config.Token = "123:unitsecret"
	Config.BotApiDataDir = t.TempDir()
	dir := filepath.Join(Config.BotApiDataDir, Config.Token, "photos")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "avatar.jpg"), quoteAvatarPNG(t), 0600); err != nil {
		t.Fatal(err)
	}
	client.setResult("getUserProfilePhotos", json.RawMessage(`{"total_count":1,"photos":[[{"file_id":"photo","width":160,"height":160}]]}`))
	apiPath := "/api/container/123:unitsecret/photos/avatar.jpg"
	client.setResult("getFile", json.RawMessage(fmt.Sprintf(`{"file_path":%q}`, apiPath)))
	if img, status := quoteAvatarWithStatus(context.Background(), bot, quoteContent{UserID: 77}); img == nil || status != "" {
		t.Fatalf("shared volume mapping failed: %s", status)
	}
	if paths := quoteAvatarLocalPaths("../../private", 123); len(paths) != 0 {
		t.Fatal("relative traversal accepted")
	}
	other := "/api/container/456:other/photos/avatar.jpg"
	if paths := quoteAvatarLocalPaths(other, 123); len(paths) != 1 || paths[0] != other {
		t.Fatalf("mapped a different bot directory: %+v", paths)
	}
}

type quoteAvatarRoundTrip func(*http.Request) (*http.Response, error)

func (f quoteAvatarRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestQuoteAvatarUsesSameTransportAsBotAPI(t *testing.T) {
	for _, absolute := range []bool{false, true} {
		t.Run(fmt.Sprint(absolute), func(t *testing.T) {
			setupTestEnv(t)
			data := quoteAvatarPNG(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Quote-Transport") != "configured" {
					http.Error(w, "missing API transport", http.StatusForbidden)
					return
				}
				switch r.URL.Path {
				case "/botTESTTOKEN/getUserProfilePhotos":
					fmt.Fprint(w, `{"ok":true,"result":{"total_count":1,"photos":[[{"file_id":"avatar","width":160,"height":160}]]}}`)
				case "/botTESTTOKEN/getFile":
					path := "quote-avatar-transport/avatar.png"
					if absolute {
						path = "/api/container/1/" + path
					}
					fmt.Fprintf(w, `{"ok":true,"result":{"file_path":%q}}`, path)
				case "/file/botTESTTOKEN/quote-avatar-transport/avatar.png":
					w.Header().Set("Content-Type", "image/png")
					w.Write(data)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			transport := &http.Transport{Proxy: nil}
			defer transport.CloseIdleConnections()
			base := &gotgbot.BaseBotClient{Client: http.Client{Transport: quoteAvatarRoundTrip(func(r *http.Request) (*http.Response, error) {
				r.Header.Set("X-Quote-Transport", "configured")
				return transport.RoundTrip(r)
			})}, DefaultRequestOpts: &gotgbot.RequestOpts{APIURL: server.URL}}
			bot := &gotgbot.Bot{User: gotgbot.User{Id: 1}, Token: "TESTTOKEN", BotClient: base}
			old := HTTPClientProxy
			HTTPClientProxy = func(*http.Request) (*url.URL, error) { return url.Parse("http://127.0.0.1:1") }
			t.Cleanup(func() { HTTPClientProxy = old })
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			img, status := quoteAvatarWithStatus(ctx, bot, quoteContent{UserID: 77})
			if img == nil || status != "" {
				t.Fatalf("API worked but file used unrelated transport/proxy: %s", status)
			}
		})
	}
}

func TestQuoteAvatarPrivateStatusIsSafe(t *testing.T) {
	bot, client := setupTestEnv(t)
	img, status := quoteAvatarWithStatus(context.Background(), bot, quoteContent{Author: "Hidden"})
	if img != nil || status != "author_unknown" || len(client.callsFor("getChat")) != 0 || len(client.callsFor("getUserProfilePhotos")) != 0 {
		t.Fatal("hidden author was guessed")
	}
	for _, code := range []string{"author_unknown", "photo_unavailable", "profile_request_failed", "file_request_failed", "download_failed", "invalid_image", "image_too_large", "timeout", "https://token-secret.invalid"} {
		notice := quoteAvatarNotice(code)
		if notice == "" || strings.Contains(notice, "token-secret") || strings.Contains(notice, "https://") {
			t.Fatal("unsafe avatar notice")
		}
	}
	ctx := testReplyContext(bot, "/quote", testUser(100), testMessage("Quote", testUser(77)))
	if err := Quote(bot, ctx); err != nil {
		t.Fatal(err)
	}
	calls := client.callsFor("sendPhoto")
	if len(calls) != 1 {
		t.Fatal("quote missing")
	}
	eph, _ := calls[0].Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
	caption, _ := calls[0].Params["caption"].(string)
	if eph != nil || caption != "" {
		t.Fatal("missing or non-private avatar failure hint")
	}
}
