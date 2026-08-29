package main

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Remove user in DB on /pidordel
func Pidordel(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		return KillSender(bot, context)
	}
	var pidor PidorList
	user, err := FindUserInMessage(*context.Message)
	if err != nil {
		return err
	}
	pidor = PidorList(user)
	result := DB.Delete(&pidor)
	if result.RowsAffected != 0 {
		return Reply(fmt.Sprintf("Пользователь %v удалён из игры <b>Пидор Дня</b>!", MentionUser(&user)), *context)
	} else {
		return Reply(fmt.Sprintf("Не удалось удалить пользователя:\n<code>%v</code>", result.Error.Error()), *context)
	}
}
