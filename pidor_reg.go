package main

import (
	tele "gopkg.in/telebot.v3"
	"gorm.io/gorm/clause"
)

// Send DB result on /pidoreg
func Pidoreg(context tele.Context) error {
	var pidor PidorList
	if DB.First(&pidor, context.Sender().ID).RowsAffected != 0 {
		return ReplyAndRemove("Эй, ты уже в игре!", context)
	} else {
		pidor = PidorList(*context.Sender())
		result := DB.Clauses(clause.OnConflict{
			UpdateAll: true,
		}).Create(&pidor)
		if result.Error != nil {
			return result.Error
		}
		return ReplyAndRemove("OK! Ты теперь участвуешь в игре <b>Пидор Дня</b>!", context)
	}
}
