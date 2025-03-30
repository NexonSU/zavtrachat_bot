package main

import (
	"encoding/json"
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Return message on /debug command
func Debug(bot *gotgbot.Bot, context *ext.Context) error {
	err := Remove(bot, context)
	if err != nil {
		return err
	}
	var message = context.Message
	if context.Message.ReplyToMessage != nil {
		message = context.Message.ReplyToMessage
	}
	MarshalledMessage, _ := json.MarshalIndent(message, "", "    ")
	_, err = Bot.SendMessage(context.Message.From.Id, fmt.Sprintf("<pre>%v</pre>", string(MarshalledMessage)), &gotgbot.SendMessageOpts{ParseMode: "HTML"})
	return err
}
