package main

import (
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gorm.io/gorm/clause"
)

// Kill user on /kill
func Kill(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		return KillSender(bot, context)
	}
	// prt will replace fmt package to format text according plurals defined in utils package
	// If no plural rule matched it will be ignored and processed as usual formatting
	prt := message.NewPrinter(language.Russian)

	command := strings.Split(strings.Split(context.Message.Text, "@")[0], " ")[0]
	if (context.Message.ReplyToMessage == nil && len(context.Args()) != 2) || (context.Message.ReplyToMessage != nil && len(context.Args()) != 1) {
		return ReplyAndRemoveWithTarget(prt.Sprintf("Пример использования: <code>%v {ID или никнейм}</code>\nИли отправь в ответ на какое-либо сообщение <code>%v</code>", command, command), *context)
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
	var duelist Duelist
	result := DB.Model(Duelist{}).Where(target.Id).First(&duelist)
	if result.RowsAffected == 0 {
		duelist.UserID = target.Id
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
	_, err = Bot.RestrictChatMember(context.Message.Chat.Id, ChatMember.GetUser().Id, gotgbot.ChatPermissions{CanSendMessages: false}, &gotgbot.RestrictChatMemberOpts{UntilDate: time.Now().Add(time.Second * time.Duration(60*duration)).Unix()})
	if err != nil {
		return err
	}
	text := prt.Sprintf("💥 %v %vпристрелил %v.\n%v отправился на респавн на %d мин.", UserFullName(context.Message.From), prependText, UserFullName(&target), UserFullName(&target), duration)
	if command == "/bless" {
		text = prt.Sprintf("🤫 %v %vпопросил %v помолчать %d минут.", UserFullName(context.Message.From), prependText, UserFullName(&target), duration)
	}
	if strings.Contains(command, "bite") {
		text = prt.Sprintf("😼 %v %vсделал кусь %v.\n%v отправился на респавн на %d мин.", UserFullName(context.Message.From), prependText, UserFullName(&target), UserFullName(&target), duration)
	}
	if victimText != "" {
		text = prt.Sprintf("💥 %v\n%v отправился на респавн на %d мин.", victimText, UserFullName(&target), duration)
	}
	_, err = context.EffectiveChat.SendMessage(bot, text, &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
	return err
}
