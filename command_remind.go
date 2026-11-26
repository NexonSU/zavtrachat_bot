package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const remindUsage = `Личное напоминание (время московское):
<code>/remind 2ч</code> — ответом на сообщение.
<code>/remind 30м проверить духовку</code>
<code>/remind завтра 19:00 посмотреть ссылку</code>
<code>/remind 30.11.2026 19:00 позвонить</code>
Сроки: м, ч, д (или m, h, d), от 1 минуты до 366 дней.
<code>/remind list</code> — мои напоминания.
<code>/remind cancel ID</code> — отменить своё напоминание.
Сначала запусти и разблокируй бота в личке.`

var remindDuration = regexp.MustCompile(`^([0-9]+)([мчдmhd])$`)

// A request key also identifies ephemeral commands, whose regular message ID is zero.
type PersonalReminder struct {
	ID             int64  `gorm:"primaryKey"`
	RequestKey     string `gorm:"uniqueIndex;not null"`
	UserID         int64  `gorm:"index;not null"`
	Text           string
	SourceURL      string
	PrivateReplyID int64
	DueAt          int64
	NextAttemptAt  int64 `gorm:"index:idx_personal_reminder_due,priority:4"`
	SentAt         int64 `gorm:"not null;default:0;index:idx_personal_reminder_due,priority:1"`
	CancelledAt    int64 `gorm:"not null;default:0;index:idx_personal_reminder_due,priority:2"`
	FailedAt       int64 `gorm:"not null;default:0;index:idx_personal_reminder_due,priority:3"`
}

func pendingPersonalReminders(db *gorm.DB) *gorm.DB {
	return db.Where("sent_at = 0 AND cancelled_at = 0 AND failed_at = 0")
}

func parseRemindTime(args []string, now time.Time) (time.Time, int, error) {
	if len(args) == 0 {
		return time.Time{}, 0, errors.New("укажи срок или дату")
	}
	var due time.Time
	used := 1
	if match := remindDuration.FindStringSubmatch(strings.ToLower(args[0])); match != nil {
		amount, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil || amount < 1 || amount > 366*24*60 {
			return due, 0, errors.New("неверный срок")
		}
		unit := time.Minute
		switch match[2] {
		case "ч", "h":
			unit = time.Hour
		case "д", "d":
			unit = 24 * time.Hour
		}
		if amount > int64(366*24*time.Hour/unit) {
			return due, 0, errors.New("максимальный срок — 366 дней")
		}
		due = now.Add(time.Duration(amount) * unit)
	} else {
		if len(args) < 2 {
			return due, 0, errors.New("после даты укажи время ЧЧ:ММ")
		}
		date := args[0]
		switch strings.ToLower(date) {
		case "сегодня":
			date = now.In(betMoscow).Format("02.01.2006")
		case "завтра":
			date = now.In(betMoscow).AddDate(0, 0, 1).Format("02.01.2006")
		}
		var err error
		due, err = time.ParseInLocation("02.01.2006 15:04", date+" "+args[1], betMoscow)
		if err != nil {
			return due, 0, errors.New("не понял дату; пример: 30.11.2026 19:00")
		}
		used = 2
	}
	if due.Before(now.Add(time.Minute)) || due.After(now.Add(366*24*time.Hour)) {
		return due, 0, errors.New("выбери время от 1 минуты до 366 дней в будущем")
	}
	return due, used, nil
}

func remindSourceURL(chat gotgbot.Chat, messageID int64) string {
	if messageID <= 0 {
		return ""
	}
	if chat.Type == "supergroup" && chat.Id < -1000000000000 {
		return fmt.Sprintf("https://t.me/c/%d/%d", -chat.Id-1000000000000, messageID)
	}
	if chat.Type != "private" && chat.Username != "" {
		return fmt.Sprintf("https://t.me/%s/%d", chat.Username, messageID)
	}
	return ""
}

func personalReminderText(row PersonalReminder, heading string) string {
	text := fmt.Sprintf("%s #%d\n%s МСК\n\n%s", heading, row.ID,
		time.Unix(row.DueAt, 0).In(betMoscow).Format("02.01.2006 15:04"), html.EscapeString(row.Text))
	if row.SourceURL != "" {
		text += "\n\n<a href=\"" + html.EscapeString(row.SourceURL) + "\">Исходное сообщение</a>"
	}
	return text
}

func Remind(bot *gotgbot.Bot, ctx *ext.Context) error {
	return remindAt(bot, ctx, time.Now())
}

func remindAt(bot *gotgbot.Bot, ctx *ext.Context, now time.Time) error {
	if ctx.EffectiveMessage == nil || ctx.EffectiveUser == nil || ctx.EffectiveUser.IsBot || ctx.EffectiveChat == nil {
		return nil
	}
	if ctx.EffectiveChat.Type != "private" && ctx.EffectiveChat.Id != Config.Chat {
		return sendZBHelp(bot, ctx, "Напоминания доступны в основном чате и в личке с ботом.")
	}
	args := ctx.Args()
	if len(args) <= 1 {
		return sendZBHelp(bot, ctx, remindUsage)
	}
	args = args[1:]
	userID := ctx.EffectiveUser.Id
	if args[0] == "list" {
		var rows []PersonalReminder
		if err := pendingPersonalReminders(&DB).Where("user_id = ?", userID).Order("due_at, id").Find(&rows).Error; err != nil {
			return err
		}
		text := "Твои активные напоминания:"
		if len(rows) == 0 {
			text = "У тебя нет активных напоминаний."
		}
		for _, row := range rows {
			preview := []rune(row.Text)
			if len(preview) > 80 {
				preview = append(preview[:80], '…')
			}
			text += fmt.Sprintf("\n#%d — %s МСК: %s", row.ID, time.Unix(row.DueAt, 0).In(betMoscow).Format("02.01.2006 15:04"), string(preview))
		}
		for _, part := range splitTelegramText(text) {
			if err := sendZBHelp(bot, ctx, html.EscapeString(part)); err != nil {
				return err
			}
		}
		return nil
	}
	if args[0] == "cancel" {
		if len(args) != 2 {
			return sendZBHelp(bot, ctx, "Пример: <code>/remind cancel 123</code>")
		}
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil || id <= 0 {
			return sendZBHelp(bot, ctx, "Укажи ID из <code>/remind list</code>.")
		}
		result := pendingPersonalReminders(&DB).Model(&PersonalReminder{}).Where("id = ? AND user_id = ?", id, userID).Update("cancelled_at", now.Unix())
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return sendZBHelp(bot, ctx, "Активное напоминание с таким ID у тебя не найдено.")
		}
		return sendZBHelp(bot, ctx, fmt.Sprintf("Напоминание #%d отменено.", id))
	}
	// Relative deadlines are tied to receipt of the original command, so a
	// delayed/replayed Telegram update cannot silently move the reminder.
	base := now
	if ctx.EffectiveMessage.Date > 0 {
		base = time.Unix(ctx.EffectiveMessage.Date, 0)
	}
	due, used, err := parseRemindTime(args, base)
	if err != nil {
		return sendZBHelp(bot, ctx, html.EscapeString(err.Error())+"\n\n"+remindUsage)
	}
	if !due.After(now) {
		return sendZBHelp(bot, ctx, "Это время уже прошло. Задай новое напоминание.")
	}
	msg := ctx.EffectiveMessage
	text := strings.Join(args[used:], " ")
	source := msg
	if msg.ReplyToMessage != nil {
		source = msg.ReplyToMessage
		if text == "" {
			text = source.Text
			if text == "" {
				text = source.Caption
			}
			if text == "" {
				text = "Посмотреть сообщение"
			}
		}
	}
	if strings.TrimSpace(text) == "" {
		return sendZBHelp(bot, ctx, "Добавь текст напоминания или ответь командой на сообщение.\n\n"+remindUsage)
	}
	if betTextUnits(text) > 3500 {
		return sendZBHelp(bot, ctx, "Текст слишком длинный: максимум 3500 символов (эмодзи могут занимать два). Добавь короткий текст после времени; ссылка на оригинал сохранится.")
	}
	row := PersonalReminder{RequestKey: fmt.Sprintf("%d:%d:%d:%d", userID, msg.Chat.Id, msg.MessageId, msg.EphemeralMessageId), UserID: userID,
		Text: text, SourceURL: remindSourceURL(msg.Chat, source.MessageId), DueAt: due.Unix(), NextAttemptAt: due.Unix()}
	if msg.ReplyToMessage == nil {
		// The standalone command is removed after saving; its text is already
		// stored, so do not attach a link that will point to a deleted message.
		row.SourceURL = ""
	}
	if msg.Chat.Type == "private" && msg.ReplyToMessage != nil {
		row.PrivateReplyID = source.MessageId
	}
	deliveryFailed := false
	err = DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil // Redelivery of the same update must not create/send twice.
		}
		_, err := bot.SendMessage(userID, personalReminderText(row, "Напоминание сохранено")+fmt.Sprintf("\n\nОтменить: <code>/remind cancel %d</code>", row.ID), personalReminderSendOpts(row))
		deliveryFailed = err != nil
		return err
	})
	if deliveryFailed {
		return sendZBHelp(bot, ctx, "Не удалось написать тебе в личку. Напоминание не сохранено. Запусти и разблокируй бота в личке и повтори команду.")
	}
	if err != nil {
		return err
	}
	if msg.Chat.Type == "private" {
		return nil
	}
	replyErr := sendZBHelp(bot, ctx, "Напоминание сохранено. Подробности отправлены в личку; список: <code>/remind list</code>.")
	// The reminder and DM confirmation have succeeded. Clean up ordinary
	// group commands even if the optional hidden acknowledgement failed.
	// Leave ephemeral commands to expire, as with /save (mobile client issue).
	if msg.EphemeralMessageId == 0 && msg.MessageId > 0 {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, err := bot.DeleteMessageWithContext(cleanupCtx, msg.Chat.Id, msg.MessageId, nil); err != nil {
			slog.Warn("remind command cleanup failed")
		}
	}
	return replyErr
}

// Private-chat attachments have no shareable URL. Reply to the original instead;
// deleted originals must not prevent the reminder from reaching the user.
func personalReminderSendOpts(row PersonalReminder) *gotgbot.SendMessageOpts {
	opts := &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, RequestOpts: &gotgbot.RequestOpts{Timeout: 15 * time.Second}}
	if row.PrivateReplyID > 0 {
		opts.ReplyParameters = &gotgbot.ReplyParameters{MessageId: row.PrivateReplyID, AllowSendingWithoutReply: true}
	}
	return opts
}
