package main

import (
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Resend post on user request
func SaveToPM(bot *gotgbot.Bot, context *ext.Context) error {
	if context.EffectiveMessage == nil || context.EffectiveMessage.ReplyToMessage == nil {
		return Reply("Пример использования:\n/topm в ответ на какое-либо сообщение\nБот должен быть запущен и разблокирован в личке.", *context)
	}
	link := fmt.Sprintf("https://t.me/c/%v/%v", strings.TrimLeft(strings.TrimLeft(strconv.Itoa(int(context.EffectiveChat.Id)), "-1"), "0"), context.EffectiveMessage.ReplyToMessage.MessageId)
	var err error
	var msg *gotgbot.MessageId
	var msgIds []int64
	if groupID := context.EffectiveMessage.ReplyToMessage.MediaGroupId; groupID != "" {
		ChatAlbumCache.mu.Lock()
		msgIds = slices.Clone(ChatAlbumCache.groups[groupID])
		ChatAlbumCache.mu.Unlock()
	}
	if len(msgIds) > 0 {
		// Sort only this request's snapshot. Incoming album messages and other
		// saves can keep using the cache while Telegram copies the messages.
		slices.Sort(msgIds)
		msgs, err := Bot.CopyMessages(context.EffectiveSender.User.Id, context.EffectiveChat.Id, msgIds, nil)
		if err != nil {
			return err
		}
		if len(msgs) == 0 {
			return fmt.Errorf("не удалось сохранить альбом: Telegram не скопировал ни одного сообщения")
		}
		msg = &msgs[0]
	} else {
		msg, err = Bot.CopyMessage(context.EffectiveSender.User.Id, context.EffectiveChat.Id, context.EffectiveMessage.ReplyToMessage.MessageId, nil)
		if err != nil {
			return err
		}
	}
	if _, err := Bot.SendMessage(context.EffectiveSender.User.Id, link, &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{MessageId: msg.MessageId, AllowSendingWithoutReply: true}}); err != nil {
		// The copy is already in the PM; only the link is missing, so the
		// command keeps its successful answer.
		log.Printf("topm: the copy reached %v, the link did not: %v", context.EffectiveSender.User.Id, err)
	}
	return Remove(bot, context)
}
