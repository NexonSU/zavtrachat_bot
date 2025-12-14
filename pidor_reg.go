package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm/clause"
)

// Send DB result on /pidoreg
func Pidoreg(bot *gotgbot.Bot, context *ext.Context) error {
	var pidor PidorList
	if DB.First(&pidor, context.Message.From.Id).RowsAffected != 0 {
		return ReplyAndRemoveWithTarget("Эй, ты уже в игре!", *context)
	} else {
		pidor = PidorList(*context.Message.From)
		result := DB.Clauses(clause.OnConflict{
			UpdateAll: true,
		}).Create(&pidor)
		if result.Error != nil {
			return result.Error
		}
		return ReplyAndRemoveWithTarget("OK! Ты теперь участвуешь в игре <b>Пидор Дня</b>!", *context)
	}
}
