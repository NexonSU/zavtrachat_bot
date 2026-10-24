package main

import (
	"errors"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gorm.io/gorm"
)

// Send user Duelist stats on /duelstats
func Duelstats(bot *gotgbot.Bot, context *ext.Context) error {
	// prt will replace fmt package to format text according plurals defined in utils package
	// If no plural rule matched it will be ignored and processed as usual formatting
	prt := message.NewPrinter(language.Russian)

	var duelist Duelist
	result := DB.Model(Duelist{}).Where(context.Message.From.Id).First(&duelist)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}
	var saves int64
	if err := DB.Model(&Stats{}).Select("COALESCE(SUM(count), 0)").Where("stat_type = ? AND context_id = ?", salvationStatType, context.Message.From.Id).Scan(&saves).Error; err != nil {
		return err
	}
	if result.RowsAffected == 0 && saves == 0 {
		return Reply("У тебя нет статистики.", *context)
	}
	winsMessage := prt.Sprintf("%d %v", duelist.Kills, ruPlural(duelist.Kills, "победа", "победы", "побед"))
	deathsMessage := prt.Sprintf("%d %v", duelist.Deaths, ruPlural(duelist.Deaths, "смерть", "смерти", "смертей"))
	savesMessage := prt.Sprintf("%d %v", saves, ruPlural(saves, "спасение", "спасения", "спасений"))
	return Reply(prt.Sprintf("У тебя %s, %s и %s", winsMessage, deathsMessage, savesMessage), *context)
}
