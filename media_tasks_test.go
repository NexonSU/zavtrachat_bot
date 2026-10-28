package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMediaQueueBoundsAndRecovery(t *testing.T) {
	q := newMediaTaskQueue(2, 1, time.Second, time.Minute)
	_, first, err := q.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, second, err := q.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer first()
	defer second()
	waiting, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, release, err := q.acquire(waiting)
		if release != nil {
			release()
		}
		result <- err
	}()
	deadline := time.Now().Add(time.Second)
	for len(q.admitted) != 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(q.admitted) != 3 {
		t.Fatal("waiting job did not enter queue")
	}
	if _, _, err := q.acquire(context.Background()); err == nil {
		t.Fatal("full queue accepted another job")
	}
	// Ordinary commands must remain independent of the heavy-job queue.
	bot, _ := setupTestEnv(t)
	if err := Ping(bot, testContext(bot, "/ping", testUser(42))); err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting cancellation lost: %v", err)
	}
	first()
	first() // Repeated cleanup must not release another running job's slot.
	ctx, release, err := q.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
	if ctx.Err() != context.Canceled || len(q.admitted) != 1 {
		t.Fatal("release did not cancel the job or leaked a slot")
	}
}

func TestMediaQueueConcurrentAndTimeouts(t *testing.T) {
	q := newMediaTaskQueue(2, 30, time.Second, time.Minute)
	var active, peak atomic.Int64
	var wg sync.WaitGroup
	for range 24 {
		wg.Go(func() {
			_, release, err := q.acquire(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			n := active.Add(1)
			for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
			}
			time.Sleep(time.Millisecond)
			active.Add(-1)
			release()
		})
	}
	wg.Wait()
	if peak.Load() != 2 || len(q.admitted) != 0 {
		t.Fatalf("parallel limit or cleanup failed: peak=%d queued=%d", peak.Load(), len(q.admitted))
	}
	q = newMediaTaskQueue(1, 1, 10*time.Millisecond, 40*time.Millisecond)
	ctx, release, err := q.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, _, err := q.acquire(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiting timeout lost: %v", err)
	}
	select {
	case <-ctx.Done():
		if ctx.Err() != context.DeadlineExceeded {
			t.Fatal(ctx.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("job has no deadline")
	}
}
