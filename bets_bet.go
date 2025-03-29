package main

import (
	"fmt"
	"html"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"
)

// Add bet
func Bet(context tele.Context) error {
	var bet Bets
	if len(context.Args()) < 2 {
		return ReplyAndRemove("Пример использования: <code>/bet 30.06.2023 ставлю жопу, что TESVI будет говном</code>", context)
	}
	date, err := time.Parse("02.01.2006", context.Args()[0])
	if err != nil {
		return ReplyAndRemove("Ошибка парсинга даты: "+err.Error(), context)
	}
	if date.Unix() < time.Now().Local().Unix() {
		return ReplyAndRemove(fmt.Sprintf("минимальная дата: %v", time.Now().Local().Add(24*time.Hour).Format("02.01.2006")), context)
	}
	bet.UserID = context.Sender().ID
	bet.Timestamp = date.Unix()
	bet.Text = strings.Join(context.Args()[1:], " ")
	result := DB.Create(&bet)
	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "UNIQUE constraint failed") {
			return ReplyAndRemove("Такая ставка уже добавлена", context)
		}
		return result.Error
	}
	return ReplyAndRemove(fmt.Sprintf("Ставка добавлена.\nДата: <code>%v</code>.\nТекст: <code>%v</code>.", time.Unix(bet.Timestamp, 0).Format("02.01.2006"), html.EscapeString(bet.Text)), context)
}
