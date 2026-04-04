package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	tgmd "github.com/eekstunt/telegramify-markdown-go"

	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino/flow/agent/react"
)

var AIBusy bool
var AISystem string
var AIAgent *react.Agent
var AIToolModel *ollama.ChatModel
var AIVisionModel *ollama.ChatModel

type AuthTransport struct {
	Header http.Header
	Base   http.RoundTripper
}

func (t *AuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.Base == nil {
		t.Base = http.DefaultTransport
	}

	// Clone the request to safely add headers without modifying the original
	// request object, which might be reused.
	req = req.Clone(req.Context())
	for k, v := range t.Header {
		req.Header[k] = v
	}
	return t.Base.RoundTrip(req)
}

func toEntities(ents []tgmd.Entity) []gotgbot.MessageEntity {
	out := make([]gotgbot.MessageEntity, len(ents))
	for i, e := range ents {
		out[i] = gotgbot.MessageEntity{
			Type:     string(e.Type),
			Offset:   int64(e.Offset),
			Length:   int64(e.Length),
			Url:      e.URL,
			Language: e.Language,
		}
	}
	return out
}

func CheckToolRestrictions(ctx context.Context, name string, arguments string) (string, error) {
	userId, ok := ctx.Value("tgUser").(int64)
	if !ok {
		return "", fmt.Errorf("ошибка определения пользователя")
	}
	if !IsAdminOrModer(userId) {
		if len(Config.AIAdminOnlyTools) == 0 {
			return "", fmt.Errorf("ошибка конфигурации, отсутствует список тулов в ai_admin_only_tools")
		}
		if slices.Contains(Config.AIAdminOnlyTools, name) {
			return "", fmt.Errorf("у этого пользователя нет прав")
		}
	}
	return arguments, nil
}

func toPtr[T any](in T) *T {
	return &in
}

func getImageForLLM(message *gotgbot.Message) (string, string, error) {
	media, err := GetMedia(message)
	if err != nil {
		return "", "", err
	}
	mimeType := mime.TypeByExtension(filepath.Ext(media.FilePath))
	queryFilePath := os.TempDir() + "/" + fmt.Sprint(time.Now().Unix()) + filepath.Base(media.FilePath)

	if _, err := os.Stat(media.FilePath); err == nil {
		queryFilePath = media.FilePath
	} else {
		err = DownloadFile(queryFilePath, fmt.Sprintf("https://api.telegram.org/file/bot%v/%v", Config.Token, media.FilePath))
		if err != nil {
			return "", "", err
		}
	}

	imgData, err := os.ReadFile(queryFilePath)
	if err != nil {
		return "", "", err
	}
	b64 := base64.StdEncoding.EncodeToString(imgData)

	os.Remove(queryFilePath)

	return b64, mimeType, nil
}
