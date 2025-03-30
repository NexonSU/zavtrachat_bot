package main

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Kick user on /kick
func Kick(bot *gotgbot.Bot, context *ext.Context) error {
	if (context.Message.ReplyToMessage == nil && len(context.Args()) == 1) || (context.Message.ReplyToMessage != nil && len(context.Args()) != 1) {
		return ReplyAndRemove("Пример использования: <code>/kick {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>/kick</code>", *context)
	}
	target, _, err := FindUserInMessage(*context)
	if err != nil {
		return err
	}
	_, err = bot.UnbanChatMember(context.Message.Chat.Id, target.Id, nil)
	if err != nil {
		return err
	}
	return ReplyAndRemove(fmt.Sprintf("Пользователь <a href=\"tg://user?id=%v\">%v</a> исключен.", target.Id, UserFullName(&target)), *context)
}
