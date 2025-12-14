package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Ban user on /ban
func Ban(bot *gotgbot.Bot, context *ext.Context) error {
	var untildate = time.Now().Unix() + 86400
	if !IsAdminOrModer(context.Message.From.Id) {
		return KillSender(bot, context)
	}
	if (context.Message.ReplyToMessage == nil && len(context.Args()) == 1) || (context.Message.ReplyToMessage != nil && len(context.Args()) > 2) {
		return ReplyAndRemoveWithTarget("Пример использования: <code>/ban {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>/ban</code>\nЕсли нужно забанить на время, то добавь время в секундах через пробел.", *context)
	}
	target, err := FindUserInMessage(*context)
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
	result, err := bot.BanChatMember(context.Message.Chat.Id, target.Id, &gotgbot.BanChatMemberOpts{UntilDate: untildate})
	if err != nil {
		return err
	}
	if result {
		return ReplyAndRemoveWithTarget(fmt.Sprintf("Пользователь <a href=\"tg://user?id=%v\">%v</a> забанен%v.", target.Id, UserFullName(&target), RestrictionTimeMessage(untildate)), *context)
	} else {
		return ReplyAndRemoveWithTarget(fmt.Sprintf("Пользователь <a href=\"tg://user?id=%v\">%v</a> забанен%v.", target.Id, UserFullName(&target), RestrictionTimeMessage(untildate)), *context)
	}
}
