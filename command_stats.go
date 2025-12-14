package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Reply with stats link
func StatsLinks(bot *gotgbot.Bot, context *ext.Context) error {
	return ReplyAndRemoveWithTarget("<a href='https://t.me/zavtrachat_bot/stats'>Webapp</a>\n<a href='https://zavtrabot.nexon.su/grafana/d/zavtrachatstats?kiosk'>Прямая ссылка</a>", *context)
}
