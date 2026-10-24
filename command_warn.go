package main

import (
	"fmt"
	"log"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm/clause"
)

// Send warning to user on /warn
func WarnUser(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		return Denied(bot, context)
	}
	var warn Warn
	if (context.Message.ReplyToMessage == nil && len(context.Args()) != 2) || (context.Message.ReplyToMessage != nil && len(context.Args()) != 1) {
		return Reply("Пример использования: <code>/warn {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>/warn</code>", *context)
	}
	target, err := FindUserInMessage(*context.Message)
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
		if _, err := context.EffectiveChat.SendMessage(bot, fmt.Sprintf("%v, у тебя 1 предупреждение.\nЕсли получишь 3 предупреждения за 2 недели, то будешь замьючен.", UserFullName(&target)), &gotgbot.SendMessageOpts{EphemeralMessageParameters: &gotgbot.EphemeralMessageParameters{ReceiverUserId: target.Id}, ParseMode: gotgbot.ParseModeHTML}); err != nil {
			log.Printf("warn: failed to notify %v about the warning: %v", target.Id, err)
		}
		return Reply(fmt.Sprintf("%v получил предупреждение. Сейчас у него 1 из 3.", UserFullName(&target)), *context)
	}
	if warn.Amount == 2 {
		if _, err := context.EffectiveChat.SendMessage(bot, (fmt.Sprintf("%v, у тебя 2 предупреждения.\nЕсли в течение недели получишь ещё одно, то будешь замьючен.", UserFullName(&target))), &gotgbot.SendMessageOpts{EphemeralMessageParameters: &gotgbot.EphemeralMessageParameters{ReceiverUserId: target.Id}, ParseMode: gotgbot.ParseModeHTML}); err != nil {
			log.Printf("warn: failed to notify %v about the warning: %v", target.Id, err)
		}
		return Reply(fmt.Sprintf("%v получил предупреждение. Сейчас у него 2 из 3.", UserFullName(&target)), *context)
	}
	if warn.Amount == 3 {
		trueVal := true
		if _, err := context.EffectiveChat.SendMessage(bot, fmt.Sprintf("%v, ты набрал 3 предупреждения.", UserFullName(&target)), &gotgbot.SendMessageOpts{EphemeralMessageParameters: &gotgbot.EphemeralMessageParameters{ReceiverUserId: target.Id}, ParseMode: gotgbot.ParseModeHTML}); err != nil {
			log.Printf("warn: failed to notify %v about the mute: %v", target.Id, err)
		}
		result, err := restrictChatMemberTracked(Bot, context.Message.Chat.Id, target.Id, gotgbot.ChatPermissions{
			CanSendMessages:    false,
			CanReactToMessages: &trueVal,
		}, &gotgbot.RestrictChatMemberOpts{UntilDate: time.Now().Add(time.Hour * time.Duration(168)).Unix()}, context.Message.From, "warn")
		if err != nil {
			return err
		}
		if result {
			// Mute time (stat type 7) accumulates the restriction in seconds.
			statsAdd(7, GetStartOfDay(), target.Id, 168*3600)
			return Reply(fmt.Sprintf("Пользователь <a href=\"tg://user?id=%v\">%v</a> замьючен, т.к. набрал 3 предупреждения.", target.Id, UserFullName(&target)), *context)
		}
	}
	return err
}
