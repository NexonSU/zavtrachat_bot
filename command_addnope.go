package main

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Adds nope text to DB
func AddNope(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.EffectiveSender.User.Id) {
		return KillSender(bot, context)
	}
	var nope Nope
	if (context.EffectiveMessage.ReplyToMessage == nil && len(context.Args()) == 1) || (context.EffectiveMessage.ReplyToMessage != nil && len(context.Args()) != 1) {
		return ReplyAndRemoveWithTarget("Пример использования: <code>/addnope {текст}</code>\nИли отправь в ответ на сообщение с текстом <code>/addnope</code>", *context)
	}
	if context.EffectiveMessage.ReplyToMessage == nil {
		_, nope.Text, _ = strings.Cut(context.EffectiveMessage.Text, " ")
	} else {
		if context.EffectiveMessage.ReplyToMessage.Text != "" {
			nope.Text = strings.ToLower(context.EffectiveMessage.ReplyToMessage.Text)
		} else {
			return ReplyAndRemoveWithTarget("Я не смог найти текст в указанном сообщении.", *context)
		}
	}
	result := DB.Create(&nope)
	if result.Error != nil {
		return ReplyAndRemoveWithTarget(fmt.Sprintf("Не удалось добавить nope, ошибка:\n<code>%v</code>", result.Error.Error()), *context)
	}
	return ReplyAndRemoveWithTarget(fmt.Sprintf("Nope добавлен как <code>%v</code>.", nope.Text), *context)
}
