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
	fs, err := loadQuoteFonts()
	if err != nil {
		return nil, err
	}
	dc := gg.NewContext(1200, 1200)
	bg := backgrounds[index]
	bounds := bg.Bounds()
	dc.Push()
	dc.Scale(1200/float64(bounds.Dx()), 1200/float64(bounds.Dy()))
	dc.DrawImage(bg, -bounds.Min.X, -bounds.Min.Y)
	dc.Pop()
	// White rim and large circular avatar, with initials when Telegram hides it.
	const cx, cy, radius = 600., 480., 305.
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
	dc.SetRGBA(1, 1, 1, .93)
	dc.DrawRoundedRectangle(100, 840, 1000, 265, 35)
	dc.Fill()
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		name = "Друг"
	}
	if utf8.RuneCountInString(name) > 100 {
		name = string([]rune(name)[:99]) + "…"
	}
	caption := name + ", с днем рождения!"
	for size := 58.; size >= 18; size -= 2 {
		face, err := fs.newFace(size, true)
		if err != nil {
			return nil, err
		}
		dc.SetFontFace(face)
		lines := quoteWrap(dc, caption, 920)
		if float64(len(lines))*size*1.2 <= 215 {
			dc.SetRGB(.13, .18, .26)
			top := 972.5 - float64(len(lines)-1)*size*1.2/2
			for i, line := range lines {
				dc.DrawStringAnchored(line, 600, top+float64(i)*size*1.2, .5, .5)
			}
			face.Close()
			break
		}
		face.Close()
	}
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
	buf := new(bytes.Buffer)
	if err := dc.EncodePNG(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
