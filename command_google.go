package main

import (
	"fmt"
	"net/url"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Reply google URL on "google"
func Google(bot *gotgbot.Bot, context *ext.Context) error {
	if len(context.Args()) == 1 {
		return ReplyAndRemove("Пример использования:\n<code>/google {запрос}</code>", *context)
	}
	_, err := context.EffectiveChat.SendMessage(bot, fmt.Sprintf("https://www.google.com/search?q=%v", url.QueryEscape(context.Message.Text)), &gotgbot.SendMessageOpts{ParseMode: "HTML", LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true}, ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.ReplyToMessage.MessageId, AllowSendingWithoutReply: true}})
	return err
}
