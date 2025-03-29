package main

import (
	"fmt"

	tele "gopkg.in/telebot.v3"
)

// Send userid on /getid
func Getid(context tele.Context) error {
	if context.Message().ReplyTo != nil && context.Message().ReplyTo.OriginalSender != nil {
		_, err := Bot.Send(context.Sender(), fmt.Sprintf("Firstname: %v\nLastname: %v\nUsername: %v\nUserID: %v", context.Message().ReplyTo.OriginalSender.FirstName, context.Message().ReplyTo.OriginalSender.LastName, context.Message().ReplyTo.OriginalSender.Username, context.Message().ReplyTo.OriginalSender.ID))
		return err
	}
	if context.Message().ReplyTo != nil {
		_, err := Bot.Send(context.Sender(), fmt.Sprintf("Firstname: %v\nLastname: %v\nUsername: %v\nUserID: %v", context.Message().ReplyTo.Sender.FirstName, context.Message().ReplyTo.Sender.LastName, context.Message().ReplyTo.Sender.Username, context.Message().ReplyTo.Sender.ID))
		return err
	}
	if len(context.Args()) == 1 {
		target, _, err := FindUserInMessage(context)
		if err != nil {
			return err
		}
		_, err = Bot.Send(context.Sender(), fmt.Sprintf("Firstname: %v\nLastname: %v\nUsername: %v\nUserID: %v", target.FirstName, target.LastName, target.Username, target.ID))
		return err
	}
	_, err := Bot.Send(context.Sender(), fmt.Sprintf("Firstname: %v\nLastname: %v\nUsername: %v\nUserID: %v", context.Sender().FirstName, context.Sender().LastName, context.Sender().Username, context.Sender().ID))
	return err
}
