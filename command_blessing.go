package main

import (
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"

	tele "gopkg.in/telebot.v3"
	"gorm.io/gorm/clause"
)

var firstSuicide int64
var lastSuicide int64
var burst int
var lastVideoSent int64

// Kill user on /blessing, /suicide
func Blessing(context tele.Context) error {
	// prt will replace fmt package to format text according plurals defined in utils package
	// If no plural rule matched it will be ignored and processed as usual formatting
	prt := message.NewPrinter(language.Russian)

	victim := context.Sender()
	ricochetText := ""

	err := context.Delete()
	if err != nil {
		return err
	}
	ChatMember, err := Bot.ChatMemberOf(context.Chat(), context.Sender())
	if err != nil {
		return err
	}
	if ChatMember.Role == "administrator" || ChatMember.Role == "creator" || context.Sender().ID == 825209730 {
		var ricochetVictim *tele.ChatMember
		var userID int64
		rows, err := DB.Model(&Stats{}).Where("stat_type = 3").Order("last_update desc").Select("context_id").Limit(100).Rows()
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			rows.Scan(&userID)
			ricochetVictim, err = Bot.ChatMemberOf(context.Chat(), &tele.User{ID: userID})
			if err != nil {
				continue
			}
			if ricochetVictim.Role == "member" {
				victim = ricochetVictim.User
				ChatMember = ricochetVictim
				ricochetText = prt.Sprintf("Пуля отскакивает от головы %v и летит в голову %v.\n", MentionUser(context.Sender()), MentionUser(victim))
				rows.Close()
				break
			}
		}
	}
	var duelist Duelist
	result := DB.Model(Duelist{}).Where(context.Sender().ID).First(&duelist)
	if result.RowsAffected == 0 {
		duelist.UserID = context.Sender().ID
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
	if context.Sender().IsPremium {
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
	ChatMember.RestrictedUntil = time.Now().Add(time.Second * time.Duration(60*duration)).Unix()
	err = Bot.Restrict(context.Chat(), ChatMember)
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
		return context.Send(&tele.Video{
			File: tele.File{
				FileID: "BAACAgIAAx0CReJGYgABAlMuYnagTilFaB8ke8Rw-dYLbfJ6iF8AAicYAAIlxrlLY9ah2fUtR40kBA",
			},
			Caption: prt.Sprintf("<code>%v💥 %v %v%v.\nРеспавн через %d мин.</code>", ricochetText, UserFullName(victim), prependText, GetBless(), duration),
		})
	} else {
		return context.Send(prt.Sprintf("<code>%v💥 %v %v%v.\nРеспавн через %d мин.</code>", ricochetText, UserFullName(victim), prependText, GetBless(), duration))
	}
}
