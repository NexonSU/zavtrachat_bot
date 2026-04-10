package main

import (
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"golang.org/x/text/language"
	plurals "golang.org/x/text/message"
	"gorm.io/gorm/clause"
)

func Accept(bot *gotgbot.Bot, context *ext.Context) error {
	// prt will replace fmt package to format text according plurals defined in utils package
	// If no plural rule matched it will be ignored and processed as usual formatting
	prt := plurals.NewPrinter(language.Russian)

	message := context.EffectiveMessage
	victim := message.Entities[0].User
	if victim.Id != context.EffectiveSender.User.Id {
		_, err := context.Update.CallbackQuery.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{
			Text: GetNope(),
		})
		return err
	}
	_, err := context.Update.CallbackQuery.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{})
	if err != nil {
		return err
	}
	player := message.Entities[1].User
	busy["russianroulette"] = false
	busy["russianroulettePending"] = false
	busy["russianrouletteInProgress"] = true
	defer func() { busy["russianrouletteInProgress"] = false }()
	success := []string{"%v остаётся в живых. Хм... может порох отсырел?", "В воздухе повисла тишина. %v остаётся в живых.", "%v сегодня заново родился.", "%v остаётся в живых. Хм... я ведь зарядил его?", "%v остаётся в живых. Прикольно, а давай проверим на ком-нибудь другом?"}
	invincible := []string{"пуля отскочила от головы %v и улетела в другой чат.", "%v похмурил брови и отклеил расплющенную пулю со своей головы.", "но ничего не произошло. %v взглянул на револьвер, он был неисправен.", "пуля прошла навылет, но не оставила каких-либо следов на %v."}
	fail := []string{"мозги %v разлетелись по чату!", "%v упал со стула и его кровь растеклась по месседжу.", "%v замер и спустя секунду упал на стол.", "пуля едва не задела кого-то из участников чата! А? Что? А, %v мёртв, да.", "и в воздухе повисла тишина. Все начали оглядываться, когда %v уже был мёртв."}
	prefix := prt.Sprintf("Дуэль! %v против %v!\n", MentionUser(player), MentionUser(victim))
	_, _, err = Bot.EditMessageText(prt.Sprintf("%vЗаряжаю один патрон в револьвер и прокручиваю барабан.", prefix), &gotgbot.EditMessageTextOpts{ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
	if err != nil {
		return err
	}
	time.Sleep(time.Second * 2)
	_, _, err = Bot.EditMessageText(prt.Sprintf("%vКладу револьвер на стол и раскручиваю его.", prefix), &gotgbot.EditMessageTextOpts{ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
	if err != nil {
		return err
	}
	time.Sleep(time.Second * 2)
	if RandInt(1, 360)%2 == 0 {
		player, victim = victim, player
	}
	_, _, err = Bot.EditMessageText(prt.Sprintf("%vРевольвер останавливается на %v, первый ход за ним.", prefix, MentionUser(victim)), &gotgbot.EditMessageTextOpts{ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
	if err != nil {
		return err
	}
	bullet := RandInt(1, 5)
	for i := 1; i <= bullet; i++ {
		time.Sleep(time.Second * 2)
		prefix = prt.Sprintf("Дуэль! %v против %v, раунд %v:\n%v берёт револьвер, приставляет его к голове и...\n", MentionUser(player), MentionUser(victim), i, MentionUser(victim))
		_, _, err = Bot.EditMessageText(prefix, &gotgbot.EditMessageTextOpts{ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		if err != nil {
			return err
		}
		if bullet != i {
			time.Sleep(time.Second * 2)
			_, _, err := Bot.EditMessageText(prt.Sprintf("%v🍾 %v", prefix, prt.Sprintf(success[RandInt(0, len(success)-1)], MentionUser(victim))), &gotgbot.EditMessageTextOpts{ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
			if err != nil {
				return err
			}
			player, victim = victim, player
		}
	}
	time.Sleep(time.Second * 2)
	PlayerChatMember, err := Bot.GetChatMember(context.EffectiveChat.Id, player.Id, nil)
	if err != nil {
		return err
	}
	VictimChatMember, err := Bot.GetChatMember(context.EffectiveChat.Id, victim.Id, nil)
	if err != nil {
		return err
	}
	if (PlayerChatMember.GetStatus() == "creator" || PlayerChatMember.GetStatus() == "administrator") && (VictimChatMember.GetStatus() == "creator" || VictimChatMember.GetStatus() == "administrator") {
		_, _, err = Bot.EditMessageText(prt.Sprintf("%vПуля отскакивает от головы %v и летит в голову %v.", prefix, MentionUser(victim), MentionUser(player)), &gotgbot.EditMessageTextOpts{ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		if err != nil {
			return err
		}
		time.Sleep(time.Second * 2)
		_, _, err = Bot.EditMessageText(prt.Sprintf("%vПуля отскакивает от головы %v и летит в голову %v.", prefix, MentionUser(player), MentionUser(victim)), &gotgbot.EditMessageTextOpts{ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		if err != nil {
			return err
		}
		time.Sleep(time.Second * 2)
		_, _, err = Bot.EditMessageText(prt.Sprintf("%vПуля отскакивает от головы %v и летит в голову %v.", prefix, MentionUser(victim), MentionUser(player)), &gotgbot.EditMessageTextOpts{ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		if err != nil {
			return err
		}
		time.Sleep(time.Second * 2)
		var userID int64
		rows, err := DB.Model(&Stats{}).Where("stat_type = 3").Order("last_update desc").Select("context_id").Limit(100).Rows()
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
				VictimChatMember = ricochetVictim
				*victim = ricochetVictim.GetUser()
				prefix = prt.Sprintf("%vПуля отскакивает от головы %v и летит в голову %v.\n", prefix, MentionUser(player), MentionUser(victim))
				_, _, err = Bot.EditMessageText(prefix, &gotgbot.EditMessageTextOpts{ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
				if err != nil {
					return err
				}
				player = &bot.User
				rows.Close()
				break
			}
		}
	}
	if IsAdmin(victim.Id) {
		_, _, err = Bot.EditMessageText(prt.Sprintf("%v😈 Наводит револьвер на %v и стреляет.", prefix, MentionUser(player)), &gotgbot.EditMessageTextOpts{ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		if err != nil {
			return err
		}
		time.Sleep(time.Second * 3)
		var duelist Duelist
		result := DB.Model(Duelist{}).Where(player.Id).First(&duelist)
		if result.RowsAffected == 0 {
			duelist.UserID = player.Id
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
		_, err = Bot.RestrictChatMember(context.EffectiveChat.Id, PlayerChatMember.GetUser().Id, gotgbot.ChatPermissions{CanSendMessages: false}, &gotgbot.RestrictChatMemberOpts{UntilDate: time.Now().Add(time.Second * time.Duration(60*duelist.Deaths)).Unix()})
		if err != nil {
			return err
		}
		_, _, err = Bot.EditMessageText(prt.Sprintf("%v😈 Наводит револьвер на %v и стреляет.\nЯ хз как это объяснить, но %v победитель!\n%v отправился на респавн на %d мин.", prefix, MentionUser(player), MentionUser(victim), MentionUser(player), duelist.Deaths), &gotgbot.EditMessageTextOpts{ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		if err != nil {
			return err
		}
		return err
	}
	if VictimChatMember.GetStatus() == "creator" || VictimChatMember.GetStatus() == "administrator" {
		prefix = prt.Sprintf("%v💥 %v", prefix, prt.Sprintf(invincible[RandInt(0, len(invincible)-1)], MentionUser(victim)))
		_, _, err := Bot.EditMessageText(prefix, &gotgbot.EditMessageTextOpts{ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		if err != nil {
			return err
		}
		time.Sleep(time.Second * 2)
		_, _, err = Bot.EditMessageText(prt.Sprintf("%v\nПохоже, у нас ничья.", prefix), &gotgbot.EditMessageTextOpts{ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		if err != nil {
			return err
		}
		return err
	}
	prefix = prt.Sprintf("%v💥 %v", prefix, prt.Sprintf(fail[RandInt(0, len(fail)-1)], MentionUser(victim)))
	_, _, err = Bot.EditMessageText(prefix, &gotgbot.EditMessageTextOpts{ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
	if err != nil {
		return err
	}
	time.Sleep(time.Second * 2)
	var VictimDuelist Duelist
	result := DB.Model(Duelist{}).Where(victim.Id).First(&VictimDuelist)
	if result.RowsAffected == 0 {
		VictimDuelist.UserID = victim.Id
		VictimDuelist.Kills = 0
		VictimDuelist.Deaths = 0
	}
	VictimDuelist.Deaths++
	result = DB.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&VictimDuelist)
	if result.Error != nil {
		return result.Error
	}
	if player.IsBot {
		VictimDuelist.Deaths = 1
	}
	_, err = Bot.RestrictChatMember(context.EffectiveChat.Id, VictimChatMember.GetUser().Id, gotgbot.ChatPermissions{CanSendMessages: false}, &gotgbot.RestrictChatMemberOpts{UntilDate: time.Now().Add(time.Second * time.Duration(60*VictimDuelist.Deaths)).Unix()})
	if err != nil {
		return err
	}
	_, _, err = Bot.EditMessageText(prt.Sprintf("%v\nПобедитель дуэли: %v.\n%v отправился на респавн на %d мин.", prefix, MentionUser(player), MentionUser(victim), VictimDuelist.Deaths), &gotgbot.EditMessageTextOpts{ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
	if err != nil {
		return err
	}
	var PlayerDuelist Duelist
	result = DB.Model(Duelist{}).Where(victim.Id).First(&PlayerDuelist)
	if result.RowsAffected == 0 {
		PlayerDuelist.UserID = victim.Id
		PlayerDuelist.Kills = 0
		PlayerDuelist.Deaths = 0
	}
	PlayerDuelist.Kills++
	result = DB.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&PlayerDuelist)
	if result.Error != nil {
		return result.Error
	}
	return err
}
