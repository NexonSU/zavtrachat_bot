package main

import (
	"gorm.io/gorm"
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
	start, end := dayBounds(time.Now())
	result := DB.Model(PidorStats{}).Where("date >= ? AND date < ?", start, end).First(&pidor)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return Reply("Сегодня пидора дня ещё не было.", *context)
	} else {
		if err := DB.Delete(&pidor).Error; err != nil {
			return err
		}
		return Reply("Текущий пидор дня больше не пидор!", *context)
	}
}
