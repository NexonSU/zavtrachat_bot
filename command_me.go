package main

import (
	"fmt"
	"strings"

	tele "gopkg.in/telebot.v3"
)

// Send formatted text on /me
func Me(context tele.Context) error {
	if len(context.Args()) == 0 {
		return ReplyAndRemove("Пример использования:\n<code>/me {делает что-то}</code>", context)
	}
	Bot.Delete(context.Message())
	return context.Send(fmt.Sprintf("<code>%v %v</code>", strings.Replace(UserFullName(context.Sender()), "💥", "", -1), context.Data()))
}
