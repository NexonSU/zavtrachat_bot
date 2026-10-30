package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// FFmpeg must read a local file: it cannot inherit the Bot API client's TLS
// settings, proxy or authentication transport. Keep downloads inside the job
// deadline and remove them on both success and failure.
func mediaInputFile(ctx context.Context, bot *gotgbot.Bot, media Media) (string, func(), error) {
	if info, err := os.Stat(media.FilePath); err == nil && info.Mode().IsRegular() {
		return media.FilePath, func() {}, nil
	}
	workdir, err := os.MkdirTemp(os.TempDir(), "zavtra-media-input-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(workdir) }
	input := filepath.Join(workdir, "input")
	if err := downloadMediaInput(ctx, bot, media, input); err != nil {
		cleanup()
		return "", nil, err
	}
	return input, cleanup, nil
}

func downloadMediaInput(ctx context.Context, bot *gotgbot.Bot, media Media, destination string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	var client *http.Client
	if base, ok := bot.BotClient.(*gotgbot.BaseBotClient); ok {
		client = &base.Client
	} else {
		client = httpClient()
		defer client.CloseIdleConnections()
	}
	urls := []string{media.FileURL}
	if relative := mediaRelativePath(media.FilePath, bot.Id); relative != "" && relative != media.FilePath {
		file := gotgbot.File{FilePath: relative}
		if address := file.URL(bot, nil); address != urls[0] {
			urls = append(urls, address)
		}
	}
	var status int
	for _, address := range urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		status = resp.StatusCode
		if status != http.StatusOK {
			resp.Body.Close()
			continue
		}
		out, err := os.Create(destination)
		if err != nil {
			resp.Body.Close()
			return err
		}
		_, copyErr := io.Copy(out, resp.Body)
		resp.Body.Close()
		closeErr := out.Close()
		if ctx.Err() != nil {
			return fmt.Errorf("не удалось загрузить файл Telegram: %w", ctx.Err())
		}
		if copyErr != nil || closeErr != nil {
			return fmt.Errorf("не удалось сохранить файл Telegram")
		}
		return nil
	}
	// HTTP errors can contain the full URL and bot token. Report only safe
	// status/context information to the command's error handler.
	if ctx.Err() != nil {
		return fmt.Errorf("не удалось загрузить файл Telegram: %w", ctx.Err())
	}
	if status != 0 {
		return fmt.Errorf("не удалось загрузить файл Telegram: HTTP %d", status)
	}
	return fmt.Errorf("не удалось загрузить файл Telegram")
}
