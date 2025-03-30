package main

import (
	"fmt"
	"html"
	"strconv"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// List all bets
func AllBets(bot *gotgbot.Bot, context *ext.Context) error {
	var betlist string
	var bet Bets
	var user gotgbot.User
	var i = 0
	var from int64
	var to int64
	if len(context.Args()) > 1 {
		if context.Args()[1] == "all" {
			from = 0
		}
	}
	from = time.Now().Local().Truncate(24 * time.Hour).Unix()
	to = time.Now().Local().Add(43800 * time.Hour).Unix()
	result, _ := DB.Model(&Bets{}).Where("timestamp > ? AND timestamp < ?", from, to).Order("timestamp ASC").Rows()
	for result.Next() {
		err := DB.ScanRows(result, &bet)
		if err != nil {
			return err
		}
		i++
		user, err = GetUserFromDB(strconv.FormatInt(bet.UserID, 10))
		if err != nil {
			return err
		}
		betlist += fmt.Sprintf("%v, %v:\n<pre>%v</pre>\n", time.Unix(bet.Timestamp, 0).Format("02.01.2006"), UserFullName(&user), html.EscapeString(bet.Text))
		if len(betlist) > 3900 {
			_, err := context.Message.Reply(bot, betlist, &gotgbot.SendMessageOpts{ParseMode: "HTML"})
			return err
		}
	}
	_, err := context.Message.Reply(bot, betlist, &gotgbot.SendMessageOpts{ParseMode: "HTML"})
	return err
}
