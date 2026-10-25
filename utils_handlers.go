package main

// The update handlers the dispatcher runs for every message: user records, the album cache,
// the ban check, the stat counters, and the message removal helpers.

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm/clause"
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
	UserResult := DB.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&chatUser)
	if UserResult.Error != nil {
		ErrorReporting(UserResult.Error)
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

	//User update
	UserResult := DB.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&context.Message.From)
	if UserResult.Error != nil {
		ErrorReporting(UserResult.Error)
	}

	//update StatsDays(1), StatsHours(2), StatsUsers(3), StatsWords(4), StatsWeekday(5)
	startOfDay := GetStartOfDay()
	timeNow := time.Now().Local()
	for _, counter := range []struct{ kind, id int64 }{
		{1, int64(timeNow.Day())}, {2, int64(timeNow.Hour())},
		{3, context.Message.From.Id}, {5, int64(timeNow.Weekday())},
	} {
		if err := statsIncrease(counter.kind, startOfDay, counter.id); err != nil {
			return err
		}
	}
	if context.Message.Text == "" || string(context.Message.Text[0]) == "/" {
		return nil
	}
	return recordMessageWords(context.Message.Text, startOfDay)
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
