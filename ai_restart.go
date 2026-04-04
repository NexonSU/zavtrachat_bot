package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Reinit AI on /restartai
func RestartAI(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		return KillSender(bot, context)
	}

	err := AiInit()

	if err == nil {
		return ReplyAndRemoveWithTarget("Агент перезапущен", *context)
	} else {
		return err
	}
}
