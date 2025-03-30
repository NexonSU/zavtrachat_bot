package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Reply "Pong!" on /ping
func Ping(bot *gotgbot.Bot, context *ext.Context) error {
	return ReplyAndRemove("Pong!", *context)
}
