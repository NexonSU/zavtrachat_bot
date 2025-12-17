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
	var untildate = time.Now().Unix() + 86400
	if !IsAdminOrModer(context.Message.From.Id) {
		return KillSender(bot, context)
	}
	if (context.Message.ReplyToMessage == nil && len(context.Args()) == 1) || (context.Message.ReplyToMessage != nil && len(context.Args()) > 2) {
		return ReplyAndRemoveWithTarget("Пример использования: <code>/mute {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>/mute</code>\nЕсли нужно замьютить на время, то добавь время в секундах через пробел.", *context)
	}
	target, err := FindUserInMessage(*context.Message)
	for _, arg := range context.Args() {
		addtime, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			continue
		}
		untildate = time.Now().Unix() + addtime
		break
	}
	if err != nil {
		return err
	}
	_, err = Bot.RestrictChatMember(context.Message.Chat.Id, target.Id, gotgbot.ChatPermissions{CanSendMessages: false}, &gotgbot.RestrictChatMemberOpts{UntilDate: untildate})
	if err != nil {
		return err
	}
	return ReplyAndRemoveWithTarget(fmt.Sprintf("Пользователь <a href=\"tg://user?id=%v\">%v</a> больше не может отправлять сообщения%v.", target.Id, UserFullName(&target), RestrictionTimeMessage(untildate)), *context)
}
