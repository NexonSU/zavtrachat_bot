package main

import (
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"math/rand/v2"
	"sync"

	"github.com/fogleman/gg"
)

//go:embed assets/sweet-backgrounds/*.jpg
var sweetBackgroundFiles embed.FS

var loadSweetBackgrounds = sync.OnceValues(func() ([]image.Image, error) {
	files, err := sweetBackgroundFiles.ReadDir("assets/sweet-backgrounds")
	if err != nil {
		return nil, err
	}
	backgrounds := make([]image.Image, 0, len(files))
	for _, file := range files {
		f, err := sweetBackgroundFiles.Open("assets/sweet-backgrounds/" + file.Name())
		if err != nil {
			return nil, err
		}
		background, err := jpeg.Decode(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("decode sweet background %s: %w", file.Name(), err)
		}
		backgrounds = append(backgrounds, background)
	}
	return backgrounds, nil
})

var sweetNames = [...]string{
	"сладкий пирожок", "булочка", "зайка", "солнышко", "лапочка",
	"сладулечка", "котенок", "пушинка", "облачко", "звездочка",
	"лучик света", "медовый пряник", "клубничка", "малинка", "персик",
	"зефирка", "мармеладка", "конфетка", "карамелька", "вафелька",
	"пончик с глазурью", "ванильный кексик", "круассанчик", "печенька", "блинчик с медом",
	"творожный сырничек", "коричная улиточка", "шоколадная капелька", "макарунчик", "мятная пастилка",
	"цветочек", "ромашка", "одуванчик", "лютик", "колокольчик",
	"лучик тепла", "лунный зайчик", "пушистый комочек", "котик в пледе", "плюшевый мишка",
	"маленькое чудо", "сокровище", "радость чата", "уютный огонек", "мурчащий клубочек",
	"панда в пижаме", "лисенок", "обнимашка", "сахарная вата", "искорка радости",
}

type sweetSparkleStyle struct {
	color    color.NRGBA
	count    int
	points   int
	tipRatio float64
}

var sweetSparkles = [...]sweetSparkleStyle{
	{color.NRGBA{255, 195, 66, 225}, 34, 4, .24},  // Gold glints
	{color.NRGBA{246, 110, 165, 205}, 25, 5, .48}, // Raspberry stars
	{color.NRGBA{123, 106, 222, 200}, 24, 4, .56}, // Lavender diamonds
	{color.NRGBA{64, 172, 203, 195}, 39, 0, 0},    // Aqua dust
	{color.NRGBA{245, 145, 76, 220}, 29, 6, .32},  // Peach bursts
	{color.NRGBA{110, 182, 137, 210}, 26, 4, .29}, // Mint glints
	{color.NRGBA{255, 255, 255, 244}, 46, 0, 0},   // Pearl dust
	{color.NRGBA{122, 162, 232, 215}, 29, 5, .43}, // Blue stars
	{color.NRGBA{222, 127, 210, 215}, 23, 8, .32}, // Orchid bursts
	{color.NRGBA{248, 125, 126, 215}, 31, 4, .42}, // Coral diamonds
}

const sweetAvatarY = 750.

func drawSweetSparkles(dc *gg.Context, style sweetSparkleStyle) {
	dc.SetColor(style.color)
	for i := 0; i < style.count; i++ {
		x := float64(34 + rand.IntN(1132))
		y := float64(34 + rand.IntN(1132))
		// Keep the artwork clear of the caption and portrait, including their edges.
		if (x > 74 && x < 1126 && y < 366) || math.Hypot(x-600, y-sweetAvatarY) < 395 {
			continue
		}
		radius := float64(5 + rand.IntN(15))
		if style.points == 0 {
			dc.DrawCircle(x, y, radius*.43)
			dc.Fill()
			continue
		}
		for vertex := 0; vertex < style.points*2; vertex++ {
			angle := math.Pi*float64(vertex)/float64(style.points) - math.Pi/2
			distance := radius
			if vertex%2 != 0 {
				distance *= style.tipRatio
			}
			px, py := x+distance*math.Cos(angle), y+distance*math.Sin(angle)
			if vertex == 0 {
				dc.MoveTo(px, py)
			} else {
				dc.LineTo(px, py)
			}
		}
		dc.ClosePath()
		dc.Fill()
	}
}

func renderSweet(name string, avatar image.Image) ([]byte, error) {
	backgrounds, err := loadSweetBackgrounds()
	if err != nil {
		return nil, err
	}
	background := backgrounds[rand.IntN(len(backgrounds))]
	nickname := sweetNames[rand.IntN(len(sweetNames))]
	sparkles := sweetSparkles[rand.IntN(len(sweetSparkles))]
	return renderPortraitCard(name, "", nickname+"!", avatar, background, sweetAvatarY, 80, func(dc *gg.Context) {
		drawSweetSparkles(dc, sparkles)
	})
}
