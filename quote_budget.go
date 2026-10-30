package main

import (
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// A single shared calendar-hour budget, persisted across bot restarts.
type QuoteBudget struct {
	Hour int64 `gorm:"primaryKey;autoIncrement:false"`
	Used int
}

func reserveQuote(now time.Time) (int, int64, error) {
	hour := now.Unix() / 3600
	row := QuoteBudget{Hour: hour, Used: 1}
	result := DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "hour"}}, DoUpdates: clause.Assignments(map[string]interface{}{"used": gorm.Expr("quote_budgets.used + 1")}), Where: clause.Where{Exprs: []clause.Expression{clause.Lt{Column: clause.Column{Table: "quote_budgets", Name: "used"}, Value: 5}}}}, clause.Returning{}).Create(&row)
	if result.Error != nil {
		return 0, hour, result.Error
	}
	if result.RowsAffected == 0 {
		return -1, hour, nil
	}
	return 5 - row.Used, hour, nil
}
func quoteBudgetText(left int, hour int64) string {
	zone, _ := time.LoadLocation("Europe/Moscow")
	until := time.Unix((hour+1)*3600, 0).In(zone).Format("15:04")
	if left < 0 {
		return "Общий лимит цитат исчерпан. Новые цитаты будут доступны в " + until + "."
	}
	word := "цитат"
	if left == 1 {
		word = "цитата"
	} else if left >= 2 && left <= 4 {
		word = "цитаты"
	}
	return fmt.Sprintf("Еще %d %s до %s", left, word, until)
}
