package main

import (
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Send text in chat on /say
func Say(bot *gotgbot.Bot, context *ext.Context) error {
	if len(context.Args()) == 1 {
		return ReplyAndRemove("Укажите сообщение.", *context)
	}
	context.Message.Delete(bot, nil)
	for i := range context.Message.Entities {
		context.Message.Entities[i].Offset = context.Message.Entities[i].Offset - int64(len(strings.Split(context.Message.Text, " ")[0])) - 1
	}
	_, err := context.EffectiveChat.SendMessage(bot, context.Message.Text, &gotgbot.SendMessageOpts{ParseMode: "HTML", Entities: context.Message.Entities})
	return err
}
