package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func Perf(bot *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveChat == nil || ctx.EffectiveUser == nil {
		return nil
	}
	if ctx.EffectiveChat.Type != "private" {
		_, err := bot.SendMessage(ctx.EffectiveChat.Id, "Команда /perf работает только для администраторов и модераторов чата в личке с ботом.", nil)
		return err
	}
	if !IsAdminOrModer(ctx.EffectiveUser.Id) {
		_, err := bot.SendMessage(ctx.EffectiveChat.Id, "Отчёт доступен только администраторам и модераторам чата.", nil)
		return err
	}
	started, database, rows := botPerformance.snapshot()
	sortPerformanceRows(rows)
	var total operationTiming
	for _, row := range rows {
		total.Count += row.Count
		total.Errors += row.Errors
		total.Slow += row.Slow
	}
	var report strings.Builder
	fmt.Fprintf(&report, "Производительность с запуска: %s\nПериод: %s\n\nОбработчики — вызовов: %d; ошибок: %d; медленных (≥5 с): %d.\n", started.Format("02.01.2006 15:04 MST"), time.Since(started).Round(time.Second), total.Count, total.Errors, total.Slow)
	fmt.Fprintf(&report, "База — запросов: %d; ошибок: %d; медленных (≥200 мс): %d.\nСреднее: %.2f мс; максимум: %.2f мс.\n", database.Count, database.Errors, database.Slow, float64(database.average())/float64(time.Millisecond), float64(database.Max)/float64(time.Millisecond))
	if len(rows) == 0 {
		report.WriteString("\nЗавершённых обработчиков пока нет.\n")
	} else {
		report.WriteString("\nДо 10 обработчиков с наибольшим суммарным временем:\nВызовы / ошибки / медленные; среднее / максимум\n")
		for i, row := range rows {
			if i == 10 {
				break
			}
			fmt.Fprintf(&report, "%s: %d / %d / %d; %s / %s\n", row.name, row.Count, row.Errors, row.Slow, row.average().Round(time.Millisecond), row.Max.Round(time.Millisecond))
		}
	}
	report.WriteString("\nДанные сбрасываются при перезапуске. Измеряются завершённые вызовы и запросы GORM, включая служебные. Фоновые задачи после возврата обработчика не входят в его время. Ошибки, которые команда обработала сама, не входят в число ошибок обработчиков.")
	_, err := bot.SendMessage(ctx.EffectiveChat.Id, report.String(), nil)
	return err
}
