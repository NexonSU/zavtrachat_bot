package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Kill user on /kill
func RemoveReplyMessage(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		return KillSender(bot, context)
	}
	_, err := context.Message.Delete(bot, nil)
	if err != nil {
		return err
	}
	_, err = context.Message.ReplyToMessage.Delete(bot, nil)
	return err
}
