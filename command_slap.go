package main

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Send slap message on /slap
func Slap(bot *gotgbot.Bot, context *ext.Context) error {
	var action = "дал леща"
	var target gotgbot.User
	if IsAdminOrModer(context.Message.From.Id) {
		action = "дал отцовского леща"
	}
	target, err := FindUserInMessage(*context.Message)
	if err != nil {
		return err
	}
	_, err = context.EffectiveChat.SendMessage(bot, (fmt.Sprintf("👋 <b>%v</b> %v %v", UserFullName(context.Message.From), action, MentionUser(&target))), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
	return err
}
