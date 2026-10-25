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

// Kill user on /kill
func Kill(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		return Denied(bot, context)
	}
	// prt will replace fmt package to format text according plurals defined in utils package
	// If no plural rule matched it will be ignored and processed as usual formatting
	prt := message.NewPrinter(language.Russian)

	command := strings.Split(strings.Split(context.Message.Text, "@")[0], " ")[0]
	if (context.Message.ReplyToMessage == nil && len(context.Args()) != 2) || (context.Message.ReplyToMessage != nil && len(context.Args()) != 1) {
		return Reply(prt.Sprintf("Пример использования: <code>%v {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>%v</code>", command, command), *context)
	}
	target, err := FindUserInMessage(*context.Message)
	if err != nil {
		return err
	}
	ChatMember, err := bot.GetChatMember(context.Message.Chat.Id, target.Id, nil)
	if err != nil {
		return err
	}
	victimText := ""
	if ChatMember.GetStatus() == "administrator" || ChatMember.GetStatus() == "creator" {
		var victim gotgbot.ChatMember
		var userID int64
		rows, err := DB.Model(&Stats{}).Where("stat_type = 3").Order("last_update desc").Select("context_id").Limit(100).Rows()
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			rows.Scan(&userID)
			victim, err = bot.GetChatMember(context.Message.Chat.Id, userID, nil)
			if err != nil {
				continue
			}
			if victim.GetStatus() == "member" {
				ChatMember = victim
				tempUser := victim.GetUser()
				victimText = prt.Sprintf("Пуля отскакивает от головы %v и летит в голову %v.\n", MentionUser(&target), MentionUser(&tempUser))
				target = ChatMember.GetUser()
				rows.Close()
				break
			}
		}
	} else {
		if context.Message.ReplyToMessage != nil {
			Bot.DeleteMessage(context.Message.Chat.Id, context.Message.ReplyToMessage.MessageId, nil)
		}
	}
	// Deaths count game attempts; mute time is recorded only for confirmed restrictions.
	duelist, err := recordGameDeath(target.Id)
	if err != nil {
		return err
	}
	duration := RandInt(1, duelist.Deaths+1)
	duration += 10
	prependText := ""
	if RandInt(0, 100) >= 90 {
		duration = duration * 10
		prependText = "критически "
		if command == "/bless" {
			prependText = "очень "
		}
	}
	if strings.Contains(command, "kilo") {
		duration = duration * 1024
	}
	if strings.Contains(command, "mega") {
		duration = duration * 1048576
	}
	if victimText != "" {
		duration = 1
	}
	muteSource := "kill"
	if victimText != "" {
		muteSource = "ricochet"
	}
	trueVal := true
	restricted, err := restrictChatMemberTracked(Bot, context.Message.Chat.Id, ChatMember.GetUser().Id, gotgbot.ChatPermissions{
		CanSendMessages:    false,
		CanReactToMessages: &trueVal,
	}, &gotgbot.RestrictChatMemberOpts{UntilDate: time.Now().Add(time.Second * time.Duration(60*duration)).Unix()}, context.Message.From, muteSource)
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
	text := prt.Sprintf("💥 %v %vпристрелил %v.\n%v отправился на респавн на %d %v.", html.EscapeString(UserFullName(context.Message.From)), prependText, html.EscapeString(UserFullName(&target)), html.EscapeString(UserFullName(&target)), duration, ruPlural(duration, "минуту", "минуты", "минут"))
	if command == "/bless" {
		text = prt.Sprintf("🤫 %v %vпопросил %v помолчать %d %v.", html.EscapeString(UserFullName(context.Message.From)), prependText, html.EscapeString(UserFullName(&target)), duration, ruPlural(duration, "минуту", "минуты", "минут"))
	}
	if strings.Contains(command, "bite") || strings.Contains(command, "mlem") {
		text = prt.Sprintf("😼 %v %vсделал кусь %v.\n%v отправился на респавн на %d %v.", html.EscapeString(UserFullName(context.Message.From)), prependText, html.EscapeString(UserFullName(&target)), html.EscapeString(UserFullName(&target)), duration, ruPlural(duration, "минуту", "минуты", "минут"))
	}
	if victimText != "" {
		text = prt.Sprintf("💥 %v\n%v отправился на респавн на %d %v.", victimText, html.EscapeString(UserFullName(&target)), duration, ruPlural(duration, "минуту", "минуты", "минут"))
	}
	_, err = context.EffectiveChat.SendMessage(bot, text, &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
	return err
}
