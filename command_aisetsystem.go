package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Change system on /aisetsystem
func AISetSystem(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		return KillSender(bot, context)
	}

	system := strings.Join(slices.Delete(context.Args(), 0, 1), " ")

	AISystem = Config.OllamaSystem + "\n" + system

	return ReplyAndRemoveWithTarget(fmt.Sprintf("Системный промпт изменен на:\n%s", system), *context)
}
