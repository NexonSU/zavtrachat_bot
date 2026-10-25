package main

import (
	"fmt"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"golang.org/x/text/language"
	plurals "golang.org/x/text/message"
)

func Accept(bot *gotgbot.Bot, context *ext.Context) error {
	return acceptDuelWith(bot, context, RandInt, time.Sleep)
}

// Supplying randomness and pauses makes every game outcome reproducible in
// tests, while the real handler retains its existing probabilities and pacing.
func acceptDuelWith(bot *gotgbot.Bot, context *ext.Context, random func(int, int) int, pause func(time.Duration)) error {
	// prt will replace fmt package to format text according plurals defined in utils package
	// If no plural rule matched it will be ignored and processed as usual formatting
	prt := plurals.NewPrinter(language.Russian)

	playerValue, victimValue, reason := claimDuel(context)
	if reason != "" {
		return answerDuelRefusal(bot, context, reason)
	}
	defer finishDuel()
	message := context.EffectiveMessage
	player, victim := &playerValue, &victimValue
	_, err := context.Update.CallbackQuery.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{})
	if err != nil {
		return err
	}
	success := []string{"%v остаётся в живых. Хм... может порох отсырел?", "В воздухе повисла тишина. %v остаётся в живых.", "%v сегодня заново родился.", "%v остаётся в живых. Хм... я ведь зарядил его?", "%v остаётся в живых. Прикольно, а давай проверим на ком-нибудь другом?"}
	invincible := []string{"пуля отскочила от головы %v и улетела в другой чат.", "%v похмурил брови и отклеил расплющенную пулю со своей головы.", "но ничего не произошло. %v взглянул на револьвер, он был неисправен.", "пуля прошла навылет, но не оставила каких-либо следов на %v."}
	fail := []string{"мозги %v разлетелись по чату!", "%v упал со стула и его кровь растеклась по месседжу.", "%v замер и спустя секунду упал на стол.", "пуля едва не задела кого-то из участников чата! А? Что? А, %v мёртв, да.", "и в воздухе повисла тишина. Все начали оглядываться, когда %v уже был мёртв."}
	prefix := prt.Sprintf("Дуэль! %v против %v!\n", MentionUser(player), MentionUser(victim))
	_, _, err = Bot.EditMessageText(&gotgbot.EditMessageTextOpts{Text: prt.Sprintf("%vЗаряжаю один патрон в револьвер и прокручиваю барабан.", prefix), ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
	if err != nil {
		return err
	}
	pause(time.Second * 2)
	_, _, err = Bot.EditMessageText(&gotgbot.EditMessageTextOpts{Text: prt.Sprintf("%vКладу револьвер на стол и раскручиваю его.", prefix), ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
	if err != nil {
		return err
	}
	pause(time.Second * 2)
	if random(1, 360)%2 == 0 {
		player, victim = victim, player
	}
	_, _, err = Bot.EditMessageText(&gotgbot.EditMessageTextOpts{Text: prt.Sprintf("%vРевольвер останавливается на %v, первый ход за ним.", prefix, MentionUser(victim)), ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
	if err != nil {
		return err
	}
	bullet := random(1, 5)
	for i := 1; i <= bullet; i++ {
		pause(time.Second * 2)
		prefix = prt.Sprintf("Дуэль! %v против %v, раунд %v:\n%v берёт револьвер, приставляет его к голове и...\n", MentionUser(player), MentionUser(victim), i, MentionUser(victim))
		_, _, err = Bot.EditMessageText(&gotgbot.EditMessageTextOpts{Text: prefix, ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		if err != nil {
			return err
		}
		if bullet != i {
			pause(time.Second * 2)
			_, _, err := Bot.EditMessageText(&gotgbot.EditMessageTextOpts{Text: prt.Sprintf("%v🍾 %v", prefix, prt.Sprintf(success[random(0, len(success)-1)], MentionUser(victim))), ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
			if err != nil {
				return err
			}
			player, victim = victim, player
		}
	}
	pause(time.Second * 2)
	PlayerChatMember, err := Bot.GetChatMember(context.EffectiveChat.Id, player.Id, nil)
	if err != nil {
		return err
	}
	VictimChatMember, err := Bot.GetChatMember(context.EffectiveChat.Id, victim.Id, nil)
	if err != nil {
		return err
	}
	if (PlayerChatMember.GetStatus() == "creator" || PlayerChatMember.GetStatus() == "administrator") && (VictimChatMember.GetStatus() == "creator" || VictimChatMember.GetStatus() == "administrator") {
		_, _, err = Bot.EditMessageText(&gotgbot.EditMessageTextOpts{Text: prt.Sprintf("%vПуля отскакивает от головы %v и летит в голову %v.", prefix, MentionUser(victim), MentionUser(player)), ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		if err != nil {
			return err
		}
		pause(time.Second * 2)
		_, _, err = Bot.EditMessageText(&gotgbot.EditMessageTextOpts{Text: prt.Sprintf("%vПуля отскакивает от головы %v и летит в голову %v.", prefix, MentionUser(player), MentionUser(victim)), ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		if err != nil {
			return err
		}
		pause(time.Second * 2)
		_, _, err = Bot.EditMessageText(&gotgbot.EditMessageTextOpts{Text: prt.Sprintf("%vПуля отскакивает от головы %v и летит в голову %v.", prefix, MentionUser(victim), MentionUser(player)), ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		if err != nil {
			return err
		}
		pause(time.Second * 2)
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
				_, _, err = Bot.EditMessageText(&gotgbot.EditMessageTextOpts{Text: prefix, ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
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
		_, _, err = Bot.EditMessageText(&gotgbot.EditMessageTextOpts{Text: prt.Sprintf("%v😈 Наводит револьвер на %v и стреляет.", prefix, MentionUser(player)), ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		if err != nil {
			return err
		}
		pause(time.Second * 3)
		duelist, err := recordGameDeath(player.Id)
		if err != nil {
			return err
		}
		trueVal := true
		restricted, err := restrictChatMemberTracked(Bot, context.EffectiveChat.Id, PlayerChatMember.GetUser().Id, gotgbot.ChatPermissions{
			CanSendMessages:    false,
			CanReactToMessages: &trueVal,
		}, &gotgbot.RestrictChatMemberOpts{UntilDate: time.Now().Add(time.Second * time.Duration(60*duelist.Deaths)).Unix()}, victim, "duel")
		if err != nil {
			return err
		}
		if !restricted {
			return fmt.Errorf("Telegram rejected duel restriction")
		}
		// Mute time (stat type 7) accumulates the restriction in seconds.
		if err := statsAdd(7, GetStartOfDay(), player.Id, int64(duelist.Deaths)*60); err != nil {
			return err
		}
		_, _, err = Bot.EditMessageText(&gotgbot.EditMessageTextOpts{Text: prt.Sprintf("%v😈 Наводит револьвер на %v и стреляет.\nЯ хз как это объяснить, но %v победитель!\n%v отправился на респавн на %d %v.", prefix, MentionUser(player), MentionUser(victim), MentionUser(player), duelist.Deaths, ruPlural(duelist.Deaths, "минуту", "минуты", "минут")), ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		if err != nil {
			return err
		}
		return err
	}
	if VictimChatMember.GetStatus() == "creator" || VictimChatMember.GetStatus() == "administrator" {
		prefix = prt.Sprintf("%v💥 %v", prefix, prt.Sprintf(invincible[random(0, len(invincible)-1)], MentionUser(victim)))
		_, _, err := Bot.EditMessageText(&gotgbot.EditMessageTextOpts{Text: prefix, ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		if err != nil {
			return err
		}
		pause(time.Second * 2)
		_, _, err = Bot.EditMessageText(&gotgbot.EditMessageTextOpts{Text: prt.Sprintf("%v\nПохоже, у нас ничья.", prefix), ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
		if err != nil {
			return err
		}
		return err
	}
	prefix = prt.Sprintf("%v💥 %v", prefix, prt.Sprintf(fail[random(0, len(fail)-1)], MentionUser(victim)))
	_, _, err = Bot.EditMessageText(&gotgbot.EditMessageTextOpts{Text: prefix, ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
	if err != nil {
		return err
	}
	pause(time.Second * 2)
	VictimDuelist, err := recordGameDeath(victim.Id)
	if err != nil {
		return err
	}
	if player.IsBot {
		VictimDuelist.Deaths = 1
	}
	trueVal := true
	restricted, err := restrictChatMemberTracked(Bot, context.EffectiveChat.Id, VictimChatMember.GetUser().Id, gotgbot.ChatPermissions{
		CanSendMessages:    false,
		CanReactToMessages: &trueVal,
	}, &gotgbot.RestrictChatMemberOpts{UntilDate: time.Now().Add(time.Second * time.Duration(60*VictimDuelist.Deaths)).Unix()}, player, "duel")
	if err != nil {
		return err
	}
	if !restricted {
		return fmt.Errorf("Telegram rejected duel restriction")
	}
	// Mute time (stat type 7) accumulates the restriction in seconds.
	if err := statsAdd(7, GetStartOfDay(), victim.Id, int64(VictimDuelist.Deaths)*60); err != nil {
		return err
	}
	_, err = addDuelistCounter(player.Id, "kills")
	if err != nil {
		return err
	}
	_, _, err = Bot.EditMessageText(&gotgbot.EditMessageTextOpts{Text: prt.Sprintf("%v\nПобедитель дуэли: %v.\n%v отправился на респавн на %d %v.", prefix, MentionUser(player), MentionUser(victim), VictimDuelist.Deaths, ruPlural(VictimDuelist.Deaths, "минуту", "минуты", "минут")), ParseMode: gotgbot.ParseModeHTML, ChatId: context.EffectiveChat.Id, MessageId: message.MessageId, ReplyMarkup: gotgbot.InlineKeyboardMarkup{}})
	if err != nil {
		return err
	}
	return err
}
