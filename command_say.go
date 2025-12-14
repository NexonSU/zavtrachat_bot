package main

import (
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Send text in chat on /say
func Say(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		return KillSender(bot, context)
	}
	if len(context.Args()) == 1 {
		return ReplyAndRemoveWithTarget("Укажите сообщение.", *context)
	}
	context.Message.Delete(bot, nil)
	_, text, _ := strings.Cut(context.EffectiveMessage.Text, " ")
	for i := range context.Message.Entities {
		context.Message.Entities[i].Offset = context.Message.Entities[i].Offset - int64(len(strings.Split(context.Message.Text, " ")[0])) - 1
	}
	_, err := context.EffectiveChat.SendMessage(bot, text, &gotgbot.SendMessageOpts{Entities: context.Message.Entities})
	return err
}
