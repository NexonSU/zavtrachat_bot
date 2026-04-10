package main

import (
	"fmt"
	"log"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

var Message *gotgbot.Message
var busy = make(map[string]bool)

func Request(bot *gotgbot.Bot, context *ext.Context) error {
	if Message == nil {
		Message = context.Message
		Message.Date = 0
	}
	if busy["bot_is_dead"] {
		if time.Now().Unix()-Message.Date > 3600 {
			busy["bot_is_dead"] = false
		} else {
			return ReplyAndRemoveWithTarget("Я не могу провести игру, т.к. я немного умер. Зайдите позже.", *context)
		}
	}
	if busy["russianroulettePending"] && !busy["russianrouletteInProgress"] && time.Now().Unix()-Message.Date > 60 {
		busy["russianroulette"] = false
		busy["russianroulettePending"] = false
		busy["russianrouletteInProgress"] = false
		_, _, err := Bot.EditMessageText(fmt.Sprintf("%v не пришел на дуэль.", UserFullName(Message.Entities[0].User)), &gotgbot.EditMessageTextOpts{ParseMode: gotgbot.ParseModeHTML, ChatId: context.Message.Chat.Id, MessageId: context.Message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		return err
	}
	if busy["russianrouletteInProgress"] && time.Now().Unix()-Message.Date > 120 {
		busy["russianroulette"] = false
		busy["russianroulettePending"] = false
		busy["russianrouletteInProgress"] = false
	}
	if busy["russianroulette"] || busy["russianroulettePending"] || busy["russianrouletteInProgress"] {
		return ReplyAndRemoveWithTarget("Команда занята. Попробуйте позже.", *context)
	}
	busy["russianroulette"] = true
	defer func() { busy["russianroulette"] = false }()
	if (context.Message.ReplyToMessage == nil && len(context.Args()) != 2) || (context.Message.ReplyToMessage != nil && len(context.Args()) != 1) {
		return ReplyAndRemoveWithTarget("Пример использования: <code>/russianroulette {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>/russianroulette</code>", *context)
	}
	target, err := FindUserInMessage(*context.Message)
	if err != nil {
		return err
	}
	if target.Id == context.Message.From.Id {
		return ReplyAndRemoveWithTarget("Как ты себе это представляешь? Нет, нельзя вызвать на дуэль самого себя.", *context)
	}
	if target.IsBot {
		return ReplyAndRemoveWithTarget("Бота нельзя вызвать на дуэль.", *context)
	}
	ChatMember, err := bot.GetChatMember(context.Message.Chat.Id, target.Id, nil)
	if err != nil {
		return err
	}
	log.Println(ChatMember)
	if false {
		_, err := context.Message.Reply(bot, "Нельзя вызвать на дуэль мертвеца.", &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
		if err != nil {
			return err
		}
		return err
	}
	_, err = context.Message.Delete(bot, nil)
	if err != nil {
		return err
	}
	_, err = Bot.SendMessage(context.Message.Chat.Id, fmt.Sprintf("%v! %v вызывает тебя на дуэль!", MentionUser(&target), MentionUser(context.Message.From)), &gotgbot.SendMessageOpts{
		ParseMode: gotgbot.ParseModeHTML,
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{
				{Text: "👍 Принять вызов", CallbackData: "russianroulette_accept"},
				{Text: "👎 Бежать с позором", CallbackData: "russianroulette_deny"},
			}},
		},
	})
	if err != nil {
		return err
	}
	busy["russianroulettePending"] = true
	return err
}
