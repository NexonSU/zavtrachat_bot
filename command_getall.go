package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Send list of Gets to user on /getall
func Getall(bot *gotgbot.Bot, context *ext.Context) error {
	var getall string
	var get Get
	result, err := DB.Model(&Get{}).Rows()
	if err != nil {
		return err
	}
	defer result.Close()
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
			Bot.SendMessage(context.Message.From.Id, getall, nil)
			getall = ""
		}
	}
	Bot.SendMessage(context.Message.From.Id, getall, nil)
	return ReplyAndRemove("Список отправлен в личку.\nЕсли список не пришел, то убедитесь, что бот запущен и не заблокирован в личке.", *context)
}
