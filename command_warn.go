package main

import (
	"fmt"
	"time"

	tele "gopkg.in/telebot.v3"
	"gorm.io/gorm/clause"
)

// Send warning to user on /warn
func WarnUser(context tele.Context) error {
	var warn Warn
	if (context.Message().ReplyTo == nil && len(context.Args()) != 1) || (context.Message().ReplyTo != nil && len(context.Args()) != 0) {
		return ReplyAndRemove("Пример использования: <code>/warn {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>/warn</code>", context)
	}
	target, _, err := FindUserInMessage(context)
	if err != nil {
		return err
	}
	result := DB.First(&warn, target.ID)
	if result.RowsAffected != 0 {
		warn.Amount = warn.Amount - int(time.Since(warn.LastWarn).Hours()/24/7)
		if warn.Amount < 0 {
			warn.Amount = 0
		}
		warn.Amount = warn.Amount + 1
	} else {
		warn.Amount = 1
	}
	warn.UserID = target.ID
	warn.LastWarn = time.Now()
	result = DB.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&warn)
	if result.Error != nil {
		return result.Error
	}
	if warn.Amount == 1 {
		return context.Send(fmt.Sprintf("%v, у тебя 1 предупреждение.\nЕсль получишь 3 предупреждения за 2 недели, то будешь исключен из чата.", UserFullName(&target)))
	}
	if warn.Amount == 2 {
		return context.Send(fmt.Sprintf("%v, у тебя 2 предупреждения.\nЕсли в течении недели получишь ещё одно, то будешь исключен из чата.", UserFullName(&target)))
	}
	if warn.Amount == 3 {
		untildate := time.Now().AddDate(0, 0, 7).Unix()
		TargetChatMember, err := Bot.ChatMemberOf(context.Chat(), &target)
		if err != nil {
			return err
		}
		TargetChatMember.RestrictedUntil = untildate
		err = Bot.Ban(context.Chat(), TargetChatMember)
		if err != nil {
			return err
		}
		return ReplyAndRemove(fmt.Sprintf("Пользователь <a href=\"tg://user?id=%v\">%v</a> забанен%v, т.к. набрал 3 предупреждения.", target.ID, UserFullName(&target), RestrictionTimeMessage(untildate)), context)
	}
	return err
}
