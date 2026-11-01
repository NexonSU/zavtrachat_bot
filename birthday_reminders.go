package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func birthdayMember(bot *gotgbot.Bot, chatID, userID int64) (*gotgbot.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m, err := bot.GetChatMemberWithContext(ctx, chatID, userID, nil)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("empty chat member")
	}
	switch m.GetStatus() {
	case "member", "administrator", "creator":
	case "restricted":
		r, ok := m.(gotgbot.ChatMemberRestricted)
		if !ok || !r.IsMember {
			return nil, nil
		}
	case "left", "kicked":
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown chat member status")
	}
	u := m.GetUser()
	if u.Id != userID {
		return nil, fmt.Errorf("unexpected chat member identity")
	}
	return &u, nil
}

func sendBirthdayCard(bot *gotgbot.Bot, chatID int64, u gotgbot.User) error {
	avatar := quoteAvatar(context.Background(), bot, quoteContent{UserID: u.Id})
	card, err := renderBirthday(UserFullName(&u), avatar, rand.IntN(len(birthdayBackgroundNames)))
	if err != nil {
		return err
	}
	label := UserFullName(&u)
	if u.Username != "" {
		label = "@" + u.Username
	}
	caption := fmt.Sprintf("🎉 <a href=\"tg://user?id=%d\">%s</a>", u.Id, html.EscapeString(label))
	_, err = bot.SendPhoto(chatID, gotgbot.InputFileByReader("birthday.png", bytes.NewReader(card)), &gotgbot.SendPhotoOpts{Caption: caption, ParseMode: gotgbot.ParseModeHTML, RequestOpts: &gotgbot.RequestOpts{Timeout: 30 * time.Second}})
	return err
}

func runBirthdayReminders(ctx context.Context, bot *gotgbot.Bot, db *gorm.DB, chatID int64) {
	if chatID == 0 {
		return
	}
	for {
		if ctx.Err() != nil {
			return
		}
		if err := sendDueBirthdays(bot, db, chatID, time.Now()); err != nil {
			slog.Error("birthday reminders failed", "error", err)
		}
		timer := time.NewTimer(time.Until(time.Now().Truncate(time.Minute).Add(time.Minute)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func sendDueBirthdays(bot *gotgbot.Bot, db *gorm.DB, chatID int64, now time.Time) error {
	now = now.In(betMoscow)
	if chatID == 0 || now.Hour() < 10 {
		return nil
	}
	month, day := int(now.Month()), now.Day()
	q := db.Where("deleted = ? AND month = ? AND day = ?", false, month, day)
	if month == 2 && day == 28 && birthdayDate(Birthday{Month: 2, Day: 29}, now.Year()).Day() == 28 {
		q = db.Where("deleted = ? AND month = 2 AND day IN ?", false, []int{28, 29})
	}
	var rows []Birthday
	if err := q.Order("id").Find(&rows).Error; err != nil {
		return err
	}
	var failures []error
	for _, row := range rows {
		err := db.Transaction(func(tx *gorm.DB) error {
			var b Birthday
			err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("id = ? AND deleted = ?", row.ID, false).First(&b).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			// A concurrent delete/re-add may have changed the date after selection.
			if birthdayDate(b, now.Year()).Format("01-02") != now.Format("01-02") {
				return nil
			}
			if b.UserID == nil {
				var u gotgbot.User
				err := tx.Where("lower(username) = ?", strings.ToLower(b.Username)).First(&u).Error
				if errors.Is(err, gorm.ErrRecordNotFound) {
					slog.Warn("birthday identity unresolved", "birthday_id", b.ID)
					return nil
				}
				if err != nil {
					return err
				}
				b.UserID = &u.Id
				if err := tx.Model(&b).Update("user_id", u.Id).Error; err != nil {
					return err
				}
			}
			var delivery BirthdayDelivery
			err = tx.Where("user_id = ? AND year = ?", *b.UserID, now.Year()).First(&delivery).Error
			if err == nil {
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			u, err := birthdayMember(bot, chatID, *b.UserID)
			if err != nil {
				return err
			}
			if u == nil {
				return nil
			}
			if err := sendBirthdayCard(bot, chatID, *u); err != nil {
				return err
			}
			return tx.Create(&BirthdayDelivery{UserID: u.Id, Year: now.Year(), SentAt: now.Unix()}).Error
		})
		if err != nil {
			failures = append(failures, fmt.Errorf("birthday %d: %w", row.ID, err))
		}
	}
	return errors.Join(failures...)
}
