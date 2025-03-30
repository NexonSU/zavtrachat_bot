package main

import (
	"fmt"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm/clause"
)

// Send warning to user on /warn
func WarnUser(bot *gotgbot.Bot, context *ext.Context) error {
	var warn Warn
	if (context.Message.ReplyToMessage == nil && len(context.Args()) != 2) || (context.Message.ReplyToMessage != nil && len(context.Args()) != 1) {
		return ReplyAndRemove("Пример использования: <code>/warn {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>/warn</code>", *context)
	}
	target, _, err := FindUserInMessage(*context)
	if err != nil {
		return err
	}
	result := DB.First(&warn, target.Id)
	if result.RowsAffected != 0 {
		warn.Amount = warn.Amount - int(time.Since(warn.LastWarn).Hours()/24/7)
		if warn.Amount < 0 {
			warn.Amount = 0
		}
		warn.Amount = warn.Amount + 1
	} else {
		warn.Amount = 1
	}
	warn.UserID = target.Id
	warn.LastWarn = time.Now()
	result = DB.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&warn)
	if result.Error != nil {
		return result.Error
	}
	if warn.Amount == 1 {
		_, err := context.EffectiveChat.SendMessage(bot, fmt.Sprintf("%v, у тебя 1 предупреждение.\nЕсль получишь 3 предупреждения за 2 недели, то будешь забанен.", UserFullName(&target)), &gotgbot.SendMessageOpts{ParseMode: "HTML"})
		return err
	}
	if warn.Amount == 2 {
		_, err := context.EffectiveChat.SendMessage(bot, (fmt.Sprintf("%v, у тебя 2 предупреждения.\nЕсли в течении недели получишь ещё одно, то будешь забанен.", UserFullName(&target))), &gotgbot.SendMessageOpts{ParseMode: "HTML"})
		return err
	}
	if warn.Amount == 3 {
		result, err := bot.BanChatMember(context.Message.Chat.Id, target.Id, nil)
		if err != nil {
			return err
		}
		if result {
			return ReplyAndRemove(fmt.Sprintf("Пользователь <a href=\"tg://user?id=%v\">%v</a> забанен, т.к. набрал 3 предупреждения.", target.Id, UserFullName(&target)), *context)
		}
	}
	return err
}
