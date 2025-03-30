package main

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func Deny(bot *gotgbot.Bot, context *ext.Context) error {
	victim := context.Message.Entities[0].User
	if victim.Id != context.Message.From.Id {
		_, err := context.Update.CallbackQuery.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{
			Text: GetNope(),
		})
		return err
	}
	_, err := context.Update.CallbackQuery.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{})
	if err != nil {
		return err
	}
	busy["russianroulette"] = false
	busy["russianroulettePending"] = false
	busy["russianrouletteInProgress"] = false
	_, _, err = Bot.EditMessageText(fmt.Sprintf("%v отказался от дуэли.", UserFullName(context.Message.From)), &gotgbot.EditMessageTextOpts{ChatId: context.Message.Chat.Id, MessageId: context.Message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
	return err
}
