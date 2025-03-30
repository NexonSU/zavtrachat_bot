package main

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Send formatted text on /me
func Me(bot *gotgbot.Bot, context *ext.Context) error {
	if len(context.Args()) == 1 {
		return ReplyAndRemove("Пример использования:\n<code>/me {делает что-то}</code>", *context)
	}
	Remove(bot, context)
	_, err := context.EffectiveChat.SendMessage(bot, (fmt.Sprintf("<code>%v %v</code>", strings.Replace(UserFullName(context.Message.From), "💥", "", -1), context.Message.Text)), &gotgbot.SendMessageOpts{ParseMode: "HTML"})
	return err
}
