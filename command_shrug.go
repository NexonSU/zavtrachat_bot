package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Send shrug in chat on /shrug
func Shrug(bot *gotgbot.Bot, context *ext.Context) error {
	_, err := context.EffectiveChat.SendMessage(bot, ("¯\\_(ツ)_/¯"), &gotgbot.SendMessageOpts{ParseMode: "HTML"})
	return err
}
