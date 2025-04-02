package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// Send top 10 pidors of all time on /pidorall
func Pidorall(bot *gotgbot.Bot, context *ext.Context) error {
	// prt will replace fmt package to format text according plurals defined in utils package
	// If no plural rule matched it will be ignored and processed as usual formatting
	prt := message.NewPrinter(language.Russian)

	var i = 0
	var username string
	var count int64
	var pidorall = "Топ-10 пидоров за всё время:\n\n"
	result, err := DB.Select("username, COUNT(*) as count").Table("pidor_stats, pidor_lists").Where("pidor_stats.user_id=pidor_lists.id").Group("user_id").Order("count DESC").Limit(10).Rows()
	if err != nil {
		return err
	}
	defer result.Close()
	for result.Next() {
		err := result.Scan(&username, &count)
		if err != nil {
			return err
		}
		i++
		pidorall += prt.Sprintf("%v. %v - %d раз\n", i, username, count)
	}
	DB.Model(PidorList{}).Count(&count)
	pidorall += prt.Sprintf("\nВсего участников — %v", count)
	_, err = context.Message.Reply(bot, pidorall, &gotgbot.SendMessageOpts{ParseMode: "HTML"})
	return err
}
