package main

import (
	"fmt"
	"log"
	"time"

	tele "gopkg.in/telebot.v3"
)

var Message *tele.Message
var Selector = tele.ReplyMarkup{}
var AcceptButton = Selector.Data("👍 Принять вызов", "russianroulette_accept")
var DenyButton = Selector.Data("👎 Бежать с позором", "russianroulette_deny")
var busy = make(map[string]bool)

func Request(context tele.Context) error {
	if Message == nil {
		Message = context.Message()
		Message.Unixtime = 0
	}
	if busy["bot_is_dead"] {
		if time.Now().Unix()-Message.Time().Unix() > 3600 {
			busy["bot_is_dead"] = false
		} else {
			return ReplyAndRemove("Я не могу провести игру, т.к. я немного умер. Зайдите позже.", context)
		}
	}
	if busy["russianroulettePending"] && !busy["russianrouletteInProgress"] && time.Now().Unix()-Message.Time().Unix() > 60 {
		busy["russianroulette"] = false
		busy["russianroulettePending"] = false
		busy["russianrouletteInProgress"] = false
		return context.Edit(fmt.Sprintf("%v не пришел на дуэль.", UserFullName(Message.Entities[0].User)))
	}
	if busy["russianrouletteInProgress"] && time.Now().Unix()-Message.Time().Unix() > 120 {
		busy["russianroulette"] = false
		busy["russianroulettePending"] = false
		busy["russianrouletteInProgress"] = false
	}
	if busy["russianroulette"] || busy["russianroulettePending"] || busy["russianrouletteInProgress"] {
		return ReplyAndRemove("Команда занята. Попробуйте позже.", context)
	}
	busy["russianroulette"] = true
	defer func() { busy["russianroulette"] = false }()
	if (context.Message().ReplyTo == nil && len(context.Args()) != 1) || (context.Message().ReplyTo != nil && len(context.Args()) != 0) {
		return ReplyAndRemove("Пример использования: <code>/russianroulette {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>/russianroulette</code>", context)
	}
	target, _, err := FindUserInMessage(context)
	if err != nil {
		return err
	}
	if target.ID == context.Sender().ID {
		return ReplyAndRemove("Как ты себе это представляешь? Нет, нельзя вызвать на дуэль самого себя.", context)
	}
	if target.IsBot {
		return ReplyAndRemove("Бота нельзя вызвать на дуэль.", context)
	}
	ChatMember, err := Bot.ChatMemberOf(context.Chat(), &target)
	if err != nil {
		return err
	}
	log.Println(ChatMember)
	if false {
		err := context.Reply("Нельзя вызвать на дуэль мертвеца.")
		if err != nil {
			return err
		}
		return err
	}
	err = Bot.Delete(context.Message())
	if err != nil {
		return err
	}
	Selector.Inline(
		Selector.Row(AcceptButton, DenyButton),
	)
	Message, err = Bot.Send(context.Chat(), fmt.Sprintf("%v! %v вызывает тебя на дуэль!", MentionUser(&target), MentionUser(context.Sender())), &Selector)
	if err != nil {
		return err
	}
	busy["russianroulettePending"] = true
	return err
}
