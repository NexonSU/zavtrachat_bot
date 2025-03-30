package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// Send user Duelist stats on /duelstats
func Duelstats(bot *gotgbot.Bot, context *ext.Context) error {
	// prt will replace fmt package to format text according plurals defined in utils package
	// If no plural rule matched it will be ignored and processed as usual formatting
	prt := message.NewPrinter(language.Russian)

	var duelist Duelist
	result := DB.Model(Duelist{}).Where(context.Message.From.Id).First(&duelist)
	if result.RowsAffected == 0 {
		return ReplyAndRemove("У тебя нет статистики.", *context)
	}
	winsMessage := prt.Sprintf("%d побед", duelist.Kills)
	deathsMessage := prt.Sprintf("%d смертей", duelist.Deaths)
	return ReplyAndRemove(prt.Sprintf("У тебя %s и %s", winsMessage, deathsMessage), *context)
}
