package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Resend post on user request
func SaveToPM(bot *gotgbot.Bot, context *ext.Context) error {
	if context.EffectiveMessage == nil || context.EffectiveMessage.ReplyToMessage == nil {
		return ReplyAndRemove("Пример использования:\n/topm в ответ на какое-либо сообщение\nБот должен быть запущен и разблокирован в личке.", *context)
	}
	link := fmt.Sprintf("https://t.me/c/%v/%v", strings.TrimLeft(strings.TrimLeft(strconv.Itoa(int(context.EffectiveChat.Id)), "-1"), "0"), context.EffectiveMessage.ReplyToMessage.MessageId)
	var err error
	var msg *gotgbot.MessageId
	if context.EffectiveMessage.ReplyToMessage.MediaGroupId != "" && chatMediaGroups[context.EffectiveMessage.ReplyToMessage.MediaGroupId] != nil {
		slices.Sort(chatMediaGroups[context.EffectiveMessage.ReplyToMessage.MediaGroupId])
		msgs, err := Bot.CopyMessages(context.EffectiveSender.User.Id, context.EffectiveChat.Id, chatMediaGroups[context.EffectiveMessage.ReplyToMessage.MediaGroupId], nil)
		if err != nil {
			return err
		}
		msg = &msgs[0]
	} else {
		msg, err = Bot.CopyMessage(context.EffectiveSender.User.Id, context.EffectiveChat.Id, context.EffectiveMessage.ReplyToMessage.MessageId, nil)
		if err != nil {
			return err
		}
	}
	Bot.SendMessage(context.EffectiveSender.User.Id, link, &gotgbot.SendMessageOpts{ReplyParameters: &gotgbot.ReplyParameters{MessageId: msg.MessageId, AllowSendingWithoutReply: true}})
	return Remove(bot, context)
}
