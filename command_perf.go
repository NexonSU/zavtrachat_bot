package main

import (
	"fmt"
	"runtime"
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
		return sendZBHelp(bot, ctx, "Команда /perf работает только для администраторов и модераторов чата в личке с ботом.")
	}
	if !IsAdminOrModer(ctx.EffectiveUser.Id) {
		_, err := bot.SendMessage(ctx.EffectiveChat.Id, "Отчёт доступен только администраторам и модераторам чата.", nil)
		return err
	}
	minutes := 0
	args := ctx.Args()
	if len(args) > 2 {
		_, err := bot.SendMessage(ctx.EffectiveChat.Id, "Использование: /perf, /perf 15 или /perf 60.", nil)
		return err
	}
	if len(args) == 2 {
		switch args[1] {
		case "15":
			minutes = 15
		case "60":
			minutes = 60
		default:
			_, err := bot.SendMessage(ctx.EffectiveChat.Id, "Использование: /perf, /perf 15 или /perf 60.", nil)
			return err
		}
	}
	started, database, rows := botPerformance.snapshotPeriod(minutes)
	sortPerformanceRows(rows)
	var total operationTiming
	for _, row := range rows {
		total.Count += row.Count
		total.Errors += row.Errors
		total.Slow += row.Slow
	}
	var report strings.Builder
	title := "Производительность с запуска"
	if minutes > 0 {
		title = fmt.Sprintf("Производительность за %d минут (поминутно)", minutes)
	}
	fmt.Fprintf(&report, "%s: %s\nПериод: %s\n\nОбработчики — вызовов: %d; ошибок: %d; медленных (≥5 с): %d.\n", title, started.Format("02.01.2006 15:04 MST"), time.Since(started).Round(time.Second), total.Count, total.Errors, total.Slow)
	fmt.Fprintf(&report, "База — запросов: %d; ошибок: %d; медленных (≥200 мс): %d.\nСреднее: %.2f мс; максимум: %.2f мс.\n", database.Count, database.Errors, database.Slow, float64(database.average())/float64(time.Millisecond), float64(database.Max)/float64(time.Millisecond))
	if minutes == 0 {
		for _, window := range []int{15, 60} {
			_, db, recent := botPerformance.snapshotPeriod(window)
			var calls, errors uint64
			for _, row := range recent {
				calls += row.Count
				errors += row.Errors
			}
			fmt.Fprintf(&report, "\nЗа %d мин (поминутно): вызовов %d, ошибок %d; SQL %d, среднее %.2f мс.\n", window, calls, errors, db.Count, float64(db.average())/float64(time.Millisecond))
		}
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	active, waiting, workers, waitingLimit := mediaTasks.snapshot()
	fmt.Fprintf(&report, "\nПамять Go: занято %.1f МиБ; получено от ОС %.1f МиБ.\nГорутин: %d. Медиа: активно %d/%d; ожидает %d (лимит очереди %d).\n", float64(mem.Alloc)/(1<<20), float64(mem.Sys)/(1<<20), runtime.NumGoroutine(), active, workers, waiting, waitingLimit)
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
	report.WriteString("\nДанные сбрасываются при перезапуске. Окна 15/60 мин включают текущую неполную и 14/59 предыдущих минут по времени завершения. Память Go не равна всей памяти процесса. /perf 15 и /perf 60 — подробности. Измеряются завершённые вызовы и запросы GORM, включая служебные. Фоновые задачи после возврата обработчика не входят в его время. Ошибки, которые команда обработала сама, не входят в число ошибок обработчиков.")
	_, err := bot.SendMessage(ctx.EffectiveChat.Id, report.String(), nil)
	return err
}
