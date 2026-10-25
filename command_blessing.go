package main

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var firstSuicide int64
var lastSuicide int64
var burst int
var lastVideoSent int64

// Self-blessing handler shared by /blessing, /suicide and /isekai.
func Blessing(bot *gotgbot.Bot, context *ext.Context) error {
	return blessingWithSalvationRoll(bot, context, func() int { return RandInt(0, 99) })
}

func blessingWithSalvationRoll(bot *gotgbot.Bot, context *ext.Context, roll func() int) error {
	// prt will replace fmt package to format text according plurals defined in utils package
	// If no plural rule matched it will be ignored and processed as usual formatting
	prt := message.NewPrinter(language.Russian)

	victim := *context.Message.From
	ricochetText := ""

	Remove(bot, context)
	ChatMember, err := Bot.GetChatMember(context.Message.Chat.Id, context.Message.From.Id, nil)
	if err != nil {
		return err
	}
	isAdmin := ChatMember.GetStatus() == "administrator" || ChatMember.GetStatus() == "creator" || context.Message.From.Id == Config.SysAdmin
	if isAdmin {
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
				victim = ricochetVictim.GetUser()
				ChatMember = ricochetVictim
				ricochetText = prt.Sprintf("Пуля отскакивает от головы %v и летит в голову %v.\n", MentionUser(context.Message.From), MentionUser(&victim))
				rows.Close()
				break
			}
		}
	}
	// Admin-triggered ricochets never roll for salvation. Resolve salvation
	// before touching deaths, mute time or the suicide/video burst state.
	if !isAdmin && roll() < 33 {
		var phrase Salvation
		result := DB.Order("RANDOM()").Limit(1).Find(&phrase)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 && strings.TrimSpace(phrase.Text) != "" {
			if err := statsAdd(salvationStatType, GetStartOfDay(), victim.Id, 1); err != nil {
				return err
			}
			text := strings.TrimSpace(phrase.Text)
			if !strings.HasSuffix(text, ".") && !strings.HasSuffix(text, "!") && !strings.HasSuffix(text, "?") && !strings.HasSuffix(text, "…") {
				text += "."
			}
			_, err := bot.SendMessage(context.EffectiveChat.Id, fmt.Sprintf("%s %s\nЖиви. Пока что.", UserFullName(&victim), text), nil)
			return err
		}
		// An empty pool keeps the existing blessing behavior.
	}
	// The death belongs to whoever ends up restricted: on a ricochet that is
	// the random member picked above, not the admin/sysadmin who pressed the
	// command (same as Kill, which reassigns target to the ricochet victim).
	// Deaths count game attempts; mute time is recorded only for confirmed restrictions.
	duelist, err := recordGameDeath(victim.Id)
	if err != nil {
		return err
	}
	duration := RandInt(1, duelist.Deaths+1)
	duration += 10
	prependText := ""
	blessingStateMu.Lock()
	additionalChance := int(time.Now().Unix() - lastSuicide)
	if additionalChance > 3600 {
		additionalChance = 3600
	}
	additionalChance = (3600 - additionalChance) / 360
	blessingStateMu.Unlock()
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
	trueVal := true
	restricted, err := restrictChatMemberTracked(Bot, context.Message.Chat.Id, ChatMember.GetUser().Id, gotgbot.ChatPermissions{
		CanSendMessages:    false,
		CanReactToMessages: &trueVal,
	}, &gotgbot.RestrictChatMemberOpts{UntilDate: time.Now().Add(time.Second * time.Duration(60*duration)).Unix()}, context.Message.From, "blessing")
	if err != nil {
		return err
	}
	if !restricted {
		return fmt.Errorf("Telegram rejected game restriction")
	}
	// Mute time (stat type 7) accumulates the restriction in seconds.
	if err := statsAdd(7, GetStartOfDay(), ChatMember.GetUser().Id, int64(duration)*60); err != nil {
		return err
	}
	blessingStateMu.Lock()
	burst++
	if time.Now().Unix() > firstSuicide+120 {
		firstSuicide = time.Now().Unix()
		burst = 1
	}
	lastSuicide = time.Now().Unix()
	sendVideo := burst > 3 && time.Now().Unix() > lastVideoSent+3600
	if sendVideo {
		lastVideoSent = time.Now().Unix()
	}
	blessingStateMu.Unlock()
	if sendVideo {
		_, err = bot.SendVideo(context.Message.Chat.Id, gotgbot.InputFileByID("BAACAgIAAx0CReJGYgABAlMuYnagTilFaB8ke8Rw-dYLbfJ6iF8AAicYAAIlxrlLY9ah2fUtR40kBA"), &gotgbot.SendVideoOpts{ParseMode: gotgbot.ParseModeHTML, Caption: prt.Sprintf("<code>%v💥 %v %v%v.\nРеспавн через %d %v.</code>", ricochetText, html.EscapeString(UserFullName(&victim)), prependText, GetBless(), duration, ruPlural(duration, "минуту", "минуты", "минут"))})
	} else {
		_, err = context.EffectiveChat.SendMessage(bot, prt.Sprintf("<code>%v💥 %v %v%v.\nРеспавн через %d %v.</code>", ricochetText, html.EscapeString(UserFullName(&victim)), prependText, GetBless(), duration, ruPlural(duration, "минуту", "минуты", "минут")), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
	}
	return err
}
