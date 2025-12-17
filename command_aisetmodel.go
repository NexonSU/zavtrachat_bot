package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Change model on /aisetmodel
func AISetModel(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		return KillSender(bot, context)
	}

	model := strings.Join(slices.Delete(context.Args(), 0, 1), " ")

	AIModel = model

	return ReplyAndRemoveWithTarget(fmt.Sprintf("Модель изменена на %s", model), *context)
}
