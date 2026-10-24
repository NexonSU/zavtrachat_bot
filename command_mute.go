package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Mute user on /mute
func Mute(bot *gotgbot.Bot, context *ext.Context) error {
	var duration int64 = 86400
	if !IsAdminOrModer(context.Message.From.Id) {
		return Denied(bot, context)
	}
	if (context.Message.ReplyToMessage == nil && len(context.Args()) == 1) || (context.Message.ReplyToMessage != nil && len(context.Args()) > 2) {
		return Reply("Пример использования: <code>/mute {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>/mute</code>\nЕсли нужно замьютить на время, то добавь время в секундах через пробел.", *context)
	}
	target, err := FindUserInMessage(*context.Message)
	// The duration may only come after the target: when replying, the target is
	// the replied-to message (so args[1] can be the time); when using arguments,
	// args[1] is the target lookup and args[2] the time.
	timeArg := 1
	if context.Message.ReplyToMessage == nil {
		timeArg = 2
	}
	if timeArg < len(context.Args()) {
		if addtime, parseErr := strconv.ParseInt(context.Args()[timeArg], 10, 64); parseErr == nil {
			duration = addtime
		}
	}
	if err != nil {
		return err
	}
	untildate := time.Now().Unix() + duration
	trueVal := true
	_, err = restrictChatMemberTracked(Bot, context.Message.Chat.Id, target.Id, gotgbot.ChatPermissions{
		CanSendMessages:    false,
		CanReactToMessages: &trueVal,
	}, &gotgbot.RestrictChatMemberOpts{UntilDate: untildate}, context.Message.From, "mute")
	if err != nil {
		return err
	}
	// Mute time (stat type 7) accumulates the restriction in seconds.
	if duration > 0 {
		statsAdd(7, GetStartOfDay(), target.Id, duration)
	}
	return Reply(fmt.Sprintf("Пользователь <a href=\"tg://user?id=%v\">%v</a> больше не может отправлять сообщения%v.", target.Id, UserFullName(&target), RestrictionTimeMessage(untildate)), *context)
}
