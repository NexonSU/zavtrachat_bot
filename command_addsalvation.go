package main

import (
	"fmt"
	"html"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Adds salvation text to DB
func AddSalvation(bot *gotgbot.Bot, context *ext.Context) error {
	if context.EffectiveChat == nil || context.EffectiveUser == nil || context.EffectiveMessage == nil {
		return nil
	}
	if !IsAdminOrModer(context.EffectiveUser.Id) {
		_, err := bot.SendAnimation(context.EffectiveChat.Id, gotgbot.InputFileByID("CgACAgQAAx0CQvXPNQABH62yYQHUkpaPOe79NW4ZnwYZWCNJXW8AAgoBAAK-qkVQnRXXGK03dEMgBA"), &gotgbot.SendAnimationOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.EffectiveMessage.MessageId, AllowSendingWithoutReply: true}})
		return err
	}
	var salvation Salvation
	if (context.EffectiveMessage.ReplyToMessage == nil && len(context.Args()) == 1) || (context.EffectiveMessage.ReplyToMessage != nil && len(context.Args()) != 1) {
		return Reply("Пример использования: <code>/addsalvation {текст}</code>\nИли отправь в ответ на сообщение с текстом <code>/addsalvation</code>", *context)
	}
	if context.EffectiveMessage.ReplyToMessage == nil {
		_, salvation.Text, _ = strings.Cut(context.EffectiveMessage.Text, " ")
	} else {
		if context.EffectiveMessage.ReplyToMessage.Text != "" {
			salvation.Text = context.EffectiveMessage.ReplyToMessage.Text
		} else {
			return Reply("Я не смог найти текст в указанном сообщении.", *context)
		}
	}
	if strings.TrimSpace(salvation.Text) == "" {
		return Reply("Укажи непустой текст спасения.", *context)
	}
	if len([]rune(salvation.Text)) > 200 {
		return Reply("Фраза спасения не может быть длиннее 200 символов.", *context)
	}
	result := DB.Create(&salvation)
	if result.Error != nil {
		return result.Error
	}
	return Reply(fmt.Sprintf("Фраза спасения добавлена как <code>%v</code>.", html.EscapeString(salvation.Text)), *context)
}
