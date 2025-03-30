package main

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Adds bless text to DB
func AddBless(bot *gotgbot.Bot, context *ext.Context) error {
	var bless Bless
	if (context.Message.ReplyToMessage == nil && len(context.Args()) == 1) || (context.Message.ReplyToMessage != nil && len(context.Args()) != 1) {
		return ReplyAndRemove("Пример использования: <code>/addbless {текст}</code>\nИли отправь в ответ на сообщение с текстом <code>/addbless</code>", *context)
	}
	if context.Message.ReplyToMessage == nil {
		bless.Text = context.Message.Text
	} else {
		if context.Message.ReplyToMessage.Text != "" {
			bless.Text = context.Message.ReplyToMessage.Text
		} else {
			return ReplyAndRemove("Я не смог найти текст в указанном сообщении.", *context)
		}
	}
	if len([]rune(bless.Text)) > 200 {
		return ReplyAndRemove("Bless не может быть длиннее 200 символов.", *context)
	}
	result := DB.Create(&bless)
	if result.Error != nil {
		return result.Error
	}
	return ReplyAndRemove(fmt.Sprintf("Bless добавлен как <code>%v</code>.", bless.Text), *context)
}
