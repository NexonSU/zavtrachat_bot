package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestMediaInputDownloadFailureCleanup(t *testing.T) {
	for _, kind := range []string{"unavailable", "truncated", "timeout", "network"} {
		t.Run(kind, func(t *testing.T) {
			setupTestEnv(t)
			missing := filepath.Join(t.TempDir(), "missing.jpg")
			t.Setenv("TMPDIR", t.TempDir())
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch kind {
				case "truncated":
					w.Header().Set("Content-Length", "100")
					w.Write([]byte("partial"))
				case "timeout":
					<-r.Context().Done()
				default:
					http.Error(w, "unavailable", http.StatusNotFound)
				}
			}))
			defer server.Close()
			client := *server.Client()
			if kind == "network" {
				client.Transport = quoteAvatarRoundTrip(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("private connection information")
				})
			}
			bot := &gotgbot.Bot{BotClient: &gotgbot.BaseBotClient{Client: client}}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			path, cleanup, err := mediaInputFile(ctx, bot, Media{
				FilePath: missing, FileURL: server.URL + "/botTOKEN_SECRET/photo.jpg",
			})
			if err == nil || path != "" || cleanup != nil {
				t.Fatalf("failed download accepted: %q, %v", path, err)
			}
			if strings.Contains(err.Error(), "TOKEN_SECRET") || strings.Contains(err.Error(), server.URL) || strings.Contains(err.Error(), "private connection") {
				t.Fatalf("download failure leaked connection details: %v", err)
			}
			if kind == "timeout" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("download lost parent deadline: %v", err)
			}
			entries, readErr := os.ReadDir(os.TempDir())
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("failed download left temporary input behind: %v, %v", entries, readErr)
			}
		})
	}
}
