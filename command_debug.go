package main

import (
	"encoding/json"
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Return message on /debug command
func Debug(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		_, err := bot.SendAnimation(context.Message.Chat.Id, gotgbot.InputFileByID("CgACAgQAAx0CQvXPNQABH62yYQHUkpaPOe79NW4ZnwYZWCNJXW8AAgoBAAK-qkVQnRXXGK03dEMgBA"), &gotgbot.SendAnimationOpts{ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.MessageId, AllowSendingWithoutReply: true}})
		return err
	}
	err := Remove(bot, context)
	if err != nil {
		return err
	}
	var message = context.Message
	if context.Message.ReplyToMessage != nil {
		message = context.Message.ReplyToMessage
	}
	MarshalledMessage, _ := json.MarshalIndent(message, "", "    ")
	_, err = Bot.SendMessage(context.Message.From.Id, fmt.Sprintf("<pre>%v</pre>", string(MarshalledMessage)), &gotgbot.SendMessageOpts{})
	return err
}
