package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Remove target message
func RemoveReplyMessage(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		return Denied(bot, context)
	}
	Remove(bot, context)
	_, err := context.Message.ReplyToMessage.Delete(bot, nil)
	return err
}
