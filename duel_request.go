package main

import (
	"fmt"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

var Message *gotgbot.Message
var busy = make(map[string]bool)

func Request(bot *gotgbot.Bot, context *ext.Context) error {
	if context.EffectiveMessage == nil || context.EffectiveUser == nil {
		return nil
	}
	duelMu.Lock()
	defer duelMu.Unlock()
	if busy["bot_is_dead"] {
		if Message == nil || time.Now().Unix()-Message.Date > 3600 {
			busy["bot_is_dead"] = false
		} else {
			return Reply("Я не могу провести игру, т.к. я немного умер. Зайдите позже.", *context)
		}
	}
	if Message != nil && busy["russianroulettePending"] && !busy["russianrouletteInProgress"] && time.Now().Unix()-Message.Date > 60 {
		busy["russianroulette"] = false
		busy["russianroulettePending"] = false
		busy["russianrouletteInProgress"] = false
		name := "Участник"
		if len(Message.Entities) > 0 && Message.Entities[0].User != nil {
			name = UserFullName(Message.Entities[0].User)
		}
		_, _, err := Bot.EditMessageText(&gotgbot.EditMessageTextOpts{Text: fmt.Sprintf("%v не пришел на дуэль.", name), ParseMode: gotgbot.ParseModeHTML, ChatId: Message.Chat.Id, MessageId: Message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		return err
	}
	// Never release a running game by age: its handler still owns the state
	// and may be waiting for Telegram. The completion defer releases it.
	if busy["russianroulette"] || busy["russianroulettePending"] || busy["russianrouletteInProgress"] {
		return Reply("Команда занята. Попробуйте позже.", *context)
	}
	busy["russianroulette"] = true
	defer func() { busy["russianroulette"] = false }()
	if (context.Message.ReplyToMessage == nil && len(context.Args()) != 2) || (context.Message.ReplyToMessage != nil && len(context.Args()) != 1) {
		return Reply("Пример использования: <code>/russianroulette {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>/russianroulette</code>", *context)
	}
	target, err := FindUserInMessage(*context.Message)
	if err != nil {
		return err
	}
	if target.Id == context.Message.From.Id {
		return Reply("Как ты себе это представляешь? Нет, нельзя вызвать на дуэль самого себя.", *context)
	}
	if target.IsBot {
		return Reply("Бота нельзя вызвать на дуэль.", *context)
	}
	// The chat member lookup doubles as a validity check on the target: a user
	// that is not in the chat cannot be called out.
	_, err = bot.GetChatMember(context.Message.Chat.Id, target.Id, nil)
	if err != nil {
		return err
	}
	Remove(bot, context)
	invitation, err := Bot.SendMessage(context.Message.Chat.Id, fmt.Sprintf("%v! %v вызывает тебя на дуэль!", MentionUser(&target), MentionUser(context.Message.From)), &gotgbot.SendMessageOpts{
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
	// Keep the actual invitation, not the deleted command. IDs bind buttons
	// to this game; participants are copied before any game can mutate them.
	player := *context.EffectiveUser
	Message = &gotgbot.Message{MessageId: invitation.MessageId, Chat: context.Message.Chat, Date: time.Now().Unix(),
		Entities: []gotgbot.MessageEntity{{Type: "text_mention", User: &target}, {Type: "text_mention", User: &player}}}
	busy["russianroulettePending"] = true
	return err
}
