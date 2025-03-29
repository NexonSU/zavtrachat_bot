package main

import (
	"fmt"
	"strings"

	tele "gopkg.in/telebot.v3"
)

// Delete Get in DB on /del
func Del(context tele.Context) error {
	var get Get
	//args check
	if len(context.Args()) == 0 {
		return ReplyAndRemove("Пример использования: <code>/del {гет}</code>", context)
	}
	//ownership check
	result := DB.Where(&Get{Name: strings.ToLower(context.Data())}).First(&get)
	if result.RowsAffected == 0 {
		return ReplyAndRemove(fmt.Sprintf("Гет <code>%v</code> не найден.", context.Data()), context)
	}
	creator, err := GetUserFromDB(fmt.Sprint(get.Creator))
	if err != nil {
		return err
	}
	if get.Creator != context.Sender().ID && !IsAdminOrModer(context.Sender().ID) {
		return ReplyAndRemove(fmt.Sprintf("Данный гет могут изменять либо администраторы, либо %v.", UserFullName(&creator)), context)
	}
	//removing Get
	result = DB.Delete(&get)
	if result.Error != nil {
		return result.Error
	}
	return ReplyAndRemove(fmt.Sprintf("Гет <code>%v</code> удалён.", context.Data()), context)
}
