package main

import (
	"context"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

const mutedPrefix = "muted:"
const mutedPageSize = 5

func canReadMuted(bot *gotgbot.Bot, userID int64) bool {
	if userID == Config.SysAdmin || IsAdminOrModer(userID) {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	member, err := bot.GetChatMemberWithContext(ctx, Config.Chat, userID, nil)
	return err == nil && (member.GetStatus() == "administrator" || member.GetStatus() == "creator")
}

func Muted(bot *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveChat == nil || ctx.EffectiveUser == nil || ctx.EffectiveMessage == nil {
		return nil
	}
	if ctx.EffectiveChat.Id != Config.Chat || (ctx.EffectiveChat.Type != "group" && ctx.EffectiveChat.Type != "supergroup") {
		return mutedNotice(bot, ctx, "Команда /muted работает в основном чате.")
	}
	if !canReadMuted(bot, ctx.EffectiveUser.Id) {
		return mutedNotice(bot, ctx, "Список доступен только администраторам и модераторам чата.")
	}
	return showMuted(bot, ctx, 0)
}

// Always use a recipient-specific response in a group, including errors and
// denials. There is deliberately no public fallback when delivery fails.
func mutedNotice(bot *gotgbot.Bot, ctx *ext.Context, text string) error {
	opts := &gotgbot.SendMessageOpts{}
	if ctx.EffectiveChat.Type != "private" {
		opts.EphemeralMessageParameters = &gotgbot.EphemeralMessageParameters{ReceiverUserId: ctx.EffectiveUser.Id}
	}
	_, err := bot.SendMessage(ctx.EffectiveChat.Id, text, opts)
	return err
}

func MutedCallback(bot *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.EffectiveUser == nil || ctx.EffectiveChat == nil || ctx.EffectiveMessage == nil {
		return nil
	}
	parts := strings.Split(ctx.CallbackQuery.Data, ":")
	var owner int64
	var page int
	var parseErr error
	if len(parts) == 3 && parts[0] == "muted" {
		owner, parseErr = strconv.ParseInt(parts[1], 10, 64)
		if parseErr == nil {
			page, parseErr = strconv.Atoi(parts[2])
		}
	} else {
		parseErr = fmt.Errorf("invalid muted callback")
	}
	msg := ctx.EffectiveMessage
	if parseErr != nil || page < 0 || owner != ctx.EffectiveUser.Id || ctx.EffectiveChat.Id != Config.Chat ||
		msg.EphemeralMessageId == 0 || msg.ReceiverUser == nil || msg.ReceiverUser.Id != owner || !canReadMuted(bot, owner) {
		_, err := ctx.CallbackQuery.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: "Список недоступен. Вызови /muted заново."})
		return err
	}
	if _, err := ctx.CallbackQuery.Answer(bot, nil); err != nil {
		return err
	}
	return showMuted(bot, ctx, page)
}

func showMuted(bot *gotgbot.Bot, ctx *ext.Context, page int) error {
	events, err := latestMuteEvents(ctx.EffectiveChat.Id)
	if err != nil {
		return mutedNotice(bot, ctx, "Не удалось прочитать учёт мутов. Попробуй позже.")
	}
	snapshot := loadMutedSnapshot(bot, ctx.EffectiveChat.Id, events)
	fillMutedNames(&snapshot, events)
	text, keyboard := renderMuted(snapshot, events, bot.Id, ctx.EffectiveUser.Id, page, time.Now())
	if ctx.CallbackQuery != nil {
		_, err := bot.EditEphemeralMessageText(ctx.EffectiveChat.Id, ctx.EffectiveUser.Id, ctx.EffectiveMessage.EphemeralMessageId,
			&gotgbot.EditEphemeralMessageTextOpts{Text: text, ParseMode: gotgbot.ParseModeHTML, ReplyMarkup: keyboard})
		// A refresh within the same second can produce an identical view.
		if err != nil && strings.Contains(err.Error(), "message is not modified") {
			return nil
		}
		return err
	}
	_, err = bot.SendMessage(ctx.EffectiveChat.Id, text, &gotgbot.SendMessageOpts{
		ParseMode: gotgbot.ParseModeHTML, ReplyMarkup: keyboard,
		EphemeralMessageParameters: &gotgbot.EphemeralMessageParameters{ReceiverUserId: ctx.EffectiveUser.Id},
	})
	return err
}

func fillMutedNames(snapshot *mutedSnapshot, events []MuteEvent) {
	names := map[int64]string{}
	for _, event := range events {
		names[event.ActorID] = event.ActorName
	}
	var ids []int64
	for _, member := range snapshot.members {
		if UserFullName(&member.user) == "" {
			ids = append(ids, member.user.Id)
		}
		if member.provider == "" && member.providerID != 0 {
			ids = append(ids, member.providerID)
		}
	}
	if len(ids) > 0 {
		var users []gotgbot.User
		if err := DB.Where("id IN ?", ids).Find(&users).Error; err == nil {
			for _, user := range users {
				names[user.Id] = UserFullName(&user)
			}
		}
	}
	for i := range snapshot.members {
		member := &snapshot.members[i]
		if UserFullName(&member.user) == "" {
			member.user.FirstName = names[member.user.Id]
		}
		if member.provider == "" {
			member.provider = names[member.providerID]
		}
	}
}

func mutedName(name string, userID int64) string {
	runes := []rune(name)
	if len(runes) > 80 {
		name = string(runes[:80]) + "…"
	}
	if strings.TrimSpace(name) == "" {
		name = strconv.FormatInt(userID, 10)
	}
	return html.EscapeString(name)
}

func mutedOrigin(member mutedMember, event MuteEvent, botID int64) string {
	// The live Telegram actor wins over our journal, especially if an admin
	// has replaced a bot restriction manually with the same expiry time.
	if member.providerID != 0 && member.providerID != botID {
		return "Мут выдал: " + mutedName(member.provider, member.providerID) + "."
	}
	valid := event.Muted && event.UntilDate == member.until && event.UserID == member.user.Id
	if member.providerID != 0 {
		valid = valid && event.ProviderID == member.providerID
	}
	if member.date != 0 {
		// Match the current Telegram restriction to the successful API call,
		// rather than attributing an unrelated later mute to an old game.
		valid = valid && member.date >= event.StartedAt-2 && member.date <= event.CompletedAt+2
	}
	if !valid {
		return "Причина неизвестна."
	}
	actor := mutedName(event.ActorName, event.ActorID)
	switch event.Source {
	case "blessing":
		if event.ActorID == member.user.Id {
			return "Сам блессанулся."
		}
		return "Рикошет блесса от " + actor + "."
	case "kill":
		return "Пристрелил: " + actor + "."
	case "ricochet":
		return "Рикошет; команду вызвал: " + actor + "."
	case "shotgun", "megamlem":
		return "Попал под /" + event.Source + "; команду вызвал: " + actor + "."
	case "duel":
		return "Погиб на дуэли с " + actor + "."
	case "mute":
		return "Замьючен админом " + actor + "."
	case "warn":
		return "Мут за три предупреждения; последнее выдал: " + actor + "."
	case "manual":
		return "Мут выдал: " + actor + "."
	default:
		return "Причина неизвестна."
	}
}

func renderMuted(snapshot mutedSnapshot, events []MuteEvent, botID, ownerID int64, page int, now time.Time) (string, gotgbot.InlineKeyboardMarkup) {
	// Work on a copy: rendering should not mutate a shared snapshot.
	members := append([]mutedMember(nil), snapshot.members...)
	sort.Slice(members, func(i, j int) bool {
		if members[i].until == members[j].until {
			return members[i].user.Id < members[j].user.Id
		}
		if members[i].until == 0 {
			return false
		}
		return members[j].until == 0 || members[i].until < members[j].until
	})
	byUser := make(map[int64]MuteEvent, len(events))
	for _, event := range events {
		byUser[event.UserID] = event
	}
	pages := max(1, (len(members)+mutedPageSize-1)/mutedPageSize)
	page = min(max(0, page), pages-1)
	var text strings.Builder
	fmt.Fprintf(&text, "🔇 <b>Сейчас замьючены: %d</b>\n", len(members))
	if snapshot.partial {
		text.WriteString("⚠️ Полный список Telegram недоступен. Показаны только известные боту муты, проверенные сейчас.\n")
	}
	if snapshot.unverified > 0 {
		fmt.Fprintf(&text, "Не удалось проверить участников: %d.\n", snapshot.unverified)
	}
	if len(members) == 0 {
		if snapshot.partial {
			text.WriteString("\nСреди проверенных участников замьюченных нет.\n")
		} else {
			text.WriteString("\nНикто не замьючен.\n")
		}
	}
	moscow := time.FixedZone("МСК", 3*3600)
	for _, member := range members[page*mutedPageSize : min((page+1)*mutedPageSize, len(members))] {
		until := "бессрочно"
		if member.until != 0 {
			minutes := max(1, (member.until-now.Unix()+59)/60)
			until = fmt.Sprintf("ещё %d %s, до %s МСК", minutes, ruPlural(minutes, "минуту", "минуты", "минут"), time.Unix(member.until, 0).In(moscow).Format("02.01 15:04"))
		}
		fmt.Fprintf(&text, "\n<a href=\"tg://user?id=%d\">%s</a> — %s\n↳ %s\n", member.user.Id, mutedName(UserFullName(&member.user), member.user.Id), until, mutedOrigin(member, byUser[member.user.Id], botID))
	}
	fmt.Fprintf(&text, "\nНа %s МСК · Страница %d/%d", now.In(moscow).Format("15:04:05"), page+1, pages)
	button := func(label string, target int) gotgbot.InlineKeyboardButton {
		return gotgbot.InlineKeyboardButton{Text: label, CallbackData: fmt.Sprintf("%s%d:%d", mutedPrefix, ownerID, target)}
	}
	var row []gotgbot.InlineKeyboardButton
	if page > 0 {
		row = append(row, button("← Назад", page-1))
	}
	row = append(row, button("🔄 Обновить", page))
	if page+1 < pages {
		row = append(row, button("Вперёд →", page+1))
	}
	return text.String(), gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{row}}
}
