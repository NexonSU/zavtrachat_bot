package main

import (
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Pidor game
func PidorRemoveToday(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		return Denied(bot, context)
	}
	var pidor PidorStats
	result := DB.Model(PidorStats{}).Where("date BETWEEN ? AND ?", time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local), time.Now()).First(&pidor)
	if result.RowsAffected == 0 {
		return Reply("Сегодня пидора дня ещё не было.", *context)
	} else {
		DB.Delete(&pidor)
		return Reply("Текущий пидор дня больше не пидор!", *context)
	}
}
