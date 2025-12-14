package main

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Send Get to user on /get
func SetGetOwner(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		_, err := bot.SendAnimation(context.Message.Chat.Id, gotgbot.InputFileByID("CgACAgQAAx0CQvXPNQABH62yYQHUkpaPOe79NW4ZnwYZWCNJXW8AAgoBAAK-qkVQnRXXGK03dEMgBA"), &gotgbot.SendAnimationOpts{ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.MessageId, AllowSendingWithoutReply: true}})
		return err
	}
	var get Get
	if len(context.Args()) != 2 || context.Message.ReplyToMessage == nil {
		return ReplyAndRemoveWithTarget("Пример использования: <code>/setgetowner {гет}</code> в ответ пользователю, которого нужно задать владельцем.", *context)
	}
	result := DB.Where(&Get{Name: strings.ToLower(context.Args()[1])}).First(&get)
	if result.RowsAffected != 0 {
		get.Creator = context.Message.ReplyToMessage.From.Id
		DB.First(&get)
		if result.Error != nil {
			return result.Error
		}
		return ReplyAndRemoveWithTarget(fmt.Sprintf("Владелец гета <code>%v</code> изменён на %v.", get.Name, MentionUser(context.Message.ReplyToMessage.From)), *context)
	} else {
		return ReplyAndRemoveWithTarget(fmt.Sprintf("Гет <code>%v</code> не найден.", context.Message.Text), *context)
	}
}
