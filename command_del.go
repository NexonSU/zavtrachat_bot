package main

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Delete Get in DB on /del
func Del(bot *gotgbot.Bot, context *ext.Context) error {
	var get Get
	//args check
	if len(context.Args()) == 1 {
		return ReplyAndRemoveWithTarget("Пример использования: <code>/del {гет}</code>", *context)
	}
	_, text, _ := strings.Cut(context.EffectiveMessage.Text, " ")
	//ownership check
	result := DB.Where(&Get{Name: strings.ToLower(text)}).First(&get)
	if result.RowsAffected == 0 {
		return ReplyAndRemoveWithTarget(fmt.Sprintf("Гет <code>%v</code> не найден.", text), *context)
	}
	creator, err := GetUserFromDB(fmt.Sprint(get.Creator))
	if err != nil {
		return err
	}
	if get.Creator != context.Message.From.Id && !IsAdminOrModer(context.Message.From.Id) {
		return ReplyAndRemoveWithTarget(fmt.Sprintf("Данный гет могут изменять либо администраторы, либо %v.", UserFullName(&creator)), *context)
	}
	//removing Get
	result = DB.Delete(&get)
	if result.Error != nil {
		return result.Error
	}
	return ReplyAndRemoveWithTarget(fmt.Sprintf("Гет <code>%v</code> удалён.", text), *context)
}
