package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// A nullable Telegram ID lets the calendar import retain people not yet known
// to the bot. Once bound, identity is by ID even if a username changes hands.
type Birthday struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    *int64 `gorm:"uniqueIndex"`
	Username  string `gorm:"index"`
	Name      string
	Day       int
	Month     int
	Deleted   bool
	AddedBy   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Separate from the birthday so deleting/readding cannot resend this year's card.
type BirthdayDelivery struct {
	UserID int64 `gorm:"primaryKey;autoIncrement:false"`
	Year   int   `gorm:"primaryKey;autoIncrement:false"`
	SentAt int64
}

type BirthdayImport struct {
	Version string `gorm:"primaryKey"`
}

//go:embed assets/birthdays.json
var birthdaySeed []byte

func validBirthday(day, month int) bool {
	d := time.Date(2000, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return int(d.Month()) == month && d.Day() == day
}

func birthdayDate(b Birthday, year int) time.Time {
	day := b.Day
	if b.Month == 2 && day == 29 && time.Date(year, 3, 0, 0, 0, 0, 0, betMoscow).Day() == 28 {
		day = 28
	}
	return time.Date(year, time.Month(b.Month), day, 0, 0, 0, 0, betMoscow)
}

func nextBirthday(b Birthday, now time.Time) time.Time {
	now = now.In(betMoscow)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, betMoscow)
	d := birthdayDate(b, now.Year())
	if d.Before(today) {
		d = birthdayDate(b, now.Year()+1)
	}
	return d
}

func importBirthdays(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		marker := BirthdayImport{Version: "zavtrachat-2026-11-01"}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&marker)
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		var seeds []struct {
			Day, Month     int
			Name, Username string
		}
		if err := json.Unmarshal(birthdaySeed, &seeds); err != nil {
			return err
		}
		for _, seed := range seeds {
			if !validBirthday(seed.Day, seed.Month) {
				return fmt.Errorf("invalid birthday seed")
			}
			b := Birthday{Username: strings.ToLower(seed.Username), Name: seed.Name, Day: seed.Day, Month: seed.Month}
			var u gotgbot.User
			err := tx.Where("lower(username) = ?", b.Username).First(&u).Error
			if err == nil {
				b.UserID = &u.Id
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&b).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// An ID match takes precedence. An unbound seed can be claimed only by the
// actual sender with that username, never by an argument naming someone else.
func findOwnBirthday(db *gorm.DB, u gotgbot.User, lock bool) (Birthday, error) {
	var b Birthday
	q := db
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := q.Session(&gorm.Session{}).Where("user_id = ?", u.Id).First(&b).Error
	if errors.Is(err, gorm.ErrRecordNotFound) && u.Username != "" {
		err = q.Session(&gorm.Session{}).Where("user_id IS NULL AND username = ?", strings.ToLower(u.Username)).First(&b).Error
	}
	return b, err
}

func addBirthday(db *gorm.DB, u gotgbot.User, day, month int) (bool, error) {
	added := false
	err := db.Transaction(func(tx *gorm.DB) error {
		b, err := findOwnBirthday(tx, u, true)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			// Bind imported records even when the response is "already exists".
			if !b.Deleted {
				return tx.Model(&b).Update("user_id", u.Id).Error
			}
			added = true
			return tx.Model(&b).Updates(map[string]any{"user_id": u.Id, "day": day, "month": month, "name": UserFullName(&u), "deleted": false, "added_by": u.Id}).Error
		}
		// Empty usernames must not conflict for users without a public handle.
		key := strings.ToLower(u.Username)
		if key == "" {
			key = fmt.Sprintf("id:%d", u.Id)
		}
		b = Birthday{UserID: &u.Id, Username: key, Name: UserFullName(&u), Day: day, Month: month, AddedBy: u.Id}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&b)
		added = result.RowsAffected > 0
		return result.Error
	})
	return added, err
}

func deleteBirthday(db *gorm.DB, u gotgbot.User) (bool, error) {
	deleted := false
	err := db.Transaction(func(tx *gorm.DB) error {
		b, err := findOwnBirthday(tx, u, true)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		deleted = !b.Deleted
		return tx.Model(&b).Updates(map[string]any{"user_id": u.Id, "deleted": true}).Error
	})
	return deleted, err
}

func upcomingBirthdays(db *gorm.DB, now time.Time) ([]Birthday, error) {
	var rows []Birthday
	if err := db.Where("deleted = ?", false).Find(&rows).Error; err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := nextBirthday(rows[i], now), nextBirthday(rows[j], now)
		if a.Equal(b) {
			return rows[i].ID < rows[j].ID
		}
		return a.Before(b)
	})
	if len(rows) > 10 {
		rows = rows[:10]
	}
	return rows, nil
}
