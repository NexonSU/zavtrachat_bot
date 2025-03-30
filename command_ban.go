package main

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Ban user on /ban
func Ban(bot *gotgbot.Bot, context *ext.Context) error {
	if (context.Message.ReplyToMessage == nil && len(context.Args()) == 1) || (context.Message.ReplyToMessage != nil && len(context.Args()) > 2) {
		return ReplyAndRemove("Пример использования: <code>/ban {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>/ban</code>\nЕсли нужно забанить на время, то добавь время в секундах через пробел.", *context)
	}
	target, untildate, err := FindUserInMessage(*context)
	if err != nil {
		return err
	}
	result, err := bot.BanChatMember(context.Message.Chat.Id, target.Id, &gotgbot.BanChatMemberOpts{UntilDate: untildate})
	if err != nil {
		return err
	}
	if result {
		return ReplyAndRemove(fmt.Sprintf("Пользователь <a href=\"tg://user?id=%v\">%v</a> забанен%v.", target.Id, UserFullName(&target), RestrictionTimeMessage(untildate)), *context)
	} else {
		return ReplyAndRemove(fmt.Sprintf("Пользователь <a href=\"tg://user?id=%v\">%v</a> забанен%v.", target.Id, UserFullName(&target), RestrictionTimeMessage(untildate)), *context)
	}
}
