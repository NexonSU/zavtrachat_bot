package main

import (
	"strings"

	tele "gopkg.in/telebot.v3"
)

// Send text in chat on /say
func Say(context tele.Context) error {
	if len(context.Args()) == 0 {
		return ReplyAndRemove("Укажите сообщение.", context)
	}
	context.Delete()
	for i := range context.Message().Entities {
		context.Message().Entities[i].Offset = context.Message().Entities[i].Offset - len(strings.Split(context.Message().Text, " ")[0]) - 1
	}
	return context.Send(context.Message().Payload, context.Message().Entities)
}
