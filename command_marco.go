package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Reply "Polo!" on "marco"
func Marco(bot *gotgbot.Bot, context *ext.Context) error {
	return ReplyAndRemoveWithTarget("Polo!", *context)
}
