package main

import (
	"bytes"
	"context"
	"image"
	_ "image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

const quoteAvatarMaxBytes = 4 << 20

// Avatar errors are non-fatal: use initials, without exposing URLs or tokens.
func quoteAvatar(ctx context.Context, bot *gotgbot.Bot, q quoteContent) image.Image {
	var id string
	opts := &gotgbot.RequestOpts{Timeout: 4 * time.Second}
	if q.UserID != 0 {
		photos, err := bot.GetUserProfilePhotosWithContext(ctx, q.UserID, &gotgbot.GetUserProfilePhotosOpts{Limit: 1, RequestOpts: opts})
		if err != nil || photos == nil || len(photos.Photos) == 0 {
			return nil
		}
		for _, p := range photos.Photos[0] {
			if p.FileId != "" {
				id = p.FileId
				break
			}
		}
	} else if q.ChatID != 0 {
		chat, err := bot.GetChatWithContext(ctx, q.ChatID, &gotgbot.GetChatOpts{RequestOpts: opts})
		if err != nil || chat == nil || chat.Photo == nil {
			return nil
		}
		id = chat.Photo.SmallFileId
	}
	if id == "" {
		return nil
	}
	f, err := bot.GetFileWithContext(ctx, id, &gotgbot.GetFileOpts{RequestOpts: opts})
	if err != nil || f == nil || f.FilePath == "" || f.FileSize > quoteAvatarMaxBytes {
		return nil
	}
	// Local Bot API servers can expose a file directly or via their shared data directory.
	paths := []string{f.FilePath}
	if Config != nil && Config.BotApiDataDir != "" && !filepath.IsAbs(f.FilePath) {
		paths = append(paths, filepath.Join(Config.BotApiDataDir, Config.Token, f.FilePath))
	}
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		img := decodeQuoteAvatar(file)
		file.Close()
		if img != nil {
			return img
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL(bot, nil), nil)
	if err != nil {
		return nil
	}
	client := httpClient()
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.ContentLength > quoteAvatarMaxBytes {
		return nil
	}
	return decodeQuoteAvatar(resp.Body)
}

func decodeQuoteAvatar(r io.Reader) image.Image {
	data, err := io.ReadAll(io.LimitReader(r, quoteAvatarMaxBytes+1))
	if err != nil || len(data) > quoteAvatarMaxBytes {
		return nil
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 2048 || cfg.Height > 2048 {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return img
}
