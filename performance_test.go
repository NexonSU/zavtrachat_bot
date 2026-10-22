package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func isolatePerformance(t *testing.T) *performanceMetrics {
	t.Helper()
	old := botPerformance
	botPerformance = newPerformanceMetrics()
	t.Cleanup(func() { botPerformance = old })
	return botPerformance
}

func TestPerformanceHandlerResults(t *testing.T) {
	setupTestEnv(t)
	m := isolatePerformance(t)
	failure := errors.New("private message and token must not be logged")
	var logs bytes.Buffer
	Logger = slog.New(slog.NewTextHandler(&logs, nil))
	for i, result := range []error{nil, failure, ext.EndGroups, fmt.Errorf("wrapped: %w", ext.ContinueGroups)} {
		name := fmt.Sprintf("/handler%d", i)
		called := 0
		err := measureHandler(name, func(b *gotgbot.Bot, c *ext.Context) error { called++; return result })(Bot, nil)
		if err != result || called != 1 {
			t.Fatalf("result changed: %v", err)
		}
	}
	func() {
		defer func() {
			if recover() != "original panic" {
				t.Error("panic was swallowed or changed")
			}
		}()
		_ = measureHandler("/panic", func(*gotgbot.Bot, *ext.Context) error { panic("original panic") })(Bot, nil)
	}()
	_, _, rows := m.snapshot()
	var count, failed uint64
	for _, r := range rows {
		count += r.Count
		failed += r.Errors
	}
	if count != 5 || failed != 2 {
		t.Fatalf("count=%d errors=%d", count, failed)
	}
	if strings.Contains(logs.String(), failure.Error()) || strings.Contains(logs.String(), "original panic") {
		t.Fatal("sensitive error or panic text logged")
	}
	if !strings.Contains(logs.String(), "handler=/handler1") {
		t.Fatal("missing failure event")
	}
}

func TestPerformanceConcurrentAggregates(t *testing.T) {
	m := newPerformanceMetrics()
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			for range 1000 {
				m.observeHandler("/test", 5*time.Second, true)
				m.observeQuery(200*time.Millisecond, true)
			}
		})
	}
	wg.Wait()
	_, db, rows := m.snapshot()
	if len(rows) != 1 || rows[0].Count != 32000 || rows[0].Errors != 32000 || rows[0].Slow != 32000 || rows[0].average() != 5*time.Second || rows[0].Max != 5*time.Second {
		t.Fatalf("bad handlers: %+v", rows)
	}
	if db.Count != 32000 || db.Errors != 32000 || db.Slow != 32000 || db.average() != 200*time.Millisecond || db.Max != 200*time.Millisecond {
		t.Fatalf("bad DB: %+v", db)
	}
	rows[0].Count = 0
	_, _, fresh := m.snapshot()
	if fresh[0].Count != 32000 {
		t.Fatal("snapshot mutated metrics")
	}
}

func TestPerformanceDatabaseTrace(t *testing.T) {
	_, _ = setupTestEnv(t)
	m := newPerformanceMetrics()
	var logs bytes.Buffer
	l := newPerformanceDBLogger(m, slog.New(slog.NewTextHandler(&logs, nil)))
	sql := func() (string, int64) { t.Fatal("SQL formatting must not be called"); return "secret", 0 }
	l.Trace(context.Background(), time.Now().Add(-time.Second), sql, nil)
	l.Trace(context.Background(), time.Now(), sql, errors.New("secret DB password"))
	l.Trace(context.Background(), time.Now(), sql, gorm.ErrRecordNotFound)
	silent := l.LogMode(logger.Silent)
	before := logs.Len()
	silent.Trace(context.Background(), time.Now().Add(-time.Second), sql, errors.New("secret"))
	if logs.Len() != before {
		t.Fatal("silent mode logged")
	}
	_, db, _ := m.snapshot()
	if db.Count != 4 || db.Errors != 2 || db.Slow != 2 || db.Max < time.Second {
		t.Fatalf("bad metrics: %+v", db)
	}
	if strings.Contains(logs.String(), "secret") {
		t.Fatal("SQL/error data exposed")
	}
	// Verify actual GORM operations use Trace, including failures, without
	// modifying their returned results. Session LogMode still records metrics.
	DB = *DB.Session(&gorm.Session{Logger: silent})
	if err := DB.Create(&Bless{Text: "metric test"}).Error; err != nil {
		t.Fatal(err)
	}
	var found Bless
	if err := DB.Where("text = ?", "metric test").First(&found).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Where("text = ?", "missing text").First(&found).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("not found changed: %v", err)
	}
	if err := DB.Exec("invalid statement").Error; err == nil {
		t.Fatal("DB error hidden")
	}
	_, db, _ = m.snapshot()
	if db.Count != 8 || db.Errors != 3 {
		t.Fatalf("actual GORM queries not recorded: %+v", db)
	}
}

func TestPerfAccessAndReport(t *testing.T) {
	bot, client := setupTestEnv(t)
	m := isolatePerformance(t)
	for i := range 30 {
		m.observeHandler(fmt.Sprintf("/command_%02d", i), time.Duration(i+1)*time.Second, i == 0)
	}
	m.observeQuery(time.Second, true)
	for _, tc := range []struct {
		id      int64
		private bool
		allowed bool
	}{{100, true, true}, {300, true, true}, {999, true, false}, {100, false, false}, {999, false, false}} {
		ctx := blessPrivateContext(bot, "/perf", tc.id)
		if !tc.private {
			ctx.EffectiveChat.Type = "supergroup"
		}
		if err := Perf(bot, ctx); err != nil {
			t.Fatal(err)
		}
		text := blessLastView(t, client).Params["text"].(string)
		if strings.Contains(text, "Производительность с запуска") != tc.allowed {
			t.Fatalf("access %+v: %s", tc, text)
		}
		if tc.allowed {
			if len([]rune(text)) > 4096 {
				t.Fatal("report exceeds Telegram limit")
			}
			for _, want := range []string{"вызовов: 30; ошибок: 1", "База — запросов: 1; ошибок: 1", "/command_29", "сбрасываются при перезапуске"} {
				if !strings.Contains(text, want) {
					t.Errorf("missing %q", want)
				}
			}
			if strings.Contains(text, "/command_00") {
				t.Fatal("report is not limited to top 10")
			}
		}
	}
	if err := Perf(bot, &ext.Context{}); err != nil {
		t.Fatal(err)
	}
	botPerformance = newPerformanceMetrics()
	if err := Perf(bot, blessPrivateContext(bot, "/perf", 100)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(blessLastView(t, client).Params["text"].(string), "пока нет") {
		t.Fatal("missing empty-state report")
	}
}
