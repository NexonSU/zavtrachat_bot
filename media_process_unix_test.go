//go:build unix

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/NexonSU/go-ytdlp"
	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestFFmpegDeadlineCleansOutputAndPreservesSource(t *testing.T) {
	bot, client := setupTestEnv(t)
	source := ffmpegMedia(t, "video")
	fakeMediaFile(t, client, source)
	// ffprobe stays real; only the expensive conversion is deliberately stuck.
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "ffmpeg"), []byte("#!/bin/sh\nexec sleep 30\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	old := mediaTasks
	mediaTasks = newMediaTaskQueue(1, 1, time.Second, 400*time.Millisecond)
	t.Cleanup(func() { mediaTasks = old })
	if err := Convert(bot, testReplyContext(bot, "/convert gif", testUser(42),
		&gotgbot.Message{Video: &gotgbot.Video{FileId: "timeout"}})); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("conversion deadline lost: %v", err)
	}
	files, err := filepath.Glob(filepath.Join(os.TempDir(), filepath.Base(source)+"_*_converted.*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("failed conversion leaked outputs: %v %v", files, err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("conversion removed the shared source: %v", err)
	}
	if len(mediaTasks.admitted) != 0 || len(client.callsFor("sendAnimation")) != 0 {
		t.Fatal("timed-out job retained its slot or sent a partial file")
	}
}

func TestMediaDeadlineKillsChildProcesses(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "orphan")
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	// The child would write after the parent's timeout if only the parent died.
	cmd := exec.CommandContext(ctx, "sh", "-c", `(sleep 0.5; printf orphan > "$1") & wait`, "sh", marker)
	if err := runMediaCommand(ctx, cmd); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline was not propagated: %v", err)
	}
	time.Sleep(550 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("cancelled job left a running child: %v", err)
	}
}

func TestMediaDownloadDeadlineAndMissingTool(t *testing.T) {
	script := filepath.Join(t.TempDir(), "yt-dlp")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexec sleep 30\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := runMediaDownload(ctx, ytdlp.New().SetExecutable(script).PrintJSON(), "unused"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("downloader did not stop at its deadline: %v", err)
	}
	if _, err := runMediaDownload(context.Background(), ytdlp.New().SetExecutable(script+"-missing"), "unused"); err == nil {
		t.Fatal("missing downloader was accepted")
	}
}
