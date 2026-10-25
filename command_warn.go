package main

import (
	"fmt"
	"html"
	"log"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Send warning to user on /warn
func WarnUser(bot *gotgbot.Bot, context *ext.Context) error {
	return warnUserAt(bot, context, time.Now())
}

func warnUserAt(bot *gotgbot.Bot, context *ext.Context, now time.Time) error {
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
	lock := &warningLocks[uint64(target.Id)%uint64(len(warningLocks))]
	lock.Lock()
	defer lock.Unlock()
	result := DB.First(&warn, target.Id)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return result.Error
	}
	previous := warn
	oldAmount := warningAmountAt(warn, now)
	if oldAmount >= 3 {
		return Reply("У участника уже 3 предупреждения. Повторное ограничение не требуется.", *context)
	}
	warn.Amount = min(3, oldAmount+1)
	warn.UserID = target.Id
	warn.LastWarn = now
	result = DB.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&warn)
	if result.Error != nil {
		return result.Error
	}
	if warn.Amount == 1 {
		if _, err := context.EffectiveChat.SendMessage(bot, fmt.Sprintf("%v, у тебя 1 предупреждение.\nЕсли получишь 3 предупреждения за 2 недели, то будешь замьючен.", html.EscapeString(UserFullName(&target))), &gotgbot.SendMessageOpts{EphemeralMessageParameters: &gotgbot.EphemeralMessageParameters{ReceiverUserId: target.Id}, ParseMode: gotgbot.ParseModeHTML}); err != nil {
			log.Printf("warn: failed to notify %v about the warning: %v", target.Id, err)
		}
		return Reply(fmt.Sprintf("%v получил предупреждение. Сейчас у него 1 из 3.", html.EscapeString(UserFullName(&target))), *context)
	}
	if warn.Amount == 2 {
		if _, err := context.EffectiveChat.SendMessage(bot, (fmt.Sprintf("%v, у тебя 2 предупреждения.\nЕсли в течение недели получишь ещё одно, то будешь замьючен.", html.EscapeString(UserFullName(&target)))), &gotgbot.SendMessageOpts{EphemeralMessageParameters: &gotgbot.EphemeralMessageParameters{ReceiverUserId: target.Id}, ParseMode: gotgbot.ParseModeHTML}); err != nil {
			log.Printf("warn: failed to notify %v about the warning: %v", target.Id, err)
		}
		return Reply(fmt.Sprintf("%v получил предупреждение. Сейчас у него 2 из 3.", html.EscapeString(UserFullName(&target))), *context)
	}
	if warn.Amount == 3 && oldAmount < 3 {
		trueVal := true
		if _, err := context.EffectiveChat.SendMessage(bot, fmt.Sprintf("%v, ты набрал 3 предупреждения.", html.EscapeString(UserFullName(&target))), &gotgbot.SendMessageOpts{EphemeralMessageParameters: &gotgbot.EphemeralMessageParameters{ReceiverUserId: target.Id}, ParseMode: gotgbot.ParseModeHTML}); err != nil {
			log.Printf("warn: failed to notify %v about the mute: %v", target.Id, err)
		}
		result, err := restrictChatMemberTracked(Bot, context.Message.Chat.Id, target.Id, gotgbot.ChatPermissions{
			CanSendMessages:    false,
			CanReactToMessages: &trueVal,
		}, &gotgbot.RestrictChatMemberOpts{UntilDate: time.Now().Add(time.Hour * time.Duration(168)).Unix()}, context.Message.From, "warn")
		if err != nil || !result {
			// A rejected restriction may be retried by a new warning command.
			rollback := DB.Model(&Warn{}).Where("user_id = ?", target.Id).Updates(map[string]any{"amount": previous.Amount, "last_warn": previous.LastWarn}).Error
			if rollback != nil {
				return fmt.Errorf("warning rollback: %v; restriction: %v", rollback, err)
			}
			if err != nil {
				return err
			}
			return fmt.Errorf("Telegram rejected warning restriction")
		}
		if result {
			// Mute time (stat type 7) accumulates the restriction in seconds.
			if err := statsAdd(7, GetStartOfDay(), target.Id, 168*3600); err != nil {
				return err
			}
			return Reply(fmt.Sprintf("Пользователь <a href=\"tg://user?id=%v\">%v</a> замьючен, т.к. набрал 3 предупреждения.", target.Id, html.EscapeString(UserFullName(&target))), *context)
		}
	}
	return err
}
