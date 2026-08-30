package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Send shrug in chat on /shrug
func Shrug(bot *gotgbot.Bot, context *ext.Context) error {
	return Reply(("<code>¯\\_(ツ)_/¯</code>"), *context)
}
