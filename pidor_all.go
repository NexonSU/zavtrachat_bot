package main

import (
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	tele "gopkg.in/telebot.v3"
)

// Send top 10 pidors of all time on /pidorall
func Pidorall(context tele.Context) error {
	// prt will replace fmt package to format text according plurals defined in utils package
	// If no plural rule matched it will be ignored and processed as usual formatting
	prt := message.NewPrinter(language.Russian)

	var i = 0
	var username string
	var count int64
	var pidorall = "Топ-10 пидоров за всё время:\n\n"
	result, _ := DB.Select("username, COUNT(*) as count").Table("pidor_stats, pidor_lists").Where("pidor_stats.user_id=pidor_lists.id").Group("user_id").Order("count DESC").Limit(10).Rows()
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
	return context.Reply(pidorall)
}
