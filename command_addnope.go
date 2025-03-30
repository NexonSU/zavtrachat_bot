package main

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Adds nope text to DB
func AddNope(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		_, err := bot.SendAnimation(context.Message.Chat.Id, gotgbot.InputFileByID("CgACAgQAAx0CQvXPNQABH62yYQHUkpaPOe79NW4ZnwYZWCNJXW8AAgoBAAK-qkVQnRXXGK03dEMgBA"), &gotgbot.SendAnimationOpts{ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.MessageId, AllowSendingWithoutReply: true}})
		return err
	}
	var nope Nope
	if (context.Message.ReplyToMessage == nil && len(context.Args()) == 1) || (context.Message.ReplyToMessage != nil && len(context.Args()) != 1) {
		return ReplyAndRemove("Пример использования: <code>/addnope {текст}</code>\nИли отправь в ответ на сообщение с текстом <code>/addnope</code>", *context)
	}
	if context.Message.ReplyToMessage == nil {
		nope.Text = strings.TrimPrefix(context.Message.Text, strings.Split(context.Message.Text, " ")[0]+" ")
	} else {
		if context.Message.ReplyToMessage.Text != "" {
			nope.Text = strings.ToLower(context.Message.ReplyToMessage.Text)
		} else {
			return ReplyAndRemove("Я не смог найти текст в указанном сообщении.", *context)
		}
	}
	result := DB.Create(&nope)
	if result.Error != nil {
		return ReplyAndRemove(fmt.Sprintf("Не удалось добавить nope, ошибка:\n<code>%v</code>", result.Error.Error()), *context)
	}
	return ReplyAndRemove(fmt.Sprintf("Nope добавлен как <code>%v</code>.", nope.Text), *context)
}
