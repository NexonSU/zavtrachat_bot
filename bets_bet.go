package main

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm/clause"
)

// Add bet
func Bet(bot *gotgbot.Bot, context *ext.Context) error {
	return betAt(bot, context, time.Now())
}

func betAt(bot *gotgbot.Bot, context *ext.Context, now time.Time) error {
	var bet Bets
	if len(context.Args()) < 3 {
		return Reply("Пример использования: <code>/bet 30.06.2023 ставлю жопу, что TESVI будет говном</code>", *context)
	}
	date, err := time.Parse("02.01.2006", context.Args()[1])
	if err != nil {
		return Reply("Ошибка парсинга даты: "+err.Error(), *context)
	}
	if date.Unix() <= betCalendarDay(now).Unix() {
		return Reply(fmt.Sprintf("минимальная дата: %v", now.In(betMoscow).AddDate(0, 0, 1).Format("02.01.2006")), *context)
	}
	bet.UserID = context.Message.From.Id
	bet.Timestamp = date.Unix()
	bet.Text = strings.Join(context.Args()[2:], " ")
	// Leave space for the author/date and HTML wrapper in the public reminder.
	if betTextUnits(bet.Text) > 3500 {
		return Reply("Текст ставки слишком длинный: максимум 3500 символов (эмодзи могут занимать два).", *context)
	}
	bet.CreatedTimestamp = context.Message.Date
	if bet.CreatedTimestamp <= 0 {
		bet.CreatedTimestamp = now.Unix()
	}
	bet.AuthorName = betUserLabel(context.Message.From)
	result := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&bet)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return Reply("Такая ставка уже добавлена", *context)
	}
	return Reply(fmt.Sprintf("Ставка добавлена.\nДата: <code>%v</code>.\nТекст: <code>%v</code>.\nЗаписана: %v МСК.\nВ день ставки напомню в чате в 12:00 МСК.", date.Format("02.01.2006"), html.EscapeString(bet.Text), time.Unix(bet.CreatedTimestamp, 0).In(betMoscow).Format("02.01.2006 15:04")), *context)
}
