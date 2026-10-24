package main

import (
	"sync"
	"testing"
	"time"
)

func TestAntispamSlowReportDoesNotBlockOtherUpdates(t *testing.T) {
	bot, client := setupTestEnv(t)
	m := NewGlobalMiddleware()
	for i := 0; i < Config.AntiSpamCount; i++ {
		if err := m.HandleUpdate(bot, testContext(bot, "привет", testUser(42))); err != nil {
			t.Fatal(err)
		}
	}

	started := make(chan struct{})
	release := make(chan struct{})
	var startOnce, releaseOnce sync.Once
	client.Observe = func(method string, _ map[string]any) {
		if method == "sendMessage" {
			startOnce.Do(func() { close(started) })
			<-release // Simulate a Telegram request that has not returned yet.
		}
	}
	var workers sync.WaitGroup
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		workers.Wait() // Finish before setupTestEnv restores the globals.
	})
	launch := func(userID int64, done chan<- error) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			done <- m.HandleUpdate(bot, testContext(bot, "привет", testUser(userID)))
		}()
	}
	reportDone := make(chan error, 1)
	launch(42, reportDone)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("spam report did not start")
	}

	otherUpdates := make(chan error, 2)
	launch(42, otherUpdates) // Same user's cooldown must already be reserved.
	launch(43, otherUpdates)
	for range 2 {
		select {
		case err := <-otherUpdates:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("another update waited for the blocked spam report")
		}
	}

	m.mu.Lock()
	sameUserCount, otherUserCount := len(m.timestamps[42]), len(m.timestamps[43])
	m.mu.Unlock()
	if sameUserCount != Config.AntiSpamCount+2 || otherUserCount != 1 {
		t.Fatalf("recorded updates = %d/%d, want %d/1", sameUserCount, otherUserCount, Config.AntiSpamCount+2)
	}
	releaseOnce.Do(func() { close(release) })
	if err := <-reportDone; err != nil {
		t.Fatal(err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("spam reports = %d, want one despite concurrent updates", len(calls))
	}
}

func TestAntispamConcurrentUpdatesKeepOneReport(t *testing.T) {
	bot, client := setupTestEnv(t)
	m := NewGlobalMiddleware()
	const count = 64
	start := make(chan struct{})
	results := make(chan error, count)
	var workers sync.WaitGroup
	for range count {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			results <- m.HandleUpdate(bot, testContext(bot, "привет", testUser(42)))
		}()
	}
	close(start)
	workers.Wait()
	for range count {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if n := len(m.timestamps[42]); n != count {
		t.Fatalf("recorded updates = %d, want %d", n, count)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("spam reports = %d, want one", len(calls))
	}
}

func TestAntispamThresholdAndCooldownBoundaries(t *testing.T) {
	setupTestEnv(t)
	m := NewGlobalMiddleware()
	base := time.Unix(1800000000, 0)
	for i := 0; i < Config.AntiSpamCount; i++ {
		if m.recordAntispamUpdate(42, base) {
			t.Fatalf("reported before %d earlier updates", Config.AntiSpamCount)
		}
	}
	if !m.recordAntispamUpdate(42, base) {
		t.Fatal("did not report after crossing the threshold")
	}
	if m.recordAntispamUpdate(42, base.Add(10*time.Second)) {
		t.Fatal("reported at the inclusive ten-second cooldown boundary")
	}
	if !m.recordAntispamUpdate(42, base.Add(11*time.Second)) {
		t.Fatal("did not report after the cooldown expired")
	}
}

func TestAntispamCleanupPreservesActiveWindowAndCooldown(t *testing.T) {
	setupTestEnv(t)
	m := NewGlobalMiddleware()
	base := time.Unix(1800000000, 0)
	m.timestamps[41] = []time.Time{base.Add(-100 * time.Second)}
	m.lastReportAt[41] = base.Unix() - 100
	m.timestamps[42] = []time.Time{base.Add(-5 * time.Second)}
	m.lastReportAt[42] = base.Unix() - 1
	// The existing rule truncates seconds before comparing to the window:
	// at 30.9 seconds a timestamp is still valid; at 31 seconds it expires.
	m.timestamps[43] = []time.Time{base.Add(-30900 * time.Millisecond)}
	m.timestamps[44] = []time.Time{base.Add(-31 * time.Second)}
	m.timestamps[45] = []time.Time{base.Add(-100 * time.Second)}
	m.lastReportAt[45] = base.Unix() - 10
	if m.recordAntispamUpdate(99, base) {
		t.Fatal("new user unexpectedly triggered a report")
	}
	for _, id := range []int64{41, 44, 45} {
		if _, exists := m.timestamps[id]; exists {
			t.Errorf("expired user %d was retained", id)
		}
	}
	if _, exists := m.lastReportAt[41]; exists {
		t.Error("expired report was retained")
	}
	for _, id := range []int64{42, 43, 99} {
		if len(m.timestamps[id]) != 1 {
			t.Errorf("active user %d was pruned", id)
		}
	}
	if m.lastReportAt[42] != base.Unix()-1 || m.lastReportAt[45] != base.Unix()-10 {
		t.Error("cleanup discarded a report still inside the cooldown")
	}

	// Reclamation runs at most once per minute, then removes users who
	// stopped sending updates without needing another update from them.
	m.timestamps[46] = []time.Time{base.Add(-100 * time.Second)}
	m.recordAntispamUpdate(99, base.Add(30*time.Second))
	if _, exists := m.timestamps[46]; !exists {
		t.Fatal("cleanup ran again before its interval")
	}
	m.recordAntispamUpdate(99, base.Add(antispamCleanupInterval))
	for _, id := range []int64{42, 43, 46} {
		if _, exists := m.timestamps[id]; exists {
			t.Errorf("idle user %d survived the next cleanup", id)
		}
	}
	if len(m.lastReportAt) != 0 {
		t.Error("old report cooldown entries were not reclaimed")
	}
}

func TestAntispamCleanupDoesNotResetShortWindowCooldown(t *testing.T) {
	setupTestEnv(t)
	Config.AntiSpamWindow = 0
	Config.AntiSpamCount = 0
	m := NewGlobalMiddleware()
	base := time.Unix(1800000000, 0)
	m.timestamps[42] = []time.Time{base.Add(-2 * time.Second)}
	m.lastReportAt[42] = base.Unix() - 2
	if m.recordAntispamUpdate(42, base) {
		t.Fatal("cleanup reset the cooldown when a shorter message window expired")
	}
	if !m.recordAntispamUpdate(42, base.Add(9*time.Second)) {
		t.Fatal("cooldown did not expire after eleven seconds")
	}
}
