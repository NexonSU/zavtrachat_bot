package main

import (
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func SalvationListFull(bot *gotgbot.Bot, ctx *ext.Context) error {
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
	var phrases []Salvation
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
