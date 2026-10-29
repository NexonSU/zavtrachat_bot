package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/fogleman/gg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
)

func TestQuoteWallpapersSameDimensionsAndContrast(t *testing.T) {
	backgrounds, err := loadQuoteWallpapers()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[[32]byte]bool{}
	firstHeight := 0
	q := quoteContent{Text: "Сначала сделаем красиво.\nА потом ещё красивее. 😎", Author: "Автор сообщения", Date: 1793273820}
	for i, bg := range backgrounds {
		if got := bg.Image.Bounds().Size(); got.X != 1200 || got.Y != 660 {
			t.Fatalf("%s: %v", bg.Name, got)
		}
		data, err := renderQuoteWithWallpaper(q, quoteTestAvatar(), i)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		if seen[digest] {
			t.Fatal("themes rendered identically")
		}
		seen[digest] = true
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if firstHeight == 0 {
			firstHeight = img.Bounds().Dy()
		}
		if img.Bounds().Dx() != 1200 || img.Bounds().Dy() < 660 || img.Bounds().Dy() != firstHeight {
			t.Fatalf("card size differs: %s %v", bg.Name, img.Bounds())
		}
		backgroundOnly := gg.NewContext(1200, img.Bounds().Dy())
		drawQuoteWallpaper(backgroundOnly, bg, 1200, img.Bounds().Dy())
		// Visible author and date outside the avatar mask, on every light/dark theme.
		footerY := img.Bounds().Dy() - int(quoteCardPadding+quoteAvatarDiameter)
		for _, band := range []struct {
			name        string
			top, bottom int
		}{{"author", footerY + 35, footerY + 78}, {"date", footerY + 90, footerY + 120}} {
			changed := 0
			for y := band.top; y < band.bottom; y++ {
				for x := int(quoteCardPadding + quoteAvatarDiameter + quoteAuthorGap); x < 1100; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					br, green, bb, _ := backgroundOnly.Image().At(x, y).RGBA()
					if absQuoteChannel(int(r>>8)-int(br>>8)) > 30 || absQuoteChannel(int(g>>8)-int(green>>8)) > 30 || absQuoteChannel(int(b>>8)-int(bb>>8)) > 30 {
						changed++
					}
				}
			}
			if changed < 30 {
				t.Fatalf("%s invisible on %s", band.name, bg.Name)
			}
		}
		// Thick underline: at least seven consecutive solid rows over 220px.
		maxRun, run := 0, 0
		for y := 168; y < footerY; y++ {
			solid := 0
			for x := int(quoteCardPadding) + 16; x < int(quoteCardPadding)+240; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if absQuoteChannel(int(r>>8)-int(bg.Foreground.R)) < 4 && absQuoteChannel(int(g>>8)-int(bg.Foreground.G)) < 4 && absQuoteChannel(int(b>>8)-int(bg.Foreground.B)) < 4 {
					solid++
				}
			}
			if solid >= 220 {
				run++
				if run > maxRun {
					maxRun = run
				}
			} else {
				run = 0
			}
		}
		if maxRun < 7 {
			t.Fatalf("bold underline absent on %s", bg.Name)
		}
		if dir := os.Getenv("ZAVTRA_QUOTE_PREVIEW_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("quote-%d-%s.png", i+1, bg.Name)), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Exercise Telegram's native Quote Reply through the real command handler.
	// Its output must equal one of the four renders of the selected fragment,
	// rather than the longer replied-to message or caller-supplied arguments.
	bot, client := setupTestEnv(t)
	avatarPath := filepath.Join(t.TempDir(), "avatar.png")
	if err := os.WriteFile(avatarPath, quoteAvatarPNG(t), 0600); err != nil {
		t.Fatal(err)
	}
	fakeMediaFile(t, client, avatarPath)
	client.setResult("getUserProfilePhotos", json.RawMessage(`{"total_count":1,"photos":[[{"file_id":"avatar","width":160,"height":160}]]}`))
	var sent []byte
	client.Observe = func(method string, params map[string]any) {
		if method == "sendPhoto" {
			if f, ok := params["photo"].(*gotgbot.FileReader); ok {
				sent, _ = io.ReadAll(f.Data)
			}
		}
	}
	prefix := "Полное сообщение 😎. "
	source := testMessage(prefix+q.Text+" И ещё заключение.", testUser(77))
	source.From.FirstName = "Автор"
	source.From.LastName = "сообщения"
	source.Date = q.Date
	ctx := testReplyContext(bot, "/quote лишние аргументы", testUser(100), source)
	ctx.Message.Quote = &gotgbot.TextQuote{Text: q.Text, IsManual: true, Position: int64(len(utf16.Encode([]rune(prefix))))}
	if err := Quote(bot, ctx); err != nil {
		t.Fatal(err)
	}
	if !seen[sha256.Sum256(sent)] {
		t.Fatal("actual card did not contain exactly the selected text and original attribution")
	}
	if dir := os.Getenv("ZAVTRA_QUOTE_PREVIEW_DIR"); dir != "" {
		if err := os.WriteFile(filepath.Join(dir, "quote-selected-fragment.png"), sent, 0644); err != nil {
			t.Fatal(err)
		}
		short := q
		short.Text = "Жизнь прекрасна."
		data, err := renderQuoteWithWallpaper(short, quoteTestAvatar(), 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "quote-short.png"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if backgrounds[1].Foreground.R != 255 {
		t.Fatal("night needs white text")
	}
	for _, i := range []int{0, 2, 3} {
		if backgrounds[i].Foreground.R > 100 {
			t.Fatal("light wallpaper needs dark text")
		}
	}
	for _, i := range []int{-1, 4} {
		if _, err := renderQuoteWithWallpaper(q, nil, i); err == nil {
			t.Fatal("invalid wallpaper accepted")
		}
	}
}

func absQuoteChannel(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
