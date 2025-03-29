package main

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/png"
	"strings"

	"github.com/fogleman/gg"
	tele "gopkg.in/telebot.v3"
)

// Write username on bonk picture and send to target
func Bonk(context tele.Context) error {
	if context.Message().ReplyTo == nil {
		return ReplyAndRemove("Просто отправь <code>/bonk</code> в ответ на чье-либо сообщение.", context)
	}
	context.Delete()

	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(bonk_png))
	im, _, err := image.Decode(reader)
	if err != nil {
		return err
	}

	dc := gg.NewContextForImage(im)
	dc.DrawImage(im, 0, 0)
	dc.SetRGB(0, 0, 0)
	err = dc.LoadFontFace(Config.FontPath, 20)
	if err != nil {
		return err
	}
	dc.SetRGB(1, 1, 1)
	s := UserFullName(context.Sender())
	n := 4
	for dy := -n; dy <= n; dy++ {
		for dx := -n; dx <= n; dx++ {
			if dx*dx+dy*dy >= n*n {
				continue
			}
			x := 140 + float64(dx)
			y := 290 + float64(dy)
			dc.DrawStringAnchored(s, x, y, 0.5, 0.5)
		}
	}
	dc.SetRGB(0, 0, 0)
	dc.DrawStringAnchored(s, 140, 290, 0.5, 0.5)
	buf := new(bytes.Buffer)
	err = dc.EncodePNG(buf)
	if err != nil {
		return err
	}
	return context.Send(&tele.Sticker{File: tele.FromReader(buf)}, &tele.SendOptions{ReplyTo: context.Message().ReplyTo})
}
