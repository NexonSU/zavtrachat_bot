package main

import (
	"fmt"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Unmute user on /unmute
func Unmute(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		return KillSender(bot, context)
	}
	if (context.Message.ReplyToMessage == nil && len(context.Args()) != 2) || (context.Message.ReplyToMessage != nil && len(context.Args()) != 1) {
		return ReplyAndRemoveWithTarget("Пример использования: <code>/unmute {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>/unmute</code>", *context)
	}
	target, err := FindUserInMessage(*context)
	if err != nil {
		return err
	}
	_, err = Bot.RestrictChatMember(context.Message.Chat.Id, target.Id, gotgbot.ChatPermissions{CanSendMessages: true}, &gotgbot.RestrictChatMemberOpts{UntilDate: time.Now().Add(time.Second * time.Duration(60)).Unix()})
	if err != nil {
		return err
	}
	return ReplyAndRemoveWithTarget(fmt.Sprintf("<a href=\"tg://user?id=%v\">%v</a> снова может отправлять сообщения в чат.", target.Id, UserFullName(&target)), *context)
}
