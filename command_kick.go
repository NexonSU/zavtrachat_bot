package main

import (
	"fmt"
	"time"

	tele "gopkg.in/telebot.v3"
)

// Kick user on /kick
func Kick(context tele.Context) error {
	if (context.Message().ReplyTo == nil && len(context.Args()) == 0) || (context.Message().ReplyTo != nil && len(context.Args()) != 0) {
		return ReplyAndRemove("Пример использования: <code>/kick {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>/kick</code>", context)
	}
	target, _, err := FindUserInMessage(context)
	if err != nil {
		return err
	}
	TargetChatMember, err := Bot.ChatMemberOf(context.Chat(), &target)
	if err != nil {
		return err
	}
	TargetChatMember.RestrictedUntil = time.Now().Unix() + 60
	err = Bot.Ban(context.Chat(), TargetChatMember)
	if err != nil {
		return err
	}
	err = Bot.Unban(context.Chat(), &target)
	if err != nil {
		return err
	}
	return ReplyAndRemove(fmt.Sprintf("Пользователь <a href=\"tg://user?id=%v\">%v</a> исключен.", target.ID, UserFullName(&target)), context)
}
