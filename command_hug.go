package main

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/png"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/fogleman/gg"
)

// Write username on hug picture and send to target
func Hug(bot *gotgbot.Bot, context *ext.Context) error {
	var err error
	if context.Message.ReplyToMessage == nil {
		return ReplyAndRemove("Просто отправь <code>/hug</code> в ответ на чье-либо сообщение.", *context)
	}
	context.Message.Delete(bot, nil)
	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(hug_png))
	im, _, err := image.Decode(reader)
	if err != nil {
		return err
	}

	dc := gg.NewContextForImage(im)
	dc.DrawImage(im, 0, 0)
	dc.Rotate(gg.Radians(15))
	dc.SetRGB(1, 1, 1)
	s := UserFullName(context.Message.From)
	n := 4
	for dy := -n; dy <= n; dy++ {
		for dx := -n; dx <= n; dx++ {
			if dx*dx+dy*dy >= n*n {
				continue
			}
			x := 400 + float64(dx)
			y := -30 + float64(dy)
			dc.DrawStringAnchored(s, x, y, 0.5, 0.5)
		}
	}
	dc.SetRGB(0, 0, 0)
	dc.DrawStringAnchored(s, 400, -30, 0.5, 0.5)
	buf := new(bytes.Buffer)
	err = dc.EncodePNG(buf)
	if err != nil {
		return err
	}
	_, err = bot.SendSticker(context.Message.Chat.Id, gotgbot.InputFileByReader("hug.png", buf), &gotgbot.SendStickerOpts{ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.ReplyToMessage.MessageId}})
	return err
}
