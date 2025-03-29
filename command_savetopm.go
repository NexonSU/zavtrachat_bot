package main

import (
	"fmt"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v3"
)

// Resend post on user request
func SaveToPM(context tele.Context) error {
	if context.Message() == nil || context.Message().ReplyTo == nil {
		return ReplyAndRemove("Пример использования:\n/topm в ответ на какое-либо сообщение\nБот должен быть запущен и разблокирован в личке.", context)
	}
	link := fmt.Sprintf("https://t.me/c/%v/%v", strings.TrimLeft(strings.TrimLeft(strconv.Itoa(int(context.Chat().ID)), "-1"), "0"), context.Message().ReplyTo.ID)
	var err error
	msg, err := Bot.Copy(context.Sender(), context.Message().ReplyTo)
	if err != nil {
		return err
	}
	Bot.Send(context.Sender(), link, &tele.SendOptions{ReplyTo: msg, AllowWithoutReply: false})
	return context.Delete()
}
