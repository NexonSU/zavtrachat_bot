package main

import (
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Sed Replace text in target message
func Sed(bot *gotgbot.Bot, context *ext.Context) error {
	var foo = strings.Split(context.Message.Text, "/")[1]
	var bar = strings.Split(context.Message.Text, "/")[2]
	if context.Message.ReplyToMessage == nil || foo == "" || bar == "" || len(context.Args()) != 2 {
		return ReplyAndRemove("Пример использования:\n/sed {патерн вида s/foo/bar/} в ответ на сообщение.", *context)
	}
	_, err := context.Message.Reply(bot, strings.ReplaceAll(context.Message.ReplyToMessage.Text, foo, bar), &gotgbot.SendMessageOpts{ParseMode: "HTML"})
	return err
}
