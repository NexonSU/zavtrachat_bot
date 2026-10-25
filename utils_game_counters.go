package main

import (
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Serialize warning read/decay/write and blessing burst state. Counter upserts
// below also protect against updates from other game handlers/processes.
var warningLocks [64]sync.Mutex
var blessingStateMu sync.Mutex

func warningAmountAt(warn Warn, now time.Time) int {
	weeks := int(now.Sub(warn.LastWarn) / (7 * 24 * time.Hour))
	return max(0, warn.Amount-max(0, weeks))
}

func addDuelistCounter(userID int64, column string) (Duelist, error) {
	return addDuelistCounterDB(&DB, userID, column)
}

func addDuelistCounterDB(db *gorm.DB, userID int64, column string) (Duelist, error) {
	row := Duelist{UserID: userID}
	if column == "deaths" {
		row.Deaths = 1
	} else {
		row.Kills = 1
	}
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{column: gorm.Expr("COALESCE(duelists." + column + ", 0) + 1")}),
	}, clause.Returning{}).Create(&row)
	return row, result.Error
}

func dayBounds(now time.Time) (time.Time, time.Time) {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return start, start.AddDate(0, 0, 1)
}

// The lifetime and daily death counters must either both advance or neither.
// Telegram calls happen afterwards, outside this short transaction.
func recordGameDeath(userID int64) (Duelist, error) {
	var row Duelist
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		row, err = addDuelistCounterDB(tx, userID, "deaths")
		if err != nil {
			return err
		}
		return statsAddDB(tx, 8, GetStartOfDay(), userID, 1)
	})
	return row, err
}
