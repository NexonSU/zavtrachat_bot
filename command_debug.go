package main

import (
	"encoding/json"
	"fmt"

	tele "gopkg.in/telebot.v3"
)

// Return message on /debug command
func Debug(context tele.Context) error {
	err := Bot.Delete(context.Message())
	if err != nil {
		return err
	}
	var message = context.Message()
	if context.Message().ReplyTo != nil {
		message = context.Message().ReplyTo
	}
	MarshalledMessage, _ := json.MarshalIndent(message, "", "    ")
	_, err = Bot.Send(context.Sender(), fmt.Sprintf("<pre>%v</pre>", string(MarshalledMessage)))
	return err
}
