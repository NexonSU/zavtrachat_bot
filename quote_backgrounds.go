package main

import (
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand/v2"
	"sync"

	"github.com/fogleman/gg"
)

const quoteWallpaperWidth = 1200
const quoteWallpaperHeight = 660
const quoteWallpaperCount = 4

//go:embed assets/quote-backgrounds/*.png
var quoteWallpaperFiles embed.FS

type quoteWallpaper struct {
	Name       string
	Foreground color.NRGBA
	Image      image.Image
}

var loadQuoteWallpapers = sync.OnceValues(func() ([quoteWallpaperCount]quoteWallpaper, error) {
	specs := [quoteWallpaperCount]struct {
		name, file string
		foreground color.NRGBA
	}{
		{"light", "light.png", color.NRGBA{R: 24, G: 52, B: 56, A: 255}},
		{"night", "night.png", color.NRGBA{R: 255, G: 255, B: 255, A: 255}},
		{"lilac", "lilac.png", color.NRGBA{R: 52, G: 37, B: 78, A: 255}},
		{"mint", "mint.png", color.NRGBA{R: 18, G: 62, B: 62, A: 255}},
	}
	var backgrounds [quoteWallpaperCount]quoteWallpaper
	for i, spec := range specs {
		f, err := quoteWallpaperFiles.Open("assets/quote-backgrounds/" + spec.file)
		if err != nil {
			return backgrounds, err
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			return backgrounds, err
		}
		if img.Bounds().Dx() != quoteWallpaperWidth || img.Bounds().Dy() != quoteWallpaperHeight {
			return backgrounds, fmt.Errorf("размер фона цитаты не соответствует 1200×660")
		}
		backgrounds[i] = quoteWallpaper{spec.name, spec.foreground, img}
	}
	return backgrounds, nil
})

// Uniform selection per new card, independent of author; repeats are allowed.
func randomQuoteWallpaper() int { return rand.IntN(quoteWallpaperCount) }

func drawQuoteWallpaper(dc *gg.Context, wallpaper quoteWallpaper, width, height int) {
	bounds := wallpaper.Image.Bounds()
	scale := math.Max(float64(width)/float64(bounds.Dx()), float64(height)/float64(bounds.Dy()))
	dc.Push()
	dc.Translate((float64(width)-float64(bounds.Dx())*scale)/2, (float64(height)-float64(bounds.Dy())*scale)/2)
	dc.Scale(scale, scale)
	dc.DrawImage(wallpaper.Image, -bounds.Min.X, -bounds.Min.Y)
	dc.Pop()
}

func setQuoteInk(dc *gg.Context, wallpaper quoteWallpaper, opacity float64) {
	c := wallpaper.Foreground
	dc.SetRGBA(float64(c.R)/255, float64(c.G)/255, float64(c.B)/255, opacity)
}
