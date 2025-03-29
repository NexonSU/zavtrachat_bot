package main

import (
	tele "gopkg.in/telebot.v3"
)

// Send list of Gets to user on /getall
func Getall(context tele.Context) error {
	var getall string
	var get Get
	result, _ := DB.Model(&Get{}).Rows()
	for result.Next() {
		if getall == "" {
			getall = "Доступные геты: "
		} else {
			getall += ", "
		}
		err := DB.ScanRows(result, &get)
		if err != nil {
			return err
		}
		getall += get.Name
		if len([]rune(getall)) > 4000 {
			Bot.Send(context.Sender(), getall)
			getall = ""
		}
	}
	Bot.Send(context.Sender(), getall)
	return ReplyAndRemove("Список отправлен в личку.\nЕсли список не пришел, то убедитесь, что бот запущен и не заблокирован в личке.", context)
}
