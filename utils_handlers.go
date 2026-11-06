package main

// The update handlers the dispatcher runs for every message: user records, the album cache,
// the ban check, the stat counters, and the message removal helpers.

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type AlbumCache struct {
	mu     sync.Mutex
	timers map[string]*time.Timer
	groups map[string][]int64
}

var ChatAlbumCache = &AlbumCache{
	timers: make(map[string]*time.Timer),
	groups: make(map[string][]int64),
}

var onlyWords = regexp.MustCompile(`[,.!?]+`)

// Remove message
func Remove(bot *gotgbot.Bot, context *ext.Context) error {
	var err error
	if context.EffectiveMessage.EphemeralMessageId != 0 {
		_, err = Bot.DeleteEphemeralMessage(context.EffectiveChat.Id, context.EffectiveMessage.ReceiverUser.Id, context.EffectiveMessage.EphemeralMessageId, nil)
	} else {
		_, err = Bot.DeleteMessage(context.EffectiveChat.Id, context.EffectiveMessage.MessageId, nil)
	}
	return err
}

func RemoveJoinMessageAndJoinUser(bot *gotgbot.Bot, context *ext.Context) error {
	Remove(bot, context)
	for _, user := range context.Message.NewChatMembers {
		context.EffectiveChat.Unban(bot, user.Id, nil)
	}
	return nil
}

func OnChatMember(bot *gotgbot.Bot, context *ext.Context) error {
	if context.EffectiveChat.Id == Config.ReserveChat {
		bot.UnbanChatMember(context.EffectiveChat.Id, context.ChatMember.From.Id, nil)
	}
	//User update
	// GetUser() returns a value; gorm needs an addressable pointer, so copy
	// it into a local variable first (passing the value directly panics in
	// the create callback: "reflect.Value.SetInt using unaddressable value").
	chatUser := context.ChatMember.NewChatMember.GetUser()
	if err := saveUser(&DB, chatUser); err != nil {
		ErrorReporting(err)
	}
	return nil
}

func OnText(bot *gotgbot.Bot, context *ext.Context) error {
	if context.Message == nil || context.Message.From == nil {
		return nil
	}
	if context.EffectiveChat.Id == Config.ReserveChat {
		context.EffectiveMessage.Delete(bot, nil)
		if !IsAdminOrModer(context.EffectiveUser.Id) && bot.Id != context.EffectiveUser.Id {
			bot.UnbanChatMember(context.EffectiveChat.Id, context.EffectiveUser.Id, nil)
		}
	}

	if context.EffectiveMessage.MediaGroupId != "" {
		go CacheAlbumId(bot, context)
	}

	go CheckUserBan(bot, context)
	replyWithShchupatGIF(bot, context.Message)

	//User update
	if err := saveUser(&DB, *context.Message.From); err != nil {
		ErrorReporting(err)
	}

	//update StatsDays(1), StatsHours(2), StatsUsers(3), StatsWords(4), StatsWeekday(5)
	startOfDay := GetStartOfDay()
	timeNow := time.Now().Local()
	rows := make([]Stats, 0, 4)
	for _, counter := range []struct{ kind, id int64 }{
		{1, int64(timeNow.Day())}, {2, int64(timeNow.Hour())},
		{3, context.Message.From.Id}, {5, int64(timeNow.Weekday())},
	} {
		rows = append(rows, Stats{StatType: counter.kind, DayTimestamp: startOfDay, ContextID: counter.id, Count: 1})
	}
	if text := context.Message.Text; text != "" && text[0] != '/' {
		words, err := messageWordStats(text, startOfDay)
		if err != nil {
			return err
		}
		rows = append(rows, words...)
	}
	return statsAddBatchDB(&DB, rows)
}

func CacheAlbumId(bot *gotgbot.Bot, context *ext.Context) error {
	if context.EffectiveMessage.MediaGroupId == "" {
		return nil
	}

	ChatAlbumCache.mu.Lock()
	defer ChatAlbumCache.mu.Unlock()

	groupID := context.EffectiveMessage.MediaGroupId
	ChatAlbumCache.groups[groupID] = append(ChatAlbumCache.groups[groupID], context.EffectiveMessage.MessageId)
	return nil
}

func CheckUserBan(bot *gotgbot.Bot, context *ext.Context) error {
	user, err := FindUserInMessage(*context.Message)
	if err != nil {
		return nil
	}
	cm, err := bot.GetChatMember(context.EffectiveChat.Id, user.Id, nil)
	if err != nil {
		return nil
	}
	mcm := cm.MergeChatMember()
	if mcm.Status == "left" {
		return Reply(fmt.Sprintf("%v не сможет ответить, т.к. вышел из чата.", MentionUser(&user)), *context)
	}
	if mcm.Status == "kicked" {
		return Reply(fmt.Sprintf("%v не сможет ответить, т.к. был забанен в чате.", MentionUser(&user)), *context)
	}
	if !mcm.CanSendMessages && mcm.Status == "restricted" {
		duration := (mcm.UntilDate - time.Now().Local().Unix()) / 60
		return Reply(fmt.Sprintf("%v не сможет ответить, т.к. умир.\nРеспавн через %d %v.", MentionUser(&user), duration, ruPlural(duration, "минуту", "минуты", "минут")), *context)
	}
	return nil
}

// Deny message on admin command
func Denied(bot *gotgbot.Bot, context *ext.Context) error {
	return Reply(GetNope(), *context)
}

func OnReaction(bot *gotgbot.Bot, context *ext.Context) error {
	if context.MessageReaction == nil {
		return nil
	}
	if len(context.MessageReaction.NewReaction) == 0 {
		return nil
	}

	statsIncrease(6, GetStartOfDay(), context.MessageReaction.MessageId)

	return nil
}
const shchupatGIF = "CgACAgIAAx0EReJGYgABQWStaugypHVPB10OnVZd4Q_L0nXqqpcAAgwRAAKEp7lKqKkocsx-ZdM9BA"

var shchupatWord = regexp.MustCompile(`(?i)(^|[^\pL\pM\pN_])(?:по|о|на|про|при|под|вы|за|пере|до|от)?щуп(?:ать|ал[аио]?|а(?:ю|ешь|ет|ем|ете|ют)|ай(?:те)?|ая|ав(?:ши|ш(?:ий|ая|ее|ие))?)(?:ся|сь)?($|[^\pL\pM\pN_])`)

type shchupatBudget struct {
	mu   sync.Mutex
	last [3]time.Time
}

func (b *shchupatBudget) take(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if now.Sub(b.last[0]) < time.Hour {
		return false
	}
	copy(b.last[:], b.last[1:])
	b.last[len(b.last)-1] = now
	return true
}

var shchupatReplies shchupatBudget

func replyWithShchupatGIF(bot *gotgbot.Bot, msg *gotgbot.Message) {
	if msg.Chat.Id != Config.Chat || (msg.Chat.Type != "group" && msg.Chat.Type != "supergroup") || msg.From.IsBot || msg.EphemeralMessageId != 0 {
		return
	}
	switch strings.ToLower(msg.From.Username) {
	case "shaikhulud", "zombak", "timurse":
		return
	}
	if !shchupatWord.MatchString(msg.Text) && !shchupatWord.MatchString(msg.Caption) {
		return
	}
	if !shchupatReplies.take(time.Now()) {
		return
	}

	_, err := bot.SendAnimation(msg.Chat.Id, gotgbot.InputFileByID(shchupatGIF), &gotgbot.SendAnimationOpts{
		MessageThreadId: msg.MessageThreadId,
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: msg.MessageId},
	})
	if err != nil {
		slog.Warn("could not send shchupat GIF", "chat_id", msg.Chat.Id, "message_id", msg.MessageId)
	}
}
