package main

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Send top 10 blessers on /blessingtop
func BlessingTop(bot *gotgbot.Bot, context *ext.Context) error {
	var i = 0
	var userIdStr string
	var count int64
	var text = "Топ-10 исекаев чата:\n\n"
	result, err := DB.Select("user_id, deaths").Table("duelists").Order("deaths DESC").Limit(10).Rows()
	if err != nil {
		return err
	}
	defer result.Close()
	for result.Next() {
		err := result.Scan(&userIdStr, &count)
		if err != nil {
			return err
		}
		user, err := GetUserFromDB(userIdStr)
		if err == nil {
			userIdStr = user.FirstName
			if user.LastName != "" {
				userIdStr += " " + user.LastName
			}
		}
		i++
		text += fmt.Sprintf("%v. %v - %d раз\n", i, userIdStr, count)
	}
	_, err = context.Message.Reply(bot, text, &gotgbot.SendMessageOpts{})
	return err
}
