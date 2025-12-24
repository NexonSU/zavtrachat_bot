package main

import (
	"encoding/json"
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Return user on /debuguser command
func DebugUser(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		return KillSender(bot, context)
	}
	err := Remove(bot, context)
	if err != nil {
		return err
	}
	user, err := FindUserInMessage(*context.Message)
	if err != nil {
		return err
	}
	cm, err := bot.GetChatMember(context.EffectiveChat.Id, user.Id, nil)
	if err != nil {
		return err
	}
	marsh, err := json.MarshalIndent(cm.MergeChatMember(), "", "    ")
	if err != nil {
		return err
	}
	_, err = Bot.SendMessage(context.Message.From.Id, fmt.Sprintf("<pre>%v</pre>", string(marsh)), &gotgbot.SendMessageOpts{})
	return err
}
