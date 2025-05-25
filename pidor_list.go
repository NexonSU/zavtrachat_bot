package main

import (
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// List add pidors from DB on /pidorlist
func Pidorlist(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		_, err := bot.SendAnimation(context.Message.Chat.Id, gotgbot.InputFileByID("CgACAgQAAx0CQvXPNQABH62yYQHUkpaPOe79NW4ZnwYZWCNJXW8AAgoBAAK-qkVQnRXXGK03dEMgBA"), &gotgbot.SendAnimationOpts{ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.MessageId, AllowSendingWithoutReply: true}})
		return err
	}
	var pidorlist string
	var pidor PidorList
	var i = 0
	var err error
	result, err := DB.Model(&PidorList{}).Rows()
	if err != nil {
		return err
	}
	defer result.Close()
	for result.Next() {
		err := DB.ScanRows(result, &pidor)
		if err != nil {
			return err
		}
		i++
		pidorlist += strconv.Itoa(i) + ". @" + pidor.Username + " (" + strconv.FormatInt(pidor.Id, 10) + ")\n"
		if len(pidorlist) > 3900 {
			_, err = Bot.SendMessage(context.Message.From.Id, pidorlist, &gotgbot.SendMessageOpts{})
			if err != nil {
				return err
			}
			pidorlist = ""
		}
	}
	_, err = Bot.SendMessage(context.Message.From.Id, pidorlist, &gotgbot.SendMessageOpts{})
	if err != nil {
		return err
	}
	return ReplyAndRemove("Список отправлен в личку.\nЕсли список не пришел, то убедитесь, что бот запущен и не заблокирован в личке.", *context)
}
