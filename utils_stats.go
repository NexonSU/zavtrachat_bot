package main

// The stat counters every message feeds, and the start-of-day timestamp they key on.

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Salvation events are stored per user/day using the same atomic counters.
const salvationStatType int64 = 9

func statsIncrease(statType int64, dayTimestamp int64, contextID int64) error {
	return statsAdd(statType, dayTimestamp, contextID, 1)
}

// statsAdd increases a stat counter by an arbitrary amount: mute time (stat
// type 7) counts seconds, not events, so a single kill can add minutes.
func statsAdd(statType int64, dayTimestamp int64, contextID int64, amount int64) error {
	row := Stats{StatType: statType, DayTimestamp: dayTimestamp, ContextID: contextID, Count: amount, LastUpdate: time.Now().Unix()}
	// One atomic statement also handles simultaneous first messages of a day.
	// An UPDATE followed by an INSERT can lose one writer at the primary key.
	// Avoid GORM's extra BEGIN/COMMIT round trips: this single-statement upsert
	// is already atomic and Stats has no associations or multi-query hooks.
	err := DB.Session(&gorm.Session{SkipDefaultTransaction: true}).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "context_id"}, {Name: "stat_type"}, {Name: "day_timestamp"}},
		DoUpdates: clause.Assignments(map[string]any{
			"count":       gorm.Expr("stats.count + ?", amount),
			"last_update": gorm.Expr("CASE WHEN stats.last_update > excluded.last_update THEN stats.last_update ELSE excluded.last_update END"),
		}),
	}).Create(&row).Error
	if err != nil && Logger != nil {
		Logger.Error("statistics update failed", "stat_type", statType, "context_id", contextID, "error", err)
	}
	return err
}

func getWordID(searchWord string) int64 {
	id, err := lookupStatsWord(searchWord)
	if err != nil && Logger != nil {
		Logger.Error("statistics word lookup failed", "error", err)
	}
	return id
}

func recordMessageWords(text string, dayTimestamp int64) error {
	text = strings.ToLower(onlyWords.ReplaceAllString(text, ""))
	counts := make(map[string]int64)
	var words []string
	for _, word := range strings.Split(text, " ") {
		if word == "" {
			continue
		}
		if counts[word] == 0 {
			words = append(words, word)
		}
		counts[word]++
	}
	// Preserve first-seen spelling in the dictionary and merge inflected forms
	// that resolve to the same existing ID before updating their counter.
	totals := make(map[int64]int64)
	var ids []int64
	for _, word := range words {
		id, err := lookupStatsWord(word)
		if err != nil {
			return err
		}
		if totals[id] == 0 {
			ids = append(ids, id)
		}
		totals[id] += counts[word]
	}
	for _, id := range ids {
		if err := statsAdd(4, dayTimestamp, id, totals[id]); err != nil {
			return err
		}
	}
	return nil
}

func GetStartOfDay() int64 {
	unixTS := time.Now().Local().Unix()
	tm := time.Unix(unixTS, 0).In(time.Local)
	hour, minute, second := tm.Clock()

	return unixTS - int64(hour*3600+minute*60+second)
}
