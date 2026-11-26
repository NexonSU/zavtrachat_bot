package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func runPersonalReminders(ctx context.Context, bot *gotgbot.Bot, db *gorm.DB) {
	for ctx.Err() == nil {
		if err := sendDuePersonalReminders(ctx, bot, db, time.Now()); err != nil && ctx.Err() == nil {
			slog.Error("personal reminders failed", "error", err)
		}
		timer := time.NewTimer(30 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// Pending records survive restarts, including deadlines missed while offline.
// Keep the batch bounded and lock each row across delivery and its marker, so
// concurrent schedulers skip claimed rows and cancellation serializes with send.
// Bot API has no idempotency key: a crash after send but before commit can repeat.
func sendDuePersonalReminders(ctx context.Context, bot *gotgbot.Bot, db *gorm.DB, now time.Time) error {
	var rows []PersonalReminder
	if err := pendingPersonalReminders(db.WithContext(ctx)).Where("next_attempt_at <= ?", now.Unix()).Order("next_attempt_at, id").Limit(20).Find(&rows).Error; err != nil {
		return err
	}
	var failures []error
	for _, candidate := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var row PersonalReminder
			err := pendingPersonalReminders(tx).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
				Where("id = ? AND next_attempt_at <= ?", candidate.ID, now.Unix()).First(&row).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			_, sendErr := bot.SendMessageWithContext(ctx, row.UserID, personalReminderText(row, "Напоминание"), personalReminderSendOpts(row))
			if sendErr == nil {
				return tx.Model(&row).Update("sent_at", now.Unix()).Error
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// Log only the record ID, never Telegram errors containing message
			// text or URL/token parameters. Retry transient failures after a minute.
			failures = append(failures, fmt.Errorf("reminder #%d delivery failed", row.ID))
			var apiErr *gotgbot.TelegramError
			if errors.As(sendErr, &apiErr) && (apiErr.Code == 403 || apiErr.Code == 400) {
				return tx.Model(&row).Update("failed_at", now.Unix()).Error
			}
			retry := int64(60)
			if apiErr != nil && apiErr.ResponseParams != nil && apiErr.ResponseParams.RetryAfter > retry {
				retry = apiErr.ResponseParams.RetryAfter
			}
			return tx.Model(&row).Update("next_attempt_at", now.Unix()+retry).Error
		})
		if err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
