package main

import (
	"fmt"

	tele "gopkg.in/telebot.v3"
)

// Ban user on /ban
func Ban(context tele.Context) error {
	if (context.Message().ReplyTo == nil && len(context.Args()) == 0) || (context.Message().ReplyTo != nil && len(context.Args()) > 1) {
		return ReplyAndRemove("Пример использования: <code>/ban {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>/ban</code>\nЕсли нужно забанить на время, то добавь время в секундах через пробел.", context)
	}
	target, untildate, err := FindUserInMessage(context)
	if err != nil {
		return err
	}
	TargetChatMember, err := Bot.ChatMemberOf(context.Chat(), &target)
	if err != nil {
		return err
	}
	TargetChatMember.RestrictedUntil = untildate
	err = Bot.Ban(context.Chat(), TargetChatMember)
	if err != nil {
		return err
	}
	return ReplyAndRemove(fmt.Sprintf("Пользователь <a href=\"tg://user?id=%v\">%v</a> забанен%v.", target.ID, UserFullName(&target), RestrictionTimeMessage(untildate)), context)
}
