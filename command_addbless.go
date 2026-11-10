package main

import (
	"fmt"
	"html"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Adds bless text to DB
func AddBless(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.EffectiveSender.User.Id) {
		_, err := bot.SendAnimation(context.EffectiveChat.Id, gotgbot.InputFileByID("CgACAgQAAx0CQvXPNQABH62yYQHUkpaPOe79NW4ZnwYZWCNJXW8AAgoBAAK-qkVQnRXXGK03dEMgBA"), &gotgbot.SendAnimationOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.EffectiveMessage.MessageId, AllowSendingWithoutReply: true}})
		return err
	}
	var bless Bless
	if (context.EffectiveMessage.ReplyToMessage == nil && len(context.Args()) == 1) || (context.EffectiveMessage.ReplyToMessage != nil && len(context.Args()) != 1) {
		return Reply("Пример использования: <code>/addbless {текст}</code>\nИли отправь в ответ на сообщение с текстом <code>/addbless</code>", *context)
	}
	if context.EffectiveMessage.ReplyToMessage == nil {
		_, bless.Text, _ = strings.Cut(context.EffectiveMessage.Text, " ")
	} else {
		if context.EffectiveMessage.ReplyToMessage.Text != "" {
			bless.Text = context.EffectiveMessage.ReplyToMessage.Text
		} else {
			return Reply("Я не смог найти текст в указанном сообщении.", *context)
		}
	}
	if strings.TrimSpace(bless.Text) == "" {
		return Reply("Укажи непустую фразу блесса.", *context)
	}
	variants, explicit, err := makePhraseVariants(bless.Text)
	if err != nil {
		return Reply(err.Error(), *context)
	}
	if len([]rune(variants.Male)) > 200 || len([]rune(variants.Female)) > 200 {
		return Reply("Bless не может быть длиннее 200 символов.", *context)
	}
	if explicit {
		bless.Text = variants.Male
	}
	bless.PhraseVariants = variants
	if err := savePhraseVariants(&bless, explicit); err != nil {
		return err
	}
	return Reply(fmt.Sprintf("Bless добавлен как <code>%v</code>.", html.EscapeString(bless.Text))+variants.confirmation(), *context)
}
