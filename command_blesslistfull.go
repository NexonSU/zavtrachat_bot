package main

import (
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func BlessListFull(bot *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveChat == nil || ctx.EffectiveUser == nil {
		return nil
	}
	if ctx.EffectiveChat.Type != "private" {
		_, err := bot.SendMessage(ctx.EffectiveChat.Id, "Команда работает только для администраторов и модераторов чата в личке с ботом.", nil)
		return err
	}
	if !IsAdminOrModer(ctx.EffectiveUser.Id) {
		_, err := bot.SendMessage(ctx.EffectiveChat.Id, "Просмотр всех фраз доступен только администраторам и модераторам чата.", nil)
		return err
	}
	var phrases []Bless
	if err := DB.Order("text").Find(&phrases).Error; err != nil {
		return err
	}
	texts := make([]string, 0, len(phrases))
	for _, phrase := range phrases {
		texts = append(texts, phrase.Text)
	}
	text := strings.Join(texts, "\n\n")
	if strings.TrimSpace(text) == "" {
		text = "Фраз пока нет."
	}
	for _, part := range blessListFullParts(text) {
		if _, err := bot.SendMessage(ctx.EffectiveChat.Id, part, nil); err != nil {
			return err
		}
	}
	return nil
}

// Count UTF-16 units conservatively and preserve every character, including
// legacy phrases longer than one Telegram message. Prefer phrase boundaries.
func blessListFullParts(text string) []string {
	var parts []string
	for len(text) > 0 {
		units, end, boundary := 0, 0, 0
		for i, r := range text {
			size := 1
			if r > 0xFFFF {
				size = 2
			}
			if units+size > 4000 {
				break
			}
			units += size
			end = i + len(string(r))
			if strings.HasPrefix(text[i:], "\n\n") && i > 0 {
				boundary = i
			}
		}
		if end < len(text) && boundary > 0 {
			end = boundary
		}
		parts = append(parts, text[:end])
		text = text[end:]
	}
	return parts
}
