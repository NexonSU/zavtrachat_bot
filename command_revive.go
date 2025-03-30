package main

import (
	"fmt"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Unmute user on /unmute
func Revive(bot *gotgbot.Bot, context *ext.Context) error {
	if (context.Message.ReplyToMessage == nil && len(context.Args()) != 2) || (context.Message.ReplyToMessage != nil && len(context.Args()) != 1) {
		return ReplyAndRemove("Пример использования: <code>/unmute {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>/unmute</code>", *context)
	}
	target, _, err := FindUserInMessage(*context)
	if err != nil {
		return err
	}
	_, err = Bot.RestrictChatMember(context.Message.Chat.Id, target.Id, gotgbot.ChatPermissions{CanSendMessages: true}, &gotgbot.RestrictChatMemberOpts{UntilDate: time.Now().Add(time.Second * time.Duration(60)).Unix()})
	if err != nil {
		return err
	}
	return ReplyAndRemove(fmt.Sprintf("%v возродился в чате.", MentionUser(&target)), *context)
}
