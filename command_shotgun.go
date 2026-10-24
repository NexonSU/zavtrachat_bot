package main

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"gorm.io/gorm/clause"
)

// Kill users on /shotgun and its variants, including /megamlem.
func Shotgun(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.EffectiveUser.Id) {
		return Denied(bot, context)
	}

	Remove(bot, context)

	var userID int64
	command := strings.Split(context.Args()[0], "@")[0]
	isMegamlem := command == "/megamlem"
	header := ""
	text := ""
	if isMegamlem {
		header = "Неизвестно откуда появившийся гигантский чёрный кот совершает <b>МЕГАМЛЕМ!</b>\n\n"
	} else if strings.Contains(context.Args()[0], "gigabite") {
		header = fmt.Sprintf("😼 %v сделал кусь чату!\n\n", MentionUser(context.EffectiveUser))
	} else {
		header = fmt.Sprintf("💥 %v выстрелил по чату из шотгана!\n\n", MentionUser(context.EffectiveUser))
	}
	rows, err := DB.Model(&Stats{}).Select("context_id").Where("stat_type = 3").Order("last_update desc").Limit(1000).Rows()
	if err != nil {
		return err
	}
	// The victims are collected first and the cursor is closed before any
	// stats row is written: the mute-time counter (statsAdd) touches the same
	// table the cursor reads, which a live database does not like mid-scan.
	var victims []gotgbot.User
	for rows.Next() {
		if len(victims) >= 10 {
			break
		}
		rows.Scan(&userID)
		ricochetVictim, err := Bot.GetChatMember(context.EffectiveChat.Id, userID, nil)
		if err != nil {
			continue
		}
		if ricochetVictim.GetStatus() == "member" {
			victims = append(victims, ricochetVictim.GetUser())
		}
	}
	rows.Close()
	for _, victim := range victims {
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
		// Deaths are duplicated into the stats (stat type 8) so /stats tops
		// can rank them per day.
		statsAdd(8, GetStartOfDay(), victim.Id, 1)
		duration := 1
		prependText := ""
		if victim.IsPremium {
			duration = duration * 2
			prependText += "премиально "
		}
		muteSource := "shotgun"
		if isMegamlem {
			muteSource = "megamlem"
		}
		trueVal := true
		_, err = restrictChatMemberTracked(Bot, context.Message.Chat.Id, victim.Id, gotgbot.ChatPermissions{
			CanSendMessages:    false,
			CanReactToMessages: &trueVal,
		}, &gotgbot.RestrictChatMemberOpts{UntilDate: time.Now().Add(time.Second * time.Duration(60*duration)).Unix()}, context.Message.From, muteSource)
		if err != nil {
			continue
		}
		// Mute time (stat type 7) accumulates the restriction in seconds.
		statsAdd(7, GetStartOfDay(), victim.Id, int64(duration)*60)

		text += fmt.Sprintf("%v %v%v.\n", UserFullName(&victim), prependText, GetBless())
	}
	text += "\nРеспавн через пару минут."
	if isMegamlem {
		caption := header + text
		// Long names or blessing texts may exceed Telegram's photo caption limit.
		// Count UTF-16 units including markup conservatively, and keep the full
		// result in a separate message when it does not fit under the picture.
		separateResult := len(utf16.Encode([]rune(caption))) > 1024
		if separateResult {
			caption = strings.TrimSpace(header)
		}
		_, err = bot.SendPhoto(context.Message.Chat.Id, gotgbot.InputFileByReader("megamlem.jpg", bytes.NewReader(megamlemJPG)), &gotgbot.SendPhotoOpts{
			Caption: caption, ParseMode: gotgbot.ParseModeHTML, DisableNotification: true,
		})
		if err != nil || !separateResult {
			return err
		}
	} else {
		text = header + text
	}
	_, err = context.EffectiveChat.SendMessage(bot, text, &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, DisableNotification: true})
	return err
}
