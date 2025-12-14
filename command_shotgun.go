package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"gorm.io/gorm/clause"
)

// Kill users on /shotgun, /gigakill /gigabite
func Shotgun(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.EffectiveUser.Id) {
		_, err := bot.SendAnimation(context.Message.Chat.Id, gotgbot.InputFileByID("CgACAgQAAx0CQvXPNQABH62yYQHUkpaPOe79NW4ZnwYZWCNJXW8AAgoBAAK-qkVQnRXXGK03dEMgBA"), &gotgbot.SendAnimationOpts{ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.MessageId, AllowSendingWithoutReply: true}})
		return err
	}

	_, err := context.Message.Delete(bot, nil)
	if err != nil {
		return err
	}

	var userID int64
	text := ""
	if strings.Contains(context.Args()[0], "gigabite") {
		text += fmt.Sprintf("😼 %v сделал кусь чату!\n\n", MentionUser(context.EffectiveUser))
	} else {
		text += fmt.Sprintf("💥 %v выстрелил по чату из шотгана!\n\n", MentionUser(context.EffectiveUser))
	}
	rows, err := DB.Model(&Stats{}).Select("context_id").Where("stat_type = 3").Order("last_update desc").Limit(1000).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		rows.Scan(&userID)
		ricochetVictim, err := Bot.GetChatMember(context.EffectiveChat.Id, userID, nil)
		if err != nil {
			continue
		}
		if ricochetVictim.GetStatus() == "member" {
			victim := ricochetVictim.GetUser()
			var duelist Duelist
			result := DB.Model(Duelist{}).Where(victim.Id).First(&duelist)
			if result.RowsAffected == 0 {
				duelist.UserID = victim.Id
				duelist.Kills = 0
				duelist.Deaths = 0
			}
			duelist.Deaths++
			DB.Clauses(clause.OnConflict{
				UpdateAll: true,
			}).Create(&duelist)
			duration := RandInt(2, 5)
			prependText := ""
			if context.Message.From.IsPremium {
				duration = duration * 2
				prependText += "премиально "
			}
			_, err = Bot.RestrictChatMember(context.Message.Chat.Id, victim.Id, gotgbot.ChatPermissions{CanSendMessages: false}, &gotgbot.RestrictChatMemberOpts{UntilDate: time.Now().Add(time.Second * time.Duration(60*duration)).Unix()})
			if err != nil {
				continue
			}

			text += fmt.Sprintf("%v %v%v. Респавн через %d мин.\n", UserFullName(&victim), prependText, GetBless(), duration)
		}
	}
	_, err = context.EffectiveChat.SendMessage(bot, text, &gotgbot.SendMessageOpts{DisableNotification: true})
	return err
}
