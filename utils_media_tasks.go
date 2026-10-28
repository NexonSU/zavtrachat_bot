package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/NexonSU/go-ytdlp"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	"golang.org/x/sync/semaphore"
)

// Bound expensive jobs independently of the dispatcher: ordinary commands
// keep working while two media jobs run and at most eight wait for a slot.
var mediaTasks = newMediaTaskQueue(2, 8, 30*time.Second, 5*time.Minute)

type mediaTaskQueue struct {
	slots                   *semaphore.Weighted
	admitted                chan struct{}
	waitTimeout, jobTimeout time.Duration
}

func newMediaTaskQueue(workers, waiting int, waitTimeout, jobTimeout time.Duration) *mediaTaskQueue {
	return &mediaTaskQueue{
		slots: semaphore.NewWeighted(int64(workers)), admitted: make(chan struct{}, workers+waiting),
		waitTimeout: waitTimeout, jobTimeout: jobTimeout,
	}
}

func (q *mediaTaskQueue) acquire(parent context.Context) (context.Context, func(), error) {
	if err := parent.Err(); err != nil {
		return nil, nil, err
	}
	select {
	case q.admitted <- struct{}{}:
	default:
		return nil, nil, fmt.Errorf("очередь обработки файлов заполнена, попробуй позже")
	}
	waitCtx, cancelWait := context.WithTimeout(parent, q.waitTimeout)
	err := q.slots.Acquire(waitCtx, 1)
	cancelWait()
	if err != nil {
		<-q.admitted
		return nil, nil, fmt.Errorf("не удалось дождаться обработки файла: %w", err)
	}
	ctx, cancel := context.WithTimeout(parent, q.jobTimeout)
	var once sync.Once
	release := func() {
		once.Do(func() {
			cancel()
			q.slots.Release(1)
			<-q.admitted
		})
	}
	return ctx, release, nil
}

func runMediaCommand(ctx context.Context, cmd *exec.Cmd) error {
	configureMediaProcess(cmd)
	cmd.WaitDelay = 2 * time.Second
	err := cmd.Run()
	if ctx.Err() != nil {
		return fmt.Errorf("превышено время обработки файла: %w", ctx.Err())
	}
	return err
}

func runMediaFFmpeg(ctx context.Context, stream *ffmpeg.Stream) error {
	cmd := exec.CommandContext(ctx, stream.FfmpegPath, stream.GetArgs()...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return runMediaCommand(ctx, cmd)
}

// Use the builder's arguments but own the process lifecycle: cancelling the
// downloader must also terminate child FFmpeg processes, not just yt-dlp.
func runMediaDownload(ctx context.Context, builder *ytdlp.Command, link string) ([]*ytdlp.ExtractedInfo, error) {
	if err := builder.GetFlagConfig().Validate(); err != nil {
		return nil, err
	}
	cmd := builder.BuildCommand(ctx, link)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := runMediaCommand(ctx, cmd); err != nil {
		return nil, fmt.Errorf("yt-dlp: %w", err)
	}
	var infos []*ytdlp.ExtractedInfo
	for _, line := range bytes.Split(stdout.Bytes(), []byte{'\n'}) {
		if !json.Valid(line) {
			continue
		}
		raw := json.RawMessage(line)
		info, err := ytdlp.ParseExtractedInfo(&raw)
		if err != nil {
			return nil, err
		}
		if info.Type != "" {
			infos = append(infos, info)
		}
	}
	return infos, nil
}
