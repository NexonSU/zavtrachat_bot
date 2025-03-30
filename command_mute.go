package main

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Mute user on /mute
func Mute(bot *gotgbot.Bot, context *ext.Context) error {
	if (context.Message.ReplyToMessage == nil && len(context.Args()) == 1) || (context.Message.ReplyToMessage != nil && len(context.Args()) > 2) {
		return ReplyAndRemove("Пример использования: <code>/mute {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>/mute</code>\nЕсли нужно замьютить на время, то добавь время в секундах через пробел.", *context)
	}
	target, untildate, err := FindUserInMessage(*context)
	if err != nil {
		return err
	}
	_, err = Bot.RestrictChatMember(context.Message.Chat.Id, target.Id, gotgbot.ChatPermissions{CanSendMessages: false}, &gotgbot.RestrictChatMemberOpts{UntilDate: untildate})
	if err != nil {
		return err
	}
	return ReplyAndRemove(fmt.Sprintf("Пользователь <a href=\"tg://user?id=%v\">%v</a> больше не может отправлять сообщения%v.", target.Id, UserFullName(&target), RestrictionTimeMessage(untildate)), *context)
}
