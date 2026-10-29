package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestPerformanceMinuteWindows(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 30, 0, time.UTC)
	m := newPerformanceMetrics()
	m.started = now.Add(-2 * time.Hour)
	m.now = func() time.Time { return now }
	end := now
	for _, ago := range []int{60, 59, 15, 14, 0} {
		now = end.Add(-time.Duration(ago) * time.Minute)
		m.observeHandler("/test", time.Duration(ago+1)*time.Second, ago == 14)
		m.observeQuery(time.Duration(ago+1)*time.Millisecond, ago == 14)
	}
	now = end
	for _, tc := range []struct {
		minutes int
		count   uint64
		max     time.Duration
	}{{0, 5, 61 * time.Second}, {15, 2, 15 * time.Second}, {60, 4, 60 * time.Second}} {
		_, db, rows := m.snapshotPeriod(tc.minutes)
		if db.Count != tc.count || len(rows) != 1 || rows[0].Count != tc.count || rows[0].Errors != 1 || db.Errors != 1 || rows[0].Max != tc.max {
			t.Fatalf("window %d: db=%+v rows=%+v", tc.minutes, db, rows)
		}
		rows[0].Count = 999
	}
	now = end.Add(2 * time.Hour)
	for _, minutes := range []int{15, 60} {
		_, db, rows := m.snapshotPeriod(minutes)
		if db.Count != 0 || len(rows) != 0 {
			t.Fatalf("stale window %d", minutes)
		}
	}
	m.observeHandler("/new", time.Second, false)
	_, _, rows := m.snapshotPeriod(60)
	if len(rows) != 1 || rows[0].name != "/new" || rows[0].Count != 1 {
		t.Fatalf("bucket reuse: %+v", rows)
	}
	// Several ring rotations cannot grow the number of retained buckets.
	for i := 0; i < 180; i++ {
		now = now.Add(time.Minute)
		m.observeQuery(time.Millisecond, false)
	}
	_, db, _ := m.snapshotPeriod(60)
	if db.Count != 60 {
		t.Fatalf("ring count=%d", db.Count)
	}
}

func TestPerfPeriodCommands(t *testing.T) {
	bot, client := setupTestEnv(t)
	m := isolatePerformance(t)
	for i := 0; i < 30; i++ {
		m.observeHandler(fmt.Sprintf("/command_%02d", i), time.Second, false)
	}
	m.observeQuery(time.Millisecond, false)
	for _, arg := range []string{"", " 15", " 60"} {
		client.clearCalls()
		if err := Perf(bot, blessPrivateContext(bot, "/perf"+arg, 100)); err != nil {
			t.Fatal(err)
		}
		text := blessLastView(t, client).Params["text"].(string)
		for _, want := range []string{"Память Go", "Медиа: активно 0/2", "ожидает 0", "Горутин:", "вызовов: 30"} {
			if !strings.Contains(text, want) {
				t.Fatalf("missing %q: %s", want, text)
			}
		}
		if len([]rune(text)) > 4096 {
			t.Fatal("report exceeds Telegram limit")
		}
	}
	for _, arg := range []string{" 0", " 10", " 15 extra"} {
		if err := Perf(bot, blessPrivateContext(bot, "/perf"+arg, 100)); err != nil {
			t.Fatal(err)
		}
		if text := blessLastView(t, client).Params["text"].(string); !strings.Contains(text, "Использование:") {
			t.Fatal(text)
		}
	}
	client.clearCalls()
	ctx := blessPrivateContext(bot, "/perf 15", 100)
	ctx.EffectiveChat.Type = "supergroup"
	if err := Perf(bot, ctx); err != nil {
		t.Fatal(err)
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	for _, call := range client.Calls {
		if call.Method == "sendMessage" {
			eph, _ := call.Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
			if eph == nil || eph.ReceiverUserId != 100 {
				t.Fatal("group denial must be requester-only")
			}
		}
	}
}

func TestMediaQueueLiveCounters(t *testing.T) {
	q := newMediaTaskQueue(1, 1, time.Second, time.Second)
	check := func(active, waiting int) {
		t.Helper()
		a, w, workers, limit := q.snapshot()
		if a != active || w != waiting || workers != 1 || limit != 1 {
			t.Fatalf("queue=%d,%d,%d,%d", a, w, workers, limit)
		}
	}
	_, release, err := q.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	check(1, 0)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, r, err := q.acquire(ctx)
		if r != nil {
			r()
		}
		done <- err
	}()
	deadline := time.Now().Add(time.Second)
	for {
		_, waiting, _, _ := q.snapshot()
		if waiting == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("waiter not counted")
		}
		time.Sleep(time.Millisecond)
	}
	check(1, 1)
	if _, _, err := q.acquire(context.Background()); err == nil {
		t.Fatal("full queue admitted a task")
	}
	check(1, 1)
	cancel()
	if err := <-done; err == nil {
		t.Fatal("cancelled waiter acquired")
	}
	check(1, 0)
	release()
	release()
	check(0, 0)
	q.waitTimeout = time.Millisecond
	_, release, err = q.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := q.acquire(context.Background()); err == nil {
		t.Fatal("waiter did not time out")
	}
	check(1, 0)
	release()
	check(0, 0)
}
