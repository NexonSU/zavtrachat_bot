package main

import (
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// Send warning amount on /mywarns
func Mywarns(bot *gotgbot.Bot, context *ext.Context) error {
	// prt will replace fmt package to format text according plurals defined in utils package
	// If no plural rule matched it will be ignored and processed as usual formatting
	prt := message.NewPrinter(language.Russian)

	var warn Warn
	result := DB.First(&warn, context.Message.From.Id)
	if result.RowsAffected != 0 {
		warn.Amount = warningAmountAt(warn, time.Now())
	} else {
		warn.UserID = context.Message.From.Id
		warn.LastWarn = time.Unix(0, 0)
		warn.Amount = 0
	}
	return Reply(prt.Sprintf("У тебя %d %v.", warn.Amount, ruPlural(warn.Amount, "предупреждение", "предупреждения", "предупреждений")), *context)
}
