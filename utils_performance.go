package main

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const slowHandlerThreshold = 5 * time.Second
const slowQueryThreshold = 200 * time.Millisecond

type operationTiming struct {
	Count, Errors, Slow uint64
	Total, Max          time.Duration
}

func (s *operationTiming) add(elapsed time.Duration, failed bool, threshold time.Duration) {
	s.Count++
	s.Total += elapsed
	if elapsed > s.Max {
		s.Max = elapsed
	}
	if failed {
		s.Errors++
	}
	if elapsed >= threshold {
		s.Slow++
	}
}

func (s operationTiming) average() time.Duration {
	if s.Count == 0 {
		return 0
	}
	return s.Total / time.Duration(s.Count)
}

// Keys come exclusively from registered handlers, never from incoming text.
// Retain aggregates only: no per-update records, SQL, user IDs or arguments.
type performanceMetrics struct {
	mu       sync.Mutex
	started  time.Time
	handlers map[string]operationTiming
	database operationTiming
}

var botPerformance = newPerformanceMetrics()

func newPerformanceMetrics() *performanceMetrics {
	return &performanceMetrics{started: time.Now(), handlers: make(map[string]operationTiming)}
}

func (m *performanceMetrics) observeHandler(name string, elapsed time.Duration, failed bool) {
	m.mu.Lock()
	s := m.handlers[name]
	s.add(elapsed, failed, slowHandlerThreshold)
	m.handlers[name] = s
	m.mu.Unlock()
}

func (m *performanceMetrics) observeQuery(elapsed time.Duration, failed bool) {
	m.mu.Lock()
	m.database.add(elapsed, failed, slowQueryThreshold)
	m.mu.Unlock()
}

type namedTiming struct {
	name string
	operationTiming
}

func (m *performanceMetrics) snapshot() (time.Time, operationTiming, []namedTiming) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rows := make([]namedTiming, 0, len(m.handlers))
	for name, timing := range m.handlers {
		rows = append(rows, namedTiming{name, timing})
	}
	return m.started, m.database, rows
}

// Preserve the exact result, including dispatcher flow-control sentinels. A
// panic is recorded by the defer and still reaches the dispatcher's recovery.
func measureHandler(name string, next handlers.Response) handlers.Response {
	return func(bot *gotgbot.Bot, ctx *ext.Context) (err error) {
		started := time.Now()
		completed := false
		defer func() {
			elapsed := time.Since(started)
			failed := !completed || (err != nil && !errors.Is(err, ext.EndGroups) && !errors.Is(err, ext.ContinueGroups))
			botPerformance.observeHandler(name, elapsed, failed)
			if Logger != nil && (failed || elapsed >= slowHandlerThreshold) {
				Logger.Warn("handler performance", "handler", name, "duration_ms", elapsed.Milliseconds(), "failed", failed)
			}
		}()
		err = next(bot, ctx)
		completed = true
		return err
	}
}

// Trace deliberately never calls fc: it expands SQL parameters, which can
// contain message contents or credentials. Count even when LogMode is Silent.
type performanceDBLogger struct {
	logger.Interface
	metrics *performanceMetrics
	output  *slog.Logger
	level   logger.LogLevel
}

func newPerformanceDBLogger(metrics *performanceMetrics, output *slog.Logger) logger.Interface {
	return &performanceDBLogger{Interface: logger.Discard, metrics: metrics, output: output, level: logger.Warn}
}

func (l *performanceDBLogger) LogMode(level logger.LogLevel) logger.Interface {
	clone := *l
	clone.level = level
	return &clone
}

func (l *performanceDBLogger) Trace(_ context.Context, begin time.Time, _ func() (string, int64), err error) {
	elapsed := time.Since(begin)
	failed := err != nil && !errors.Is(err, gorm.ErrRecordNotFound)
	l.metrics.observeQuery(elapsed, failed)
	if l.output == nil {
		return
	}
	if (failed && l.level >= logger.Error) || (elapsed >= slowQueryThreshold && l.level >= logger.Warn) {
		l.output.Warn("database performance", "duration_ms", elapsed.Milliseconds(), "failed", failed)
	}
}

func sortPerformanceRows(rows []namedTiming) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Total != rows[j].Total {
			return rows[i].Total > rows[j].Total
		}
		return rows[i].name < rows[j].name
	})
}
