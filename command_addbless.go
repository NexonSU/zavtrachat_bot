package main

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Adds bless text to DB
func AddBless(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.EffectiveSender.User.Id) {
		_, err := bot.SendAnimation(context.EffectiveChat.Id, gotgbot.InputFileByID("CgACAgQAAx0CQvXPNQABH62yYQHUkpaPOe79NW4ZnwYZWCNJXW8AAgoBAAK-qkVQnRXXGK03dEMgBA"), &gotgbot.SendAnimationOpts{ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.EffectiveMessage.MessageId, AllowSendingWithoutReply: true}})
		return err
	}
	var bless Bless
	if (context.EffectiveMessage.ReplyToMessage == nil && len(context.Args()) == 1) || (context.EffectiveMessage.ReplyToMessage != nil && len(context.Args()) != 1) {
		return ReplyAndRemoveWithTarget("Пример использования: <code>/addbless {текст}</code>\nИли отправь в ответ на сообщение с текстом <code>/addbless</code>", *context)
	}
	if context.EffectiveMessage.ReplyToMessage == nil {
		_, bless.Text, _ = strings.Cut(context.EffectiveMessage.Text, " ")
	} else {
		if context.EffectiveMessage.ReplyToMessage.Text != "" {
			bless.Text = context.EffectiveMessage.ReplyToMessage.Text
		} else {
			return ReplyAndRemoveWithTarget("Я не смог найти текст в указанном сообщении.", *context)
		}
	}
	if len([]rune(bless.Text)) > 200 {
		return ReplyAndRemoveWithTarget("Bless не может быть длиннее 200 символов.", *context)
	}
	result := DB.Create(&bless)
	if result.Error != nil {
		return result.Error
	}
	return ReplyAndRemoveWithTarget(fmt.Sprintf("Bless добавлен как <code>%v</code>.", bless.Text), *context)
}
