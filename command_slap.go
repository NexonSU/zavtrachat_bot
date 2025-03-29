package main

import (
	"fmt"

	tele "gopkg.in/telebot.v3"
)

// Send slap message on /slap
func Slap(context tele.Context) error {
	var action = "дал леща"
	var target tele.User
	if IsAdminOrModer(context.Sender().ID) {
		action = "дал отцовского леща"
	}
	target, _, err := FindUserInMessage(context)
	if err != nil {
		return err
	}
	return context.Send(fmt.Sprintf("👋 <b>%v</b> %v %v", UserFullName(context.Sender()), action, MentionUser(&target)))
}
