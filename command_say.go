package main

import (
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Send text in chat on /say
func Say(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		_, err := bot.SendAnimation(context.Message.Chat.Id, gotgbot.InputFileByID("CgACAgQAAx0CQvXPNQABH62yYQHUkpaPOe79NW4ZnwYZWCNJXW8AAgoBAAK-qkVQnRXXGK03dEMgBA"), &gotgbot.SendAnimationOpts{ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.MessageId, AllowSendingWithoutReply: true}})
		return err
	}
	if len(context.Args()) == 1 {
		return ReplyAndRemove("Укажите сообщение.", *context)
	}
	context.Message.Delete(bot, nil)
	_, text, _ := strings.Cut(context.EffectiveMessage.Text, " ")
	for i := range context.Message.Entities {
		context.Message.Entities[i].Offset = context.Message.Entities[i].Offset - int64(len(strings.Split(context.Message.Text, " ")[0])) - 1
	}
	_, err := context.EffectiveChat.SendMessage(bot, text, &gotgbot.SendMessageOpts{ParseMode: "HTML", Entities: context.Message.Entities})
	return err
}
