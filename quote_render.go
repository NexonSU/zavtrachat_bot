package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"math"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed assets/NotoEmoji.ttf
var quoteEmojiFont []byte

type quoteFontSet struct{ regular, bold, emoji *opentype.Font }

var loadQuoteFonts = sync.OnceValues(func() (quoteFontSet, error) {
	var fs quoteFontSet
	var err error
	fs.regular, err = opentype.Parse(goregular.TTF)
	if err != nil {
		return fs, err
	}
	fs.bold, err = opentype.Parse(gobold.TTF)
	if err != nil {
		return fs, err
	}
	fs.emoji, err = opentype.Parse(quoteEmojiFont)
	return fs, err
})

type quoteFace struct{ primary, emoji font.Face }

func quoteInvisible(r rune) bool { return r == '\u200d' || r == '\ufe0f' || r == '\ufe0e' }
func (f *quoteFace) face(r rune) font.Face {
	if _, ok := f.primary.GlyphAdvance(r); ok {
		return f.primary
	}
	return f.emoji
}
func (f *quoteFace) Close() error {
	a := f.primary.Close()
	b := f.emoji.Close()
	if a != nil {
		return a
	}
	return b
}
func (f *quoteFace) Metrics() font.Metrics { return f.primary.Metrics() }
func (f *quoteFace) Kern(a, b rune) fixed.Int26_6 {
	if quoteInvisible(a) || quoteInvisible(b) {
		return 0
	}
	p := f.face(a)
	if p == f.face(b) {
		return p.Kern(a, b)
	}
	return 0
}
func (f *quoteFace) GlyphAdvance(r rune) (fixed.Int26_6, bool) {
	if quoteInvisible(r) {
		return 0, true
	}
	return f.face(r).GlyphAdvance(r)
}
func (f *quoteFace) GlyphBounds(r rune) (fixed.Rectangle26_6, fixed.Int26_6, bool) {
	if quoteInvisible(r) {
		return fixed.Rectangle26_6{}, 0, true
	}
	return f.face(r).GlyphBounds(r)
}
func (f *quoteFace) Glyph(dot fixed.Point26_6, r rune) (image.Rectangle, image.Image, image.Point, fixed.Int26_6, bool) {
	if quoteInvisible(r) {
		return image.Rectangle{}, image.NewUniform(color.Transparent), image.Point{}, 0, true
	}
	return f.face(r).Glyph(dot, r)
}
func (fs quoteFontSet) newFace(size float64, bold bool) (font.Face, error) {
	p := fs.regular
	if bold {
		p = fs.bold
	}
	a, err := opentype.NewFace(p, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, err
	}
	b, err := opentype.NewFace(fs.emoji, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		a.Close()
		return nil, err
	}
	return &quoteFace{a, b}, nil
}

func quoteDate(timestamp int64) string {
	if timestamp <= 0 {
		return "Дата неизвестна"
	}
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		return time.Unix(timestamp, 0).UTC().Format("02.01.2006 · 15:04 UTC")
	}
	return time.Unix(timestamp, 0).In(location).Format("02.01.2006 · 15:04 МСК")
}

// Wrap each paragraph and split oversized words. No quote text is truncated.
func quoteWrap(dc *gg.Context, text string, width float64) []string {
	var lines []string
	for _, paragraph := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := ""
		for _, word := range strings.Fields(paragraph) {
			candidate := word
			if line != "" {
				candidate = line + " " + word
			}
			w, _ := dc.MeasureString(candidate)
			if w <= width {
				line = candidate
				continue
			}
			if line != "" {
				lines = append(lines, line)
				line = ""
			}
			for _, r := range word {
				candidate = line + string(r)
				w, _ = dc.MeasureString(candidate)
				if w > width && line != "" && !unicode.Is(unicode.Mn, r) && !quoteInvisible(r) {
					lines = append(lines, line)
					line = ""
				}
				line += string(r)
			}
		}
		lines = append(lines, line)
	}
	return lines
}

func quoteInitials(name string) string {
	var letters []rune
	for _, word := range strings.Fields(name) {
		for _, r := range word {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				letters = append(letters, unicode.ToUpper(r))
				break
			}
		}
		if len(letters) == 2 {
			break
		}
	}
	if len(letters) == 0 {
		return "?"
	}
	return string(letters)
}

func quoteRGB(v uint32) color.Color {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}
}

func renderQuote(q quoteContent, avatar image.Image) ([]byte, error) {
	const width = 1200
	const padding = 96.
	fs, err := loadQuoteFonts()
	if err != nil {
		return nil, fmt.Errorf("ошибка шрифта")
	}
	size := 60.
	switch n := utf8.RuneCountInString(q.Text); {
	case n > 1000:
		size = 36
	case n > 400:
		size = 42
	case n > 180:
		size = 50
	}
	textFace, err := fs.newFace(size, false)
	if err != nil {
		return nil, err
	}
	defer textFace.Close()
	nameFace, err := fs.newFace(30, true)
	if err != nil {
		return nil, err
	}
	defer nameFace.Close()
	dateFace, err := fs.newFace(24, false)
	if err != nil {
		return nil, err
	}
	defer dateFace.Close()
	dc := gg.NewContext(1, 1)
	dc.SetFontFace(textFace)
	lines := quoteWrap(dc, q.Text, width-2*padding)
	lineHeight := math.Ceil(size * 1.36)
	dc.SetFontFace(nameFace)
	names := quoteWrap(dc, q.Author, width-2*padding-136)
	footerHeight := math.Max(112, float64(len(names))*40+44)
	height := int(math.Ceil(math.Max(660, 160+float64(len(lines))*lineHeight+76+footerHeight+padding)))
	if height > 6000 {
		return nil, fmt.Errorf("слишком много строк для одной карточки — выбери сообщение покороче")
	}
	dc = gg.NewContext(width, height)
	hash := fnv.New32a()
	hash.Write([]byte(q.Author))
	palettes := [][2]uint32{{0x201D4D, 0x703856}, {0x142F49, 0x235B68}, {0x31264E, 0x4E4D91}, {0x382442, 0x854950}}
	palette := palettes[int(hash.Sum32())%len(palettes)]
	gradient := gg.NewLinearGradient(0, 0, width, float64(height))
	gradient.AddColorStop(0, quoteRGB(palette[0]))
	gradient.AddColorStop(1, quoteRGB(palette[1]))
	dc.SetFillStyle(gradient)
	dc.DrawRectangle(0, 0, width, float64(height))
	dc.Fill()
	// A soft corner glow adds depth while keeping the text area quiet.
	glow := gg.NewRadialGradient(width-40, 30, 0, width-40, 30, 950)
	glow.AddColorStop(0, color.NRGBA{R: 143, G: 151, B: 255, A: 48})
	glow.AddColorStop(1, color.NRGBA{R: 143, G: 151, B: 255, A: 0})
	dc.SetFillStyle(glow)
	dc.DrawRectangle(0, 0, width, float64(height))
	dc.Fill()
	dc.SetFontFace(textFace)
	dc.SetRGBA(1, 1, 1, .20)
	dc.DrawString("“", padding-4, 112)
	dc.SetRGB(1, 1, 1)
	y := 160 + textFace.Metrics().Ascent.Ceil()
	for _, line := range lines {
		dc.DrawString(line, padding, float64(y))
		y += int(math.Ceil(lineHeight))
	}
	footerY := float64(height) - padding - footerHeight
	dc.SetRGBA(1, 1, 1, .18)
	dc.SetLineWidth(1)
	dc.DrawLine(padding, footerY-30, width-padding, footerY-30)
	dc.Stroke()
	const diameter = 104.
	cx, cy := padding+diameter/2, footerY+diameter/2
	dc.Push()
	dc.DrawCircle(cx, cy, diameter/2)
	dc.Clip()
	if avatar != nil {
		bounds := avatar.Bounds()
		scale := diameter / math.Min(float64(bounds.Dx()), float64(bounds.Dy()))
		dc.Translate(cx, cy)
		dc.Scale(scale, scale)
		dc.DrawImage(avatar, -bounds.Min.X-bounds.Dx()/2, -bounds.Min.Y-bounds.Dy()/2)
	} else {
		dc.SetRGBA(1, 1, 1, .14)
		dc.DrawRectangle(padding, footerY, diameter, diameter)
		dc.Fill()
		dc.SetFontFace(nameFace)
		dc.SetRGB(1, 1, 1)
		dc.DrawStringAnchored(quoteInitials(q.Author), cx, cy, .5, .5)
	}
	dc.Pop()
	dc.ResetClip() // gg.Pop restores transforms but intentionally retains the clip mask.
	dc.SetFontFace(nameFace)
	dc.SetRGB(1, 1, 1)
	for i, line := range names {
		dc.DrawString(line, padding+136, footerY+34+float64(i)*40)
	}
	dc.SetFontFace(dateFace)
	dc.SetRGBA(1, 1, 1, .65)
	dc.DrawString(quoteDate(q.Date), padding+136, footerY+float64(len(names))*40+28)
	buf := new(bytes.Buffer)
	if err := dc.EncodePNG(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
