package main

import (
	"bytes"
	"context"
	"image"
	_ "image/jpeg"
	"io"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

const quoteAvatarMaxBytes = 4 << 20

func quoteAvatar(ctx context.Context, bot *gotgbot.Bot, q quoteContent) image.Image {
	img, _ := quoteAvatarWithStatus(ctx, bot, q)
	return img
}

// Return a fixed diagnostic code only: never expose URLs, IDs, paths or errors.
func quoteAvatarWithStatus(ctx context.Context, bot *gotgbot.Bot, q quoteContent) (image.Image, string) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if q.UserID == 0 && q.ChatID == 0 {
		return nil, "author_unknown"
	}
	if ctx.Err() != nil {
		return nil, "timeout"
	}
	opts := &gotgbot.RequestOpts{Timeout: 3 * time.Second}
	status := "photo_unavailable"
	lookupSucceeded := false
	tried := make(map[string]bool)
	try := func(ids []string) image.Image {
		for _, id := range ids {
			if id == "" || tried[id] || len(tried) >= 8 {
				continue
			}
			if ctx.Err() != nil {
				status = "timeout"
				return nil
			}
			tried[id] = true
			img, reason := quoteAvatarFile(ctx, bot, id, opts)
			if img != nil {
				return img
			}
			status = reason
		}
		return nil
	}
	if q.UserID != 0 {
		photos, err := bot.GetUserProfilePhotosWithContext(ctx, q.UserID, &gotgbot.GetUserProfilePhotosOpts{Limit: 1, RequestOpts: opts})
		if err != nil {
			status = "profile_request_failed"
		} else if photos != nil {
			lookupSucceeded = true
		}
		if photos != nil && len(photos.Photos) > 0 {
			sizes := append([]gotgbot.PhotoSize(nil), photos.Photos[0]...)
			// Prefer a full image; small cached thumbnails are not always downloadable.
			sort.SliceStable(sizes, func(i, j int) bool { return sizes[i].Width*sizes[i].Height > sizes[j].Width*sizes[j].Height })
			var ids []string
			for _, size := range sizes {
				ids = append(ids, size.FileId)
			}
			if img := try(ids); img != nil {
				return img, ""
			}
		}
	}
	// getChat can expose the current public photo when profile-photo history is
	// empty/unavailable. Never look up hidden forward authors by their display name.
	if ctx.Err() != nil {
		return nil, "timeout"
	}
	chatID := q.ChatID
	if chatID == 0 {
		chatID = q.UserID
	}
	chat, err := bot.GetChatWithContext(ctx, chatID, &gotgbot.GetChatOpts{RequestOpts: opts})
	if err == nil && chat != nil {
		lookupSucceeded = true
	}
	if len(tried) == 0 {
		if lookupSucceeded {
			status = "photo_unavailable"
		} else {
			status = "profile_request_failed"
		}
	}
	if err == nil && chat != nil && chat.Photo != nil {
		if img := try([]string{chat.Photo.BigFileId, chat.Photo.SmallFileId}); img != nil {
			return img, ""
		}
	}
	if ctx.Err() != nil {
		return nil, "timeout"
	}
	return nil, status
}

func quoteAvatarFile(ctx context.Context, bot *gotgbot.Bot, id string, opts *gotgbot.RequestOpts) (image.Image, string) {
	f, err := bot.GetFileWithContext(ctx, id, &gotgbot.GetFileOpts{RequestOpts: opts})
	if err != nil || f == nil || f.FilePath == "" {
		return nil, "file_request_failed"
	}
	if f.FileSize > quoteAvatarMaxBytes {
		return nil, "image_too_large"
	}
	reason := "download_failed"
	for _, path := range mediaLocalPaths(f.FilePath, bot.Id) {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		img, status := decodeQuoteAvatarStatus(file)
		file.Close()
		if img != nil {
			return img, ""
		}
		reason = status
	}
	// Files belong to the same Bot API endpoint as getFile. Reuse its transport
	// (proxy, TLS, local routing), rather than the external-content proxy client.
	var client *http.Client
	if base, ok := bot.BotClient.(*gotgbot.BaseBotClient); ok {
		client = &base.Client
	} else {
		client = httpClient()
		defer client.CloseIdleConnections()
	}
	urls := []string{f.URL(bot, nil)}
	// Local-mode getFile may return an API-container absolute path. Its file
	// endpoint can require the bot-relative path rather than that container path.
	if relative := mediaRelativePath(f.FilePath, bot.Id); relative != "" && relative != f.FilePath {
		relativeFile := *f
		relativeFile.FilePath = relative
		if relativeURL := relativeFile.URL(bot, nil); relativeURL != urls[0] {
			urls = append(urls, relativeURL)
		}
	}
	for _, address := range urls {
		if ctx.Err() != nil {
			return nil, "timeout"
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}
		if resp.ContentLength > quoteAvatarMaxBytes {
			resp.Body.Close()
			reason = "image_too_large"
			continue
		}
		img, status := decodeQuoteAvatarStatus(resp.Body)
		resp.Body.Close()
		if img != nil {
			return img, ""
		}
		reason = status
	}
	return nil, reason
}

func decodeQuoteAvatar(r io.Reader) image.Image { img, _ := decodeQuoteAvatarStatus(r); return img }
func decodeQuoteAvatarStatus(r io.Reader) (image.Image, string) {
	data, err := io.ReadAll(io.LimitReader(r, quoteAvatarMaxBytes+1))
	if err != nil {
		return nil, "download_failed"
	}
	if len(data) > quoteAvatarMaxBytes {
		return nil, "image_too_large"
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, "invalid_image"
	}
	if cfg.Width > 2048 || cfg.Height > 2048 {
		return nil, "image_too_large"
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "invalid_image"
	}
	return img, ""
}

func quoteAvatarNotice(status string) string {
	switch status {
	case "":
		return ""
	case "author_unknown":
		return "Аватар: Telegram не передал данные для получения фото автора. Использованы инициалы."
	case "photo_unavailable":
		return "Аватар: Telegram не вернул фотографию автора. Использованы инициалы."
	case "profile_request_failed":
		return "Аватар: не удалось запросить фото автора у Telegram. Использованы инициалы."
	case "file_request_failed":
		return "Аватар: Telegram вернул фото, но не предоставил его файл. Использованы инициалы."
	case "download_failed":
		return "Аватар: не удалось скачать файл фотографии. Использованы инициалы."
	case "invalid_image":
		return "Аватар: файл фотографии не удалось прочитать как изображение. Использованы инициалы."
	case "image_too_large":
		return "Аватар: фотография превышает допустимый размер. Использованы инициалы."
	case "timeout":
		return "Аватар: истекло время загрузки фотографии. Использованы инициалы."
	default:
		return "Аватар недоступен. Использованы инициалы."
	}
}
