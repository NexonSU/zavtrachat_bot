package main

import (
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Only one invitation/game is active, as before. Claiming it is atomic;
// the running game's Telegram calls and animation do not hold this lock.
var duelMu sync.Mutex

const duelUnavailable = "Этот вызов уже завершён или устарел. Вызови на дуэль заново."
const duelWrongUser = "wrong participant"

func claimDuel(ctx *ext.Context) (player, victim gotgbot.User, reason string) {
	duelMu.Lock()
	defer duelMu.Unlock()
	msg := ctx.EffectiveMessage
	if ctx.CallbackQuery == nil || ctx.EffectiveUser == nil || msg == nil || Message == nil ||
		!busy["russianroulettePending"] || busy["russianrouletteInProgress"] ||
		msg.Chat.Id != Message.Chat.Id || msg.MessageId != Message.MessageId ||
		len(Message.Entities) < 2 || Message.Entities[0].User == nil || Message.Entities[1].User == nil {
		return player, victim, duelUnavailable
	}
	if time.Now().Unix()-Message.Date > 60 {
		busy["russianroulettePending"] = false
		return player, victim, duelUnavailable
	}
	if ctx.EffectiveUser.Id != Message.Entities[0].User.Id {
		return player, victim, duelWrongUser
	}
	player, victim = *Message.Entities[1].User, *Message.Entities[0].User
	busy["russianroulettePending"] = false
	busy["russianroulette"] = false
	busy["russianrouletteInProgress"] = true
	return player, victim, ""
}

func finishDuel() {
	duelMu.Lock()
	defer duelMu.Unlock()
	busy["russianroulette"] = false
	busy["russianroulettePending"] = false
	busy["russianrouletteInProgress"] = false
}

func answerDuelRefusal(bot *gotgbot.Bot, ctx *ext.Context, reason string) error {
	if ctx.CallbackQuery == nil {
		return nil
	}
	if reason == duelWrongUser {
		reason = GetNope()
	}
	_, err := ctx.CallbackQuery.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: reason})
	return err
}
