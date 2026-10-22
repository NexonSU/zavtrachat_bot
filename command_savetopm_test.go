package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func pmAlbumContext(bot *gotgbot.Bot, group string) *ext.Context {
	target := testMessage("альбом", testUser(777))
	target.MessageId, target.MediaGroupId = 61, group
	msg := testMessage("/topm", testUser(42))
	msg.Chat = gotgbot.Chat{Id: -100123, Type: "supergroup"}
	msg.ReplyToMessage = target
	return ext.NewContext(bot, &gotgbot.Update{Message: msg}, nil)
}

func seedPMAlbum(t *testing.T, group string, ids []int64) {
	t.Helper()
	ChatAlbumCache.mu.Lock()
	ChatAlbumCache.groups[group] = ids
	ChatAlbumCache.mu.Unlock()
	t.Cleanup(func() {
		ChatAlbumCache.mu.Lock()
		delete(ChatAlbumCache.groups, group)
		ChatAlbumCache.mu.Unlock()
	})
}

// A save must sort a snapshot without changing the live album, and must not
// hold the cache lock while waiting for the Telegram request to finish.
func TestSaveToPMAlbumSnapshot(t *testing.T) {
	bot, client := setupTestEnv(t)
	const group = "pm-snapshot"
	ids := make([]int64, 3, 10)
	copy(ids, []int64{63, 61, 62})
	seedPMAlbum(t, group, ids)
	client.setResult("copyMessages", json.RawMessage(`[{"message_id":999}]`))
	update := pmAlbumContext(bot, group)
	update.EffectiveMessage.MessageId, update.EffectiveMessage.MediaGroupId = 64, group
	var writer sync.WaitGroup
	client.Observe = func(method string, params map[string]any) {
		if method != "copyMessages" {
			return
		}
		done := make(chan struct{})
		writer.Add(1)
		go func() {
			defer writer.Done()
			CacheAlbumId(bot, update)
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("Telegram request blocks incoming album messages")
		}
	}
	if err := SaveToPM(bot, pmAlbumContext(bot, group)); err != nil {
		t.Fatal(err)
	}
	writer.Wait()
	calls := client.callsFor("copyMessages")
	if len(calls) != 1 || !reflect.DeepEqual(calls[0].Params["message_ids"], []int64{61, 62, 63}) {
		t.Fatalf("copied ids = %v, want the sorted original snapshot", calls)
	}
	ChatAlbumCache.mu.Lock()
	after := slices.Clone(ChatAlbumCache.groups[group])
	ChatAlbumCache.mu.Unlock()
	if !reflect.DeepEqual(after, []int64{63, 61, 62, 64}) {
		t.Fatalf("saving mutated the live album: %v", after)
	}
	if len(client.callsFor("deleteMessage")) != 1 {
		t.Error("successful save did not remove the command")
	}
}

func TestSaveToPMUncachedAlbum(t *testing.T) {
	for _, tc := range []struct {
		name string
		ids  []int64
	}{
		{"missing", nil},
		{"empty", []int64{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bot, client := setupTestEnv(t)
			group := "pm-uncached-" + tc.name
			if tc.ids != nil {
				seedPMAlbum(t, group, tc.ids)
			}
			if err := SaveToPM(bot, pmAlbumContext(bot, group)); err != nil {
				t.Fatal(err)
			}
			calls := client.callsFor("copyMessage")
			if len(calls) != 1 || calls[0].Params["chat_id"] != int64(42) || calls[0].Params["message_id"] != int64(61) || len(client.callsFor("copyMessages")) != 0 {
				t.Fatalf("uncached album did not fall back to its replied-to message: %v", calls)
			}
		})
	}
}

func TestSaveToPMAlbumCopyFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"nothing copied", nil},
		{"Telegram rejected copy", errors.New("copy failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bot, client := setupTestEnv(t)
			seedPMAlbum(t, "pm-copy-failure", []int64{61, 62})
			if tc.err != nil {
				client.setError("copyMessages", tc.err)
			}
			if err := SaveToPM(bot, pmAlbumContext(bot, "pm-copy-failure")); err == nil {
				t.Fatal("failed or empty copy must return an error instead of indexing an empty response")
			}
			if len(client.callsFor("sendMessage")) != 0 || len(client.callsFor("deleteMessage")) != 0 {
				t.Fatal("failed save sent a success link or removed the command")
			}
		})
	}
}

func TestSaveToPMConcurrentAlbumUpdates(t *testing.T) {
	bot, client := setupTestEnv(t)
	const group = "pm-concurrent"
	seedPMAlbum(t, group, append(make([]int64, 0, 10), 63, 61, 62))
	client.setResult("copyMessages", json.RawMessage(`[{"message_id":999}]`))
	// Separate keys exercise concurrent map writes; the shared album also gains
	// its remaining messages while multiple users request sorted snapshots.
	for i := 0; i < 64; i++ {
		seedPMAlbum(t, fmt.Sprintf("pm-background-%d", i), nil)
	}
	start := make(chan struct{})
	errs := make(chan error, 128)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 64; i++ {
			ctx := pmAlbumContext(bot, fmt.Sprintf("pm-background-%d", i))
			ctx.EffectiveMessage.MediaGroupId = ctx.EffectiveMessage.ReplyToMessage.MediaGroupId
			CacheAlbumId(bot, ctx)
			if i < 7 {
				ctx = pmAlbumContext(bot, group)
				ctx.EffectiveMessage.MediaGroupId = group
				ctx.EffectiveMessage.MessageId = int64(70 - i)
				CacheAlbumId(bot, ctx)
			}
			runtime.Gosched()
		}
	}()
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 4; j++ {
				if err := SaveToPM(bot, pmAlbumContext(bot, group)); err != nil {
					errs <- err
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	calls := client.callsFor("copyMessages")
	if len(calls) != 128 {
		t.Fatalf("copied %d albums, want 128", len(calls))
	}
	for _, call := range calls {
		ids := call.Params["message_ids"].([]int64)
		if !slices.IsSorted(ids) || len(ids) < 3 || len(ids) > 10 || call.Params["chat_id"] != int64(42) {
			t.Fatalf("invalid album snapshot: %v", call.Params)
		}
	}
}
