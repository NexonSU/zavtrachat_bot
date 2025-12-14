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

// Write username on bonk picture and send to target
func Bonk(bot *gotgbot.Bot, context *ext.Context) error {
	if context.Message.ReplyToMessage == nil {
		return ReplyAndRemoveWithTarget("Просто отправь <code>/bonk</code> в ответ на чье-либо сообщение.", *context)
	}
	context.Message.Delete(bot, nil)

	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(bonk_png))
	im, _, err := image.Decode(reader)
	if err != nil {
		return err
	}

	dc := gg.NewContextForImage(im)
	dc.DrawImage(im, 0, 0)
	dc.SetRGB(1, 1, 1)
	s := UserFullName(context.Message.From)
	n := 4
	for dy := -n; dy <= n; dy++ {
		for dx := -n; dx <= n; dx++ {
			if dx*dx+dy*dy >= n*n {
				continue
			}
			x := 150 + float64(dx)
			y := 300 + float64(dy)
			dc.DrawStringAnchored(s, x, y, 0.5, 0.5)
		}
	}
	dc.SetRGB(0, 0, 0)
	dc.DrawStringAnchored(s, 150, 300, 0.5, 0.5)
	buf := new(bytes.Buffer)
	err = dc.EncodePNG(buf)
	if err != nil {
		return err
	}
	_, err = bot.SendSticker(context.Message.Chat.Id, gotgbot.InputFileByReader("bonk.png", buf), &gotgbot.SendStickerOpts{ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.EffectiveMessage.ReplyToMessage.MessageId}})
	return err
}
