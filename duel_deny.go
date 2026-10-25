package main

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func Deny(bot *gotgbot.Bot, context *ext.Context) error {
	_, _, reason := claimDuel(context)
	if reason != "" {
		return answerDuelRefusal(bot, context, reason)
	}
	defer finishDuel()
	_, err := context.Update.CallbackQuery.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{})
	if err != nil {
		return err
	}
	_, _, err = Bot.EditMessageText(&gotgbot.EditMessageTextOpts{Text: fmt.Sprintf("%v отказался от дуэли.", UserFullName(context.EffectiveSender.User)), ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: context.EffectiveMessage.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
	return err
}
