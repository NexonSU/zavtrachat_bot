package main

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Add bet
func Bet(bot *gotgbot.Bot, context *ext.Context) error {
	var bet Bets
	if len(context.Args()) < 3 {
		return ReplyAndRemoveWithTarget("Пример использования: <code>/bet 30.06.2023 ставлю жопу, что TESVI будет говном</code>", *context)
	}
	date, err := time.Parse("02.01.2006", context.Args()[1])
	if err != nil {
		return ReplyAndRemoveWithTarget("Ошибка парсинга даты: "+err.Error(), *context)
	}
	if date.Unix() < time.Now().Local().Unix() {
		return ReplyAndRemoveWithTarget(fmt.Sprintf("минимальная дата: %v", time.Now().Local().Add(24*time.Hour).Format("02.01.2006")), *context)
	}
	bet.UserID = context.Message.From.Id
	bet.Timestamp = date.Unix()
	bet.Text = strings.Join(context.Args()[2:], " ")
	result := DB.Create(&bet)
	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "UNIQUE constraint failed") {
			return ReplyAndRemoveWithTarget("Такая ставка уже добавлена", *context)
		}
		return result.Error
	}
	return ReplyAndRemoveWithTarget(fmt.Sprintf("Ставка добавлена.\nДата: <code>%v</code>.\nТекст: <code>%v</code>.", time.Unix(bet.Timestamp, 0).Format("02.01.2006"), html.EscapeString(bet.Text)), *context)
}
