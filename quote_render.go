package main

import (
	"bytes"
	_ "embed"
	"fmt"
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

const quoteCardPadding = 88.
const quoteAvatarDiameter = 160.
const quoteAuthorGap = 32.

func quoteTextSize(text string) float64 {
	n := utf8.RuneCountInString(text)
	words := len(quoteWords(text))
	switch {
	case n <= 60 && words <= 4:
		return 100
	case n <= 140 && words <= 10:
		return 84
	case n <= 300:
		return 64
	case n <= 700:
		return 50
	case n <= 1400:
		return 42
	default:
		return 36
	}
}

func drawQuoteMark(dc *gg.Context, face font.Face, mark rune, x, y float64) {
	bounds, _, _ := face.GlyphBounds(mark)
	dc.SetFontFace(face)
	dc.DrawString(string(mark), x-float64(bounds.Min.X)/64, y-float64(bounds.Min.Y)/64)
}

func renderQuote(q quoteContent, avatar image.Image) ([]byte, error) {
	return renderQuoteWithWallpaper(q, avatar, randomQuoteWallpaper())
}

func renderQuoteWithWallpaper(q quoteContent, avatar image.Image, index int) ([]byte, error) {
	if index < 0 || index >= quoteWallpaperCount {
		return nil, fmt.Errorf("неизвестный фон цитаты")
	}
	backgrounds, err := loadQuoteWallpapers()
	if err != nil {
		return nil, fmt.Errorf("ошибка фона цитаты: %w", err)
	}
	wallpaper := backgrounds[index]
	const width = quoteWallpaperWidth
	const padding = quoteCardPadding
	fs, err := loadQuoteFonts()
	if err != nil {
		return nil, fmt.Errorf("ошибка шрифта")
	}
	size := quoteTextSize(q.Text)
	textFace, err := fs.newFace(size, true)
	if err != nil {
		return nil, err
	}
	defer textFace.Close()
	nameFace, err := fs.newFace(40, true)
	if err != nil {
		return nil, err
	}
	defer nameFace.Close()
	dateFace, err := fs.newFace(28, false)
	if err != nil {
		return nil, err
	}
	defer dateFace.Close()
	markFace, err := fs.newFace(220, true)
	if err != nil {
		return nil, err
	}
	defer markFace.Close()
	dc := gg.NewContext(1, 1)
	dc.SetFontFace(textFace)
	lines := quoteWrap(dc, q.Text, width-2*padding)
	lineHeight := math.Ceil(size * 1.25)
	dc.SetFontFace(nameFace)
	names := quoteWrap(dc, q.Author, width-2*padding-quoteAvatarDiameter-quoteAuthorGap)
	footerHeight := math.Max(quoteAvatarDiameter, float64(len(names))*54+40)
	bodyTop := 168.
	bodyBottom := bodyTop + float64(len(lines))*lineHeight
	height := int(math.Ceil(math.Max(quoteWallpaperHeight, bodyBottom+120+footerHeight+padding)))
	if height > 6000 {
		return nil, fmt.Errorf("слишком много строк для одной карточки — выбери сообщение покороче")
	}
	dc = gg.NewContext(width, height)
	drawQuoteWallpaper(dc, wallpaper, width, height)
	setQuoteInk(dc, wallpaper, .75)
	drawQuoteMark(dc, markFace, '“', padding-8, 48)
	closingBounds, _, _ := markFace.GlyphBounds('”')
	closingWidth := float64(closingBounds.Max.X-closingBounds.Min.X) / 64
	drawQuoteMark(dc, markFace, '”', width-padding-closingWidth, bodyBottom+12)
	dc.SetFontFace(textFace)
	setQuoteInk(dc, wallpaper, 1)
	y := bodyTop + float64(textFace.Metrics().Ascent.Ceil())
	maxLineWidth := 0.
	for _, line := range lines {
		dc.DrawString(line, padding, y)
		w, _ := dc.MeasureString(line)
		maxLineWidth = math.Max(maxLineWidth, w)
		y += lineHeight
	}
	underlineWidth := math.Min(width-2*padding-closingWidth-48, math.Max(280, maxLineWidth))
	dc.SetLineWidth(9)
	dc.SetLineCap(gg.LineCapRound)
	dc.DrawLine(padding, bodyBottom+38, padding+underlineWidth, bodyBottom+38)
	dc.Stroke()
	footerY := float64(height) - padding - footerHeight
	const diameter = quoteAvatarDiameter
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
		setQuoteInk(dc, wallpaper, .14)
		dc.DrawRectangle(padding, footerY, diameter, diameter)
		dc.Fill()
		dc.SetFontFace(nameFace)
		setQuoteInk(dc, wallpaper, 1)
		dc.DrawStringAnchored(quoteInitials(q.Author), cx, cy, .5, .5)
	}
	dc.Pop()
	dc.ResetClip() // gg.Pop restores transforms but intentionally retains the clip mask.
	dc.SetFontFace(nameFace)
	setQuoteInk(dc, wallpaper, 1)
	nameTop := footerY + (footerHeight-(float64(len(names))*54+40))/2
	nameX := padding + quoteAvatarDiameter + quoteAuthorGap
	for i, line := range names {
		dc.DrawString(line, nameX, nameTop+40+float64(i)*54)
	}
	dc.SetFontFace(dateFace)
	setQuoteInk(dc, wallpaper, .65)
	dc.DrawString(quoteDate(q.Date), nameX, nameTop+float64(len(names))*54+28)
	buf := new(bytes.Buffer)
	if err := dc.EncodePNG(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
