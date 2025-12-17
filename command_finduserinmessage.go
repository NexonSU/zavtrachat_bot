package main

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Mute user on /mute
func FindUserInMessageTest(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		return KillSender(bot, context)
	}
	if context.Message.ReplyToMessage == nil {
		return ReplyAndRemoveWithTarget("Укажите сообщение", *context)
	}
	user, err := FindUserInMessage(*context.Message.ReplyToMessage)
	if err != nil {
		return err
	}
	return ReplyAndRemoveWithTarget(fmt.Sprintf("Пользователь %v:\nUsername: %v\nID: %v\nFirstname: %v\nLastname: %v", MentionUser(&user), user.Username, user.Id, user.FirstName, user.LastName), *context)
}
