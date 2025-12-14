package main

import (
	"fmt"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Return date on /date
func Date(bot *gotgbot.Bot, context *ext.Context) error {
	return ReplyAndRemoveWithTarget(fmt.Sprintf("%v", time.Now().Local().Format("02.01.2006 03:04")), *context)
}
