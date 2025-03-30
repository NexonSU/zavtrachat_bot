package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Reply with stats link
func StatsLinks(bot *gotgbot.Bot, context *ext.Context) error {
	return ReplyAndRemove("<a href='https://t.me/zavtrachat_bot/stats'>Webapp</a>\n<a href='https://grafana.nexon.su/d/aef7a25c-3824-4046-8ed3-53ccb5850c9d/zavtrachat?kiosk'>Прямая ссылка</a>", *context)
}
