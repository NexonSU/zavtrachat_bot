package main

import (
	"fmt"

	tele "gopkg.in/telebot.v3"
)

// Remove user in DB on /pidordel
func Pidordel(context tele.Context) error {
	var pidor PidorList
	user, _, err := FindUserInMessage(context)
	if err != nil {
		return err
	}
	pidor = PidorList(user)
	result := DB.Delete(&pidor)
	if result.RowsAffected != 0 {
		return ReplyAndRemove(fmt.Sprintf("Пользователь %v удалён из игры <b>Пидор Дня</b>!", MentionUser(&user)), context)
	} else {
		return ReplyAndRemove(fmt.Sprintf("Не удалось удалить пользователя:\n<code>%v</code>", result.Error.Error()), context)
	}
}
