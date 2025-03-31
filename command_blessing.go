package main

import (
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"gorm.io/gorm/clause"
)

var firstSuicide int64
var lastSuicide int64
var burst int
var lastVideoSent int64

// Kill user on /blessing, /suicide
func Blessing(bot *gotgbot.Bot, context *ext.Context) error {
	// prt will replace fmt package to format text according plurals defined in utils package
	// If no plural rule matched it will be ignored and processed as usual formatting
	prt := message.NewPrinter(language.Russian)

	victim := context.Message.From
	ricochetText := ""

	_, err := context.Message.Delete(bot, nil)
	if err != nil {
		return err
	}
	ChatMember, err := Bot.GetChatMember(context.Message.Chat.Id, context.Message.From.Id, nil)
	if err != nil {
		return err
	}
	if ChatMember.GetStatus() == "administrator" || ChatMember.GetStatus() == "creator" || context.Message.From.Id == Config.SysAdmin {
		var userID int64
		rows, err := DB.Model(&Stats{}).Select("context_id").Where("stat_type = 3").Order("last_update desc").Limit(100).Rows()
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			rows.Scan(&userID)
			ricochetVictim, err := Bot.GetChatMember(context.Message.Chat.Id, userID, nil)
			if err != nil {
				continue
			}
			if ricochetVictim.GetStatus() == "member" {
				*victim = ricochetVictim.GetUser()
				ChatMember = ricochetVictim
				ricochetText = prt.Sprintf("Пуля отскакивает от головы %v и летит в голову %v.\n", MentionUser(context.Message.From), MentionUser(victim))
				rows.Close()
				break
			}
		}
	}
	var duelist Duelist
	result := DB.Model(Duelist{}).Where(context.Message.From.Id).First(&duelist)
	if result.RowsAffected == 0 {
		duelist.UserID = context.Message.From.Id
		duelist.Kills = 0
		duelist.Deaths = 0
	}
	duelist.Deaths++
	result = DB.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&duelist)
	if result.Error != nil {
		return result.Error
	}
	duration := RandInt(1, duelist.Deaths+1)
	duration += 10
	prependText := ""
	additionalChance := int(time.Now().Unix() - lastSuicide)
	if additionalChance > 3600 {
		additionalChance = 3600
	}
	additionalChance = (3600 - additionalChance) / 360
	if context.Message.From.IsPremium {
		duration = duration * 2
		prependText += "премиально "
	}
	if RandInt(0, 100) >= 90-additionalChance {
		duration = duration * 10
		prependText += "критически "
	}
	if duration >= 1400 && duration <= 1500 {
		duration = 1488
	}
	if ricochetText != "" {
		duration = 1
	}
	_, err = Bot.RestrictChatMember(context.Message.Chat.Id, ChatMember.GetUser().Id, gotgbot.ChatPermissions{CanSendMessages: false}, &gotgbot.RestrictChatMemberOpts{UntilDate: time.Now().Add(time.Second * time.Duration(60*duration)).Unix()})
	if err != nil {
		return err
	}
	burst++
	if time.Now().Unix() > firstSuicide+120 {
		firstSuicide = time.Now().Unix()
		burst = 1
	}
	lastSuicide = time.Now().Unix()
	if burst > 3 && time.Now().Unix() > lastVideoSent+3600 {
		lastVideoSent = time.Now().Unix()
		_, err = bot.SendVideo(context.Message.Chat.Id, gotgbot.InputFileByID("BAACAgIAAx0CReJGYgABAlMuYnagTilFaB8ke8Rw-dYLbfJ6iF8AAicYAAIlxrlLY9ah2fUtR40kBA"), &gotgbot.SendVideoOpts{Caption: prt.Sprintf("<code>%v💥 %v %v%v.\nРеспавн через %d мин.</code>", ricochetText, UserFullName(victim), prependText, GetBless(), duration)})
	} else {
		_, err = context.EffectiveChat.SendMessage(bot, prt.Sprintf("<code>%v💥 %v %v%v.\nРеспавн через %d мин.</code>", ricochetText, UserFullName(victim), prependText, GetBless(), duration), &gotgbot.SendMessageOpts{ParseMode: "HTML"})
	}
	return err
}
