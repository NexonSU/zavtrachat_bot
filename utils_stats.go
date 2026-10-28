package main

// The stat counters every message feeds, and the start-of-day timestamp they key on.

import (
	"sort"
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
	return statsAddDB(&DB, statType, dayTimestamp, contextID, amount)
}

func statsAddDB(db *gorm.DB, statType int64, dayTimestamp int64, contextID int64, amount int64) error {
	return statsAddBatchDB(db, []Stats{{StatType: statType, DayTimestamp: dayTimestamp, ContextID: contextID, Count: amount}})
}

// One statement commits all of a message's counters together. Merge duplicate
// keys before PostgreSQL's upsert, and lock rows in primary-key order so
// overlapping batches cannot deadlock by taking the same locks in reverse.
func statsAddBatchDB(db *gorm.DB, updates []Stats) error {
	if len(updates) == 0 {
		return nil
	}
	indices := make(map[[3]int64]int, len(updates))
	rows := make([]Stats, 0, len(updates))
	now := time.Now().Unix()
	for _, row := range updates {
		key := [3]int64{row.ContextID, row.StatType, row.DayTimestamp}
		if i, ok := indices[key]; ok {
			rows[i].Count += row.Count
			continue
		}
		row.LastUpdate = now
		indices[key] = len(rows)
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.ContextID != b.ContextID {
			return a.ContextID < b.ContextID
		}
		if a.StatType != b.StatType {
			return a.StatType < b.StatType
		}
		return a.DayTimestamp < b.DayTimestamp
	})
	// This single-statement upsert is already atomic; Stats has no associations
	// or multi-query hooks requiring GORM's extra BEGIN/COMMIT round trips.
	var dest any = &rows
	if len(rows) == 1 {
		// Preserve the single-row callback contract used by game counters.
		dest = &rows[0]
	}
	err := db.Session(&gorm.Session{SkipDefaultTransaction: true}).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "context_id"}, {Name: "stat_type"}, {Name: "day_timestamp"}},
		DoUpdates: clause.Assignments(map[string]any{
			"count":       gorm.Expr("stats.count + excluded.count"),
			"last_update": gorm.Expr("CASE WHEN stats.last_update > excluded.last_update THEN stats.last_update ELSE excluded.last_update END"),
		}),
	}).Create(dest).Error
	if err != nil && Logger != nil {
		Logger.Error("statistics update failed", "counters", len(rows), "error", err)
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
	rows, err := messageWordStats(text, dayTimestamp)
	if err != nil {
		return err
	}
	return statsAddBatchDB(&DB, rows)
}

func messageWordStats(text string, dayTimestamp int64) ([]Stats, error) {
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
			return nil, err
		}
		if totals[id] == 0 {
			ids = append(ids, id)
		}
		totals[id] += counts[word]
	}
	rows := make([]Stats, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, Stats{StatType: 4, DayTimestamp: dayTimestamp, ContextID: id, Count: totals[id]})
	}
	return rows, nil
}

func GetStartOfDay() int64 {
	unixTS := time.Now().Local().Unix()
	tm := time.Unix(unixTS, 0).In(time.Local)
	hour, minute, second := tm.Clock()

	return unixTS - int64(hour*3600+minute*60+second)
}
