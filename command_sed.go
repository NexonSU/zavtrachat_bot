package main

import (
	"html"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Sed replaces literal text in the replied message.
func Sed(bot *gotgbot.Bot, context *ext.Context) error {
	foo, bar, ok := sedPattern(context.Message.Text)
	if context.Message.ReplyToMessage == nil || !ok {
		return Reply("Пример использования:\n/sed {патерн вида s/foo/bar/} в ответ на сообщение.", *context)
	}
	_, err := context.Message.Reply(bot, html.EscapeString(strings.ReplaceAll(context.Message.ReplyToMessage.Text, foo, bar)), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
	return err
}

func sedPattern(text string) (string, string, bool) {
	args := strings.Fields(text)
	if len(args) != 2 {
		return "", "", false
	}
	// Keep the historical /x/foo/bar y spelling as well as /sed s/foo/bar/.
	pattern := args[1]
	if strings.HasPrefix(args[0], "/sed") {
		pattern = args[1]
	} else {
		pattern = args[0]
	}
	parts := strings.Split(pattern, "/")
	if len(parts) < 3 || parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	if strings.HasPrefix(args[0], "/sed") && (len(parts) != 4 || parts[0] != "s" || parts[3] != "") {
		return "", "", false
	}
	return parts[1], parts[2], true
}
