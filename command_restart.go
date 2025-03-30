package main

import (
	"os"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Restart bot on /restart
func Restart(bot *gotgbot.Bot, context *ext.Context) error {
	Remove(bot, context)
	os.Exit(0)
	return nil
}
