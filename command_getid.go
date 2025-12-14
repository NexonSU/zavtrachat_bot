package main

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Send userid on /getid
func Getid(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		return KillSender(bot, context)
	}
	if context.Message.ReplyToMessage != nil && context.Message.ReplyToMessage.From != nil {
		_, err := Bot.SendMessage(context.Message.From.Id, fmt.Sprintf("Firstname: %v\nLastname: %v\nUsername: %v\nUserID: %v", context.Message.ReplyToMessage.From.FirstName, context.Message.ReplyToMessage.From.LastName, context.Message.ReplyToMessage.From.Username, context.Message.ReplyToMessage.From.Id), &gotgbot.SendMessageOpts{})
		return err
	}
	if len(context.Args()) == 2 {
		target, err := FindUserInMessage(*context)
		if err != nil {
			return err
		}
		_, err = Bot.SendMessage(context.Message.From.Id, fmt.Sprintf("Firstname: %v\nLastname: %v\nUsername: %v\nUserID: %v", target.FirstName, target.LastName, target.Username, target.Id), &gotgbot.SendMessageOpts{})
		return err
	}
	_, err := Bot.SendMessage(context.Message.From.Id, fmt.Sprintf("Firstname: %v\nLastname: %v\nUsername: %v\nUserID: %v", context.Message.From.FirstName, context.Message.From.LastName, context.Message.From.Username, context.Message.From.Id), &gotgbot.SendMessageOpts{})
	return err
}
