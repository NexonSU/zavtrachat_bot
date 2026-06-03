package main

import (
	"fmt"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Revive users on /redemption
func Redemption(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		return KillSender(bot, context)
	}

	_, err := context.Message.Delete(bot, nil)
	if err != nil {
		return err
	}

	var userID int64
	text := fmt.Sprintf("✨ %v кастует редемпшн!\n\n", MentionUser(context.EffectiveUser))
	rows, err := DB.Model(&Stats{}).Select("context_id").Where("stat_type = 3").Order("last_update desc").Limit(100).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()
	revived := []int64{}
	for rows.Next() {
		rows.Scan(&userID)
		if slices.Contains(revived, userID) {
			continue
		}
		target, err := Bot.GetChatMember(context.EffectiveChat.Id, userID, nil)
		if err != nil {
			continue
		}
		if target.GetStatus() == "restricted" {
			user := target.GetUser()
			_, err = Bot.RestrictChatMember(context.Message.Chat.Id, user.Id, gotgbot.ChatPermissions{CanSendMessages: true}, &gotgbot.RestrictChatMemberOpts{UntilDate: time.Now().Add(time.Second * time.Duration(60)).Unix()})
			if err != nil {
				continue
			}

			revived = append(revived, user.Id)

			if utf8.RuneCountInString(text) < 3600 {
				text += fmt.Sprintf("%v возродился в чате.\n", UserFullName(&user))
			}
		}
	}
	if len(revived) == 0 {
		text += "Но никто не воскрес..."
		_, err = context.EffectiveChat.SendMessage(bot, text, &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, DisableNotification: true})
		return err
	}
	_, err = bot.SendVideo(context.Message.Chat.Id, gotgbot.InputFileByID("CgACAgIAAx0CTSN9dQACGxtqIF4VpuUIWpnabA9lbsBZO4MJqgACuZcAAmsTAAFJTvlY4-Qfso86BA"), &gotgbot.SendVideoOpts{ParseMode: gotgbot.ParseModeHTML, Caption: text})
	return err
}
