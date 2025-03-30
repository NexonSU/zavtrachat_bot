package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Resend post on user request
func SaveToPM(bot *gotgbot.Bot, context *ext.Context) error {
	if context.Message == nil || context.Message.ReplyToMessage == nil {
		return ReplyAndRemove("Пример использования:\n/topm в ответ на какое-либо сообщение\nБот должен быть запущен и разблокирован в личке.", *context)
	}
	link := fmt.Sprintf("https://t.me/c/%v/%v", strings.TrimLeft(strings.TrimLeft(strconv.Itoa(int(context.Message.Chat.Id)), "-1"), "0"), context.Message.ReplyToMessage.MessageId)
	var err error
	msg, err := Bot.CopyMessage(context.Message.From.Id, context.Message.Chat.Id, context.Message.ReplyToMessage.MessageId, nil)
	if err != nil {
		return err
	}
	Bot.SendMessage(context.Message.From.Id, link, &gotgbot.SendMessageOpts{ParseMode: "HTML", ReplyParameters: &gotgbot.ReplyParameters{MessageId: msg.MessageId, AllowSendingWithoutReply: true}})
	return Remove(bot, context)
}
