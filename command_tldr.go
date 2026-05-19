package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Send Yandex 300 response on link
func TLDR(bot *gotgbot.Bot, context *ext.Context) error {
	return ReplyAndRemoveWithTarget("Юзайте кнопку TLDR напротив сообщения", *context)
}
