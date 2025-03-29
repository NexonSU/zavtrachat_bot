package main

import (
	"strings"

	tele "gopkg.in/telebot.v3"
)

// Sed Replace text in target message
func Sed(context tele.Context) error {
	var foo = strings.Split(context.Data(), "/")[1]
	var bar = strings.Split(context.Data(), "/")[2]
	if context.Message().ReplyTo == nil || foo == "" || bar == "" || len(context.Args()) != 1 {
		return ReplyAndRemove("Пример использования:\n/sed {патерн вида s/foo/bar/} в ответ на сообщение.", context)
	}
	return context.Reply(strings.ReplaceAll(context.Message().ReplyTo.Text, foo, bar))
}
