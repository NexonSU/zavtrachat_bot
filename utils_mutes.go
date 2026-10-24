package main

import (
	"log/slog"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm/clause"
)

// MuteEvent records successful restrictions and lifts, separately from game
// counters. Telegram remains the authority for whether a member is muted now.
// Nullable update IDs deduplicate redelivered chat_member updates; commands
// can produce several events (shotgun/redemption), so their IDs are not unique.
type MuteEvent struct {
	ID               uint64 `gorm:"primaryKey"`
	ChatID           int64  `gorm:"index:idx_mute_member,priority:1"`
	UserID           int64  `gorm:"index:idx_mute_member,priority:2"`
	ActorID          int64
	ActorName        string
	ProviderID       int64
	Source           string
	Muted            bool
	UntilDate        int64
	StartedAt        int64
	CompletedAt      int64  `gorm:"index:idx_mute_member,priority:3"`
	TelegramUpdateID *int64 `gorm:"uniqueIndex"`
}

func normalizedMuteUntil(until, now int64) int64 {
	// Telegram treats restrictions shorter than 30 seconds or longer than
	// 366 days as permanent. Keep the API request itself unchanged.
	if until == 0 || until-now < 30 || until-now > 366*86400 {
		return 0
	}
	return until
}

func saveMuteEvent(event *MuteEvent) {
	if err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(event).Error; err != nil {
		// A reporting failure must not turn a successful game/moderation action
		// into a failed one, or leak moderation details to the public timeline.
		slog.Warn("mute event could not be recorded")
	}
}

func restrictChatMemberTracked(bot *gotgbot.Bot, chatID, userID int64, permissions gotgbot.ChatPermissions, opts *gotgbot.RestrictChatMemberOpts, actor *gotgbot.User, source string) (bool, error) {
	started := time.Now().Unix()
	ok, err := bot.RestrictChatMember(chatID, userID, permissions, opts)
	if err != nil || !ok {
		return ok, err
	}
	event := MuteEvent{ChatID: chatID, UserID: userID, ProviderID: bot.Id, Source: source,
		Muted: !permissions.CanSendMessages, StartedAt: started, CompletedAt: time.Now().Unix()}
	if actor != nil {
		event.ActorID, event.ActorName = actor.Id, UserFullName(actor)
	}
	if event.Muted && opts != nil {
		event.UntilDate = normalizedMuteUntil(opts.UntilDate, started)
	}
	saveMuteEvent(&event)
	return ok, nil
}

func chatMemberIsMuted(member gotgbot.ChatMember) bool {
	if member == nil || member.GetStatus() != "restricted" {
		return false
	}
	m := member.MergeChatMember()
	return m.IsMember && !m.CanSendMessages
}

// Observe manual restrictions, including changes made in Telegram's admin UI.
// Our own API calls record the actual initiating human and game source above;
// the generic bot-authored update must not overwrite that richer information.
func OnMuteChatMember(bot *gotgbot.Bot, ctx *ext.Context) error {
	change := ctx.ChatMember
	if change == nil || change.Chat.Id != Config.Chat || change.From.Id == bot.Id || change.NewChatMember == nil {
		return nil
	}
	wasMuted, isMuted := chatMemberIsMuted(change.OldChatMember), chatMemberIsMuted(change.NewChatMember)
	if !wasMuted && !isMuted {
		return nil
	}
	until := change.NewChatMember.MergeChatMember().UntilDate
	if wasMuted && isMuted && change.OldChatMember.MergeChatMember().UntilDate == until {
		return nil
	}
	user := change.NewChatMember.GetUser()
	event := MuteEvent{ChatID: change.Chat.Id, UserID: user.Id, ActorID: change.From.Id,
		ActorName: UserFullName(&change.From), ProviderID: change.From.Id, Source: "manual",
		Muted: isMuted, StartedAt: change.Date, CompletedAt: change.Date, TelegramUpdateID: &ctx.UpdateId}
	if isMuted {
		// Values here come from Telegram, so do not re-apply the 30-second rule.
		event.UntilDate = until
	}
	saveMuteEvent(&event)
	return nil
}

func latestMuteEvents(chatID int64) ([]MuteEvent, error) {
	var events []MuteEvent
	// chat_member delivery can be delayed or out of order. Its insertion ID
	// alone must not resurrect an older restriction after a more recent lift.
	times := DB.Model(&MuteEvent{}).Select("user_id, MAX(completed_at)").Where("chat_id = ?", chatID).Group("user_id")
	latest := DB.Model(&MuteEvent{}).Select("MAX(id)").Where("chat_id = ?", chatID).
		Where("(user_id, completed_at) IN (?)", times).Group("user_id")
	err := DB.Where("chat_id = ? AND id IN (?)", chatID, latest).Order("user_id").Find(&events).Error
	return events, err
}
