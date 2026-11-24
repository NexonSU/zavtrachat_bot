package main

import (
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func inChatChannelCatalog(ctx *ext.Context) bool {
	chat, user, msg := ctx.EffectiveChat, ctx.EffectiveUser, ctx.EffectiveMessage
	return chat != nil && chat.Id == Config.Chat && (chat.Type == "group" || chat.Type == "supergroup") &&
		user != nil && !user.IsBot && msg != nil && msg.SenderChat == nil
}

func replyChatChannels(bot *gotgbot.Bot, ctx *ext.Context, text string) error {
	opts := &gotgbot.SendMessageOpts{
		EphemeralMessageParameters: &gotgbot.EphemeralMessageParameters{ReceiverUserId: ctx.EffectiveUser.Id},
		LinkPreviewOptions:         &gotgbot.LinkPreviewOptions{IsDisabled: true},
	}
	for _, part := range splitTelegramText(text) {
		if _, err := bot.SendMessage(ctx.EffectiveChat.Id, strings.TrimSpace(part), opts); err != nil {
			return err
		}
	}
	return nil
}

func ChatChannels(bot *gotgbot.Bot, ctx *ext.Context) error {
	if !inChatChannelCatalog(ctx) {
		return nil
	}
	text, err := chatChannelList(&DB)
	if err != nil {
		return err
	}
	return replyChatChannels(bot, ctx, text)
}

func AddChatChannel(bot *gotgbot.Bot, ctx *ext.Context) error {
	if !inChatChannelCatalog(ctx) {
		return nil
	}
	args := ctx.Args()
	if len(args) != 3 {
		return replyChatChannels(bot, ctx, "Формат: /addchatchannel @свой_ник https://t.me/канал")
	}
	user := ctx.EffectiveUser
	if user.Username == "" || !strings.EqualFold(strings.TrimPrefix(args[1], "@"), user.Username) {
		return replyChatChannels(bot, ctx, "Укажи свой текущий ник в Telegram.")
	}
	channel, ok := parseChatChannel(args[2])
	if !ok {
		return replyChatChannels(bot, ctx, "Укажи ссылку на канал Telegram или инвайт-ссылку t.me.")
	}
	added, err := addOwnChatChannel(&DB, user, channel)
	if err != nil {
		return err
	}
	if !added {
		return replyChatChannels(bot, ctx, "Этот канал уже есть в каталоге.")
	}
	return replyChatChannels(bot, ctx, "Канал добавлен.")
}

func DelMyChatChannel(bot *gotgbot.Bot, ctx *ext.Context) error {
	if !inChatChannelCatalog(ctx) {
		return nil
	}
	args := ctx.Args()
	if len(args) > 2 {
		return replyChatChannels(bot, ctx, "Формат: /delmychatchannel [ссылка]")
	}
	var key string
	if len(args) == 2 {
		channel, ok := parseChatChannel(args[1])
		if !ok {
			return replyChatChannels(bot, ctx, "Укажи ссылку на свой канал Telegram.")
		}
		key = channel.Key
	}
	deleted, choices, err := deleteOwnChatChannel(&DB, ctx.EffectiveUser, key)
	if err != nil {
		return err
	}
	if len(choices) != 0 {
		return replyChatChannels(bot, ctx, "У тебя несколько каналов. Укажи нужную ссылку после /delmychatchannel.\n\n"+strings.Join(choices, "\n"))
	}
	if !deleted {
		return replyChatChannels(bot, ctx, "Твой канал не найден в каталоге.")
	}
	return replyChatChannels(bot, ctx, "Канал удалён.")
}

func DelChatChannel(bot *gotgbot.Bot, ctx *ext.Context) error {
	if !inChatChannelCatalog(ctx) {
		return nil
	}
	userID := ctx.EffectiveUser.Id
	if !IsAdmin(userID) && userID != Config.SysAdmin && !isTelegramChatAdmin(bot, userID) {
		return replyChatChannels(bot, ctx, "Удалять любой канал могут только администраторы.")
	}
	args := ctx.Args()
	if len(args) != 2 {
		return replyChatChannels(bot, ctx, "Формат: /delchatchannel ссылка")
	}
	channel, ok := parseChatChannel(args[1])
	if !ok {
		return replyChatChannels(bot, ctx, "Укажи ссылку на канал Telegram.")
	}
	result := DB.Where("key = ?", channel.Key).Delete(&ChatChannel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return replyChatChannels(bot, ctx, "Канал не найден в каталоге.")
	}
	return replyChatChannels(bot, ctx, "Канал удалён.")
}
