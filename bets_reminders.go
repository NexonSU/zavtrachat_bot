package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"time"
	"unicode/utf16"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var betMoscow = time.FixedZone("МСК", 3*60*60)

// Existing bets store a date at UTC midnight, not a local instant. Keep that
// representation so migration and /delbet preserve their original date keys.
func betCalendarDay(now time.Time) time.Time {
	local := now.In(betMoscow)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

func betTextUnits(text string) int { return len(utf16.Encode([]rune(text))) }

func betUserLabel(user *gotgbot.User) string {
	if user.Username != "" {
		return "@" + user.Username
	}
	if name := UserFullName(user); name != "" {
		return name
	}
	return strconv.FormatInt(user.Id, 10)
}

func betReminderText(bet Bets, label string) string {
	date := "Ранее"
	if bet.CreatedTimestamp > 0 {
		date = time.Unix(bet.CreatedTimestamp, 0).In(betMoscow).Format("02.01.2006")
	}
	return fmt.Sprintf("%s <a href=\"tg://user?id=%d\">%s</a> сказал:\n<blockquote>%s</blockquote>\nНу что, сбылось?", date, bet.UserID, html.EscapeString(label), html.EscapeString(bet.Text))
}

// Run at startup and on each minute boundary. A late restart catches up only
// today's reminders after noon; old overdue bets never flood the group.
func runBetReminders(ctx context.Context, bot *gotgbot.Bot, db *gorm.DB, chatID int64) {
	if chatID == 0 {
		return
	}
	for {
		if ctx.Err() != nil {
			return
		}
		now := time.Now()
		if err := sendDueBetReminders(bot, db, chatID, now); err != nil {
			slog.Error("bet reminders failed", "error", err)
		}
		delay := time.Until(time.Now().Truncate(time.Minute).Add(time.Minute))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func sendDueBetReminders(bot *gotgbot.Bot, db *gorm.DB, chatID int64, now time.Time) error {
	if chatID == 0 || now.In(betMoscow).Hour() < 12 {
		return nil
	}
	day := betCalendarDay(now).Unix()
	var bets []Bets
	if err := db.Where("timestamp >= ? AND timestamp < ? AND reminder_sent_at = 0", day, day+86400).
		Order("timestamp, user_id, text").Find(&bets).Error; err != nil {
		return err
	}
	var failures []error
	for _, bet := range bets {
		err := db.Transaction(func(tx *gorm.DB) error {
			// PostgreSQL locks survive until commit. Concurrent scheduler instances
			// skip a claimed row; a normal restart sees the persisted sent marker.
			var pending Bets
			err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
				Where("user_id = ? AND text = ? AND timestamp = ? AND reminder_sent_at = 0", bet.UserID, bet.Text, bet.Timestamp).
				First(&pending).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			label := pending.AuthorName
			var user gotgbot.User
			if err := tx.First(&user, pending.UserID).Error; err == nil {
				label = betUserLabel(&user)
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if label == "" {
				label = strconv.FormatInt(pending.UserID, 10)
			}
			// Old bets had no length limit. Split exceptionally long legacy text
			// without cutting HTML entities or UTF-16 surrogate pairs.
			for _, part := range betReminderParts(pending.Text) {
				page := pending
				page.Text = part
				_, err := bot.SendMessage(chatID, betReminderText(page, label), &gotgbot.SendMessageOpts{
					ParseMode:   gotgbot.ParseModeHTML,
					RequestOpts: &gotgbot.RequestOpts{Timeout: 15 * time.Second},
				})
				if err != nil {
					return err
				}
			}
			return tx.Model(&Bets{}).Where("user_id = ? AND text = ? AND timestamp = ?", pending.UserID, pending.Text, pending.Timestamp).
				Update("reminder_sent_at", now.Unix()).Error
		})
		if err != nil {
			failures = append(failures, fmt.Errorf("bet reminder: %w", err))
		}
	}
	return errors.Join(failures...)
}

func betReminderParts(text string) []string {
	var parts []string
	var start, units int
	for offset, r := range text {
		size := 1
		if r > 0xffff {
			size = 2
		}
		if units+size > 3500 {
			parts = append(parts, text[start:offset])
			start, units = offset, 0
		}
		units += size
	}
	return append(parts, text[start:])
}
