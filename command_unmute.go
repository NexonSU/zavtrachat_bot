package main

import (
	"fmt"
	"time"

	tele "gopkg.in/telebot.v3"
)

// Unmute user on /unmute
func Unmute(context tele.Context) error {
	if (context.Message().ReplyTo == nil && len(context.Args()) != 1) || (context.Message().ReplyTo != nil && len(context.Args()) != 0) {
		return ReplyAndRemove("Пример использования: <code>/unmute {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>/unmute</code>", context)
	}
	target, _, err := FindUserInMessage(context)
	if err != nil {
		return err
	}
	TargetChatMember, err := Bot.ChatMemberOf(context.Chat(), &target)
	if err != nil {
		return err
	}
	TargetChatMember.CanSendMessages = true
	TargetChatMember.CanSendMedia = true
	TargetChatMember.CanSendPolls = true
	TargetChatMember.CanSendOther = true
	TargetChatMember.CanAddPreviews = true
	TargetChatMember.RestrictedUntil = time.Now().Unix() + 60
	err = Bot.Restrict(context.Chat(), TargetChatMember)
	if err != nil {
		return err
	}
	return ReplyAndRemove(fmt.Sprintf("<a href=\"tg://user?id=%v\">%v</a> снова может отправлять сообщения в чат.", target.ID, UserFullName(&target)), context)
}
