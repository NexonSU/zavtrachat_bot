package main

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// A single shared calendar-hour budget, persisted across bot restarts.
type QuoteBudget struct {
	Hour int64 `gorm:"primaryKey;autoIncrement:false"`
	Used int
}

func reserveQuote(now time.Time) (int, int64, error) {
	return reserveCard(now, "quote_budgets", 7)
}

type SweetBudget QuoteBudget

func reserveCard(now time.Time, table string, limit int) (int, int64, error) {
	hour := now.Unix() / 3600
	row := QuoteBudget{Hour: hour, Used: 1}
	column := clause.Column{Table: table, Name: "used"}
	result := DB.Table(table).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hour"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"used": gorm.Expr("? + 1", column)}),
		Where:     clause.Where{Exprs: []clause.Expression{clause.Lt{Column: column, Value: limit}}},
	}, clause.Returning{}).Create(&row)
	if result.Error != nil {
		return 0, hour, result.Error
	}
	if result.RowsAffected == 0 {
		return -1, hour, nil
	}
	return limit - row.Used, hour, nil
}
func quoteBudgetText(left int, hour int64) string {
	until := cardBudgetResetTime(hour)
	return "Лимит цитат в час исчерпан. Новые цитаты будут доступны в " + until + "."
}

func cardBudgetResetTime(hour int64) string {
	zone, _ := time.LoadLocation("Europe/Moscow")
	return time.Unix((hour+1)*3600, 0).In(zone).Format("15:04")
}
