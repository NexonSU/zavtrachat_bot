package main

import (
	"strconv"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"

	tele "gopkg.in/telebot.v3"
)

// Send top 10 pidors of year on /pidorstats
func Pidorstats(context tele.Context) error {
	// prt will replace fmt package to format text according plurals defined in utils package
	// If no plural rule matched it will be ignored and processed as usual formatting
	prt := message.NewPrinter(language.Russian)

	var i = 0
	var year = time.Now().Year()
	var username string
	var count int64
	if len(context.Args()) == 1 {
		argYear, err := strconv.Atoi(context.Data())
		if err != nil {
			return err
		}
		if argYear == 2077 {
			return context.Reply(&tele.Video{File: tele.File{FileID: "BAACAgIAAx0CRXO-MQADWWB4LQABzrOqWPkq-JXIi4TIixY4dwACPw4AArBgwUt5sRu-_fDR5x4E"}})
		}
		if argYear < year && argYear > 2018 {
			year = argYear
		}
	}
	var pidorall = "Топ-10 пидоров за " + strconv.Itoa(year) + " год:\n\n"
	result, _ := DB.Select("username, COUNT(*) as count").Table("pidor_stats, pidor_lists").Where("pidor_stats.user_id=pidor_lists.id").Where("date BETWEEN ? AND ?", time.Date(year, 1, 1, 0, 0, 0, 0, time.Local), time.Date(year+1, 1, 1, 0, 0, 0, 0, time.Local)).Group("user_id").Order("count DESC").Limit(10).Rows()
	for result.Next() {
		err := result.Scan(&username, &count)
		if err != nil {
			return err
		}
		i++
		pidorall += prt.Sprintf("%v. %v - %d раз\n", i, username, count)
	}
	DB.Model(PidorList{}).Count(&count)
	pidorall += prt.Sprintf("\nВсего участников — %d", count)
	return context.Reply(pidorall)
}
