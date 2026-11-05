package main

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/png"
	"math"
	"math/rand/v2"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
)

//go:embed assets/birthday-backgrounds/*.png
var birthdayBackgroundFiles embed.FS

var birthdayBackgroundNames = []string{"sky", "pink", "lilac", "mint", "yellow", "coral", "peach", "purple", "blue", "turquoise"}

var loadBirthdayBackgrounds = sync.OnceValues(func() ([]image.Image, error) {
	var images []image.Image
	for _, name := range birthdayBackgroundNames {
		f, err := birthdayBackgroundFiles.Open("assets/birthday-backgrounds/" + name + ".png")
		if err != nil {
			return nil, err
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, nil
})

func renderBirthday(name string, avatar image.Image, index int) ([]byte, error) {
	backgrounds, err := loadBirthdayBackgrounds()
	if err != nil {
		return nil, err
	}
	if index < 0 || index >= len(backgrounds) {
		return nil, fmt.Errorf("invalid birthday background")
	}
	return renderPortraitCard(name, ",", "с днем рождения!", avatar, backgrounds[index], 450, 860, drawBirthdaySparkles)
}

func renderPortraitCard(name, suffix, caption string, avatar, bg image.Image, avatarY, plateY float64, sparkles func(*gg.Context)) ([]byte, error) {
	fs, err := loadQuoteFonts()
	if err != nil {
		return nil, err
	}
	dc := gg.NewContext(1200, 1200)
	bounds := bg.Bounds()
	dc.Push()
	dc.Scale(1200/float64(bounds.Dx()), 1200/float64(bounds.Dy()))
	dc.DrawImage(bg, -bounds.Min.X, -bounds.Min.Y)
	dc.Pop()
	// White rim and large circular avatar, with initials when Telegram hides it.
	const cx, radius = 600., 360.
	cy := avatarY
	dc.SetRGBA(1, 1, 1, .94)
	dc.DrawCircle(cx, cy, radius+9)
	dc.Fill()
	dc.Push()
	dc.DrawCircle(cx, cy, radius)
	dc.Clip()
	if avatar != nil {
		b := avatar.Bounds()
		scale := radius * 2 / math.Min(float64(b.Dx()), float64(b.Dy()))
		dc.Translate(cx, cy)
		dc.Scale(scale, scale)
		dc.DrawImage(avatar, -b.Min.X-b.Dx()/2, -b.Min.Y-b.Dy()/2)
	} else {
		dc.SetRGB(.23, .42, .62)
		dc.DrawRectangle(0, 0, 1200, 1200)
		dc.Fill()
		face, err := fs.newFace(170, true)
		if err != nil {
			return nil, err
		}
		dc.SetFontFace(face)
		dc.SetRGB(1, 1, 1)
		dc.DrawStringAnchored(quoteInitials(name), cx, cy, .5, .5)
		face.Close()
	}
	dc.Pop()
	dc.ResetClip()
	// A neutral plate keeps the caption legible on all ten colorways.
	dc.SetRGBA(1, 1, 1, .67)
	dc.DrawRoundedRectangle(100, plateY, 1000, 260, 35)
	dc.Fill()
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		name = "Друг"
	}
	if utf8.RuneCountInString(name) > 100 {
		name = string([]rune(name)[:99]) + "…"
	}
	// Keep the name on its own line and center the visible glyph bounds,
	// rather than the font's baseline/nominal metrics (which skewed the old plate).
	for size := 80.; size >= 18; size -= 2 {
		face, err := fs.newFace(size, true)
		if err != nil {
			return nil, err
		}
		dc.SetFontFace(face)
		lines := append(quoteWrap(dc, name+suffix, 920), quoteWrap(dc, caption, 920)...)
		gap := size * .24
		height := gap * float64(len(lines)-1)
		for _, line := range lines {
			bounds, _ := font.BoundString(face, line)
			height += float64(bounds.Max.Y-bounds.Min.Y) / 64
		}
		if height <= 216 {
			dc.SetRGB(.13, .18, .26)
			top := plateY + 130 - height/2
			for _, line := range lines {
				bounds, _ := font.BoundString(face, line)
				x := 600. - float64(bounds.Min.X+bounds.Max.X)/128
				baseline := top - float64(bounds.Min.Y)/64
				dc.DrawString(line, x, baseline)
				top += float64(bounds.Max.Y-bounds.Min.Y)/64 + gap
			}
			face.Close()
			break
		}
		face.Close()
	}
	sparkles(dc)
	buf := new(bytes.Buffer)
	if err := dc.EncodePNG(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawBirthdaySparkles(dc *gg.Context) {
	const cx, cy, radius = 600., 450., 360.
	// Fresh foreground sparkles, concentrated above and alongside the portrait.
	for i := 0; i < 38; i++ {
		x, y := float64(70+rand.IntN(1060)), float64(35+rand.IntN(760))
		if math.Hypot(x-cx, y-cy) < radius+18 {
			continue
		}
		s := float64(5 + rand.IntN(10))
		dc.SetRGBA(1, .9, .42, .95)
		dc.MoveTo(x, y-s)
		dc.LineTo(x+s*.28, y-s*.28)
		dc.LineTo(x+s, y)
		dc.LineTo(x+s*.28, y+s*.28)
		dc.LineTo(x, y+s)
		dc.LineTo(x-s*.28, y+s*.28)
		dc.LineTo(x-s, y)
		dc.LineTo(x-s*.28, y-s*.28)
		dc.ClosePath()
		dc.Fill()
	}
}
