package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestQuoteWallpapersSameDimensionsAndContrast(t *testing.T) {
	backgrounds, err := loadQuoteWallpapers()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[[32]byte]bool{}
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
		if img.Bounds().Dx() != 1200 || img.Bounds().Dy() != 660 {
			t.Fatalf("card size differs: %s %v", bg.Name, img.Bounds())
		}
		// Visible author and date outside the avatar mask, on every light/dark theme.
		for _, band := range []struct {
			name        string
			top, bottom int
		}{{"author", 462, 490}, {"date", 496, 524}} {
			changed := 0
			for y := band.top; y < band.bottom; y++ {
				for x := 232; x < 850; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					br, green, bb, _ := bg.Image.At(x, y).RGBA()
					if absQuoteChannel(int(r>>8)-int(br>>8)) > 30 || absQuoteChannel(int(g>>8)-int(green>>8)) > 30 || absQuoteChannel(int(b>>8)-int(bb>>8)) > 30 {
						changed++
					}
				}
			}
			if changed < 30 {
				t.Fatalf("%s invisible on %s", band.name, bg.Name)
			}
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
