package main

// FFmpegConvert is the conversion path every media effect goes through: /convert, /distort,
// /invert and /loop all end up here with a target type.

import (
	cntx "context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	ffprobe "gopkg.in/vansante/go-ffprobe.v2"
)

func FFmpegConvert(context *ext.Context, media Media, targetType string) error {
	jobCtx, release, err := mediaTasks.acquire(cntx.Background())
	if err != nil {
		return err
	}
	defer release()
	var KwArgs ffmpeg.KwArgs
	var extension string
	var height int
	var width int
	var duration float64

	videoKwArgs := ffmpeg.KwArgs{}
	defaultKwArgs := ffmpeg.KwArgs{"loglevel": "fatal", "hide_banner": ""}
	name := media.FileName

	if _, err := os.Stat(media.FilePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			workdir, err := os.MkdirTemp(os.TempDir(), "zavtra-convert-*")
			if err != nil {
				return err
			}
			defer os.RemoveAll(workdir)
			remotePath := media.FilePath
			media.FilePath = filepath.Join(workdir, "input")
			err = DownloadFile(media.FilePath, fmt.Sprintf("https://api.telegram.org/file/bot%v/%v", Config.Token, remotePath))
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	probeCtx, cancelProbe := cntx.WithTimeout(jobCtx, 5*time.Second)
	defer cancelProbe()
	data, err := ffprobe.ProbeURL(probeCtx, media.FilePath)
	if err != nil {
		return err
	}
	inputVideoFormat := data.FirstVideoStream()
	inputAudioFormat := data.FirstAudioStream()

	if inputVideoFormat != nil {
		width = inputVideoFormat.Width
		height = inputVideoFormat.Height
		duration, err = strconv.ParseFloat(inputVideoFormat.Duration, 32)
		if err != nil {
			duration = 0
		}
	} else {
		switch targetType {
		case "animation", "gif", "webm", "animation_reverse", "sticker_reverse", "animation_loop", "loop":
			return fmt.Errorf("видео-дорожка не найдена")
		case "video", "mp4":
			targetType = "audio"
		case "video_reverse", "reverse", "invert":
			targetType = "audio_reverse"
		}
	}

	if inputAudioFormat != nil {
		if duration == 0 {
			duration, err = strconv.ParseFloat(inputAudioFormat.Duration, 32)
			if err != nil {
				duration = 0
			}
		}
	} else {
		switch targetType {
		case "audio", "mp3", "voice", "ogg", "audio_reverse", "voice_reverse", "audio_loop", "voice_loop":
			return fmt.Errorf("аудио-дорожка не найдена")
		case "video", "mp4", "webm":
			targetType = "animation"
		case "video_reverse", "reverse", "invert":
			targetType = "animation_reverse"
		case "video_loop", "loop":
			targetType = "animation_loop"
		}
	}

	if (strings.Contains(targetType, "reverse") || strings.Contains(targetType, "loop")) && duration > 120 {
		return fmt.Errorf("слишком длинное видео для эффекта")
	}

	if inputAudioFormat == nil && inputVideoFormat == nil {
		return fmt.Errorf("медиа-дорожек не найдено")
	}

	switch targetType {
	case "audio", "mp3":
		KwArgs = ffmpeg.KwArgs{"vn": "", "c:a": "libmp3lame"}
		extension = "mp3"
		targetType = "audio"
	case "voice", "ogg":
		KwArgs = ffmpeg.KwArgs{"vn": "", "c:a": "libopus"}
		extension = "ogg"
		targetType = "voice"
	case "photo", "jpg":
		KwArgs = ffmpeg.KwArgs{"vf": "pad=ceil(iw/2)*2:ceil(ih/2)*2, select=eq(n\\,0)", "format": "image2"}
		extension = "jpg"
		targetType = "photo"
	case "sticker", "webp":
		KwArgs = ffmpeg.KwArgs{"vf": "pad=ceil(iw/2)*2:ceil(ih/2)*2, select=eq(n\\,0)", "format": "image2"}
		extension = "webp"
		targetType = "sticker"
	case "animation", "gif":
		KwArgs = ffmpeg.MergeKwArgs([]ffmpeg.KwArgs{videoKwArgs, {"an": ""}})
		extension = "mp4"
		targetType = "animation"
	case "video", "mp4":
		KwArgs = ffmpeg.MergeKwArgs([]ffmpeg.KwArgs{videoKwArgs, {"c:a": "aac"}})
		extension = "mp4"
		targetType = "video"
	case "video_reverse", "reverse", "invert":
		KwArgs = ffmpeg.MergeKwArgs([]ffmpeg.KwArgs{videoKwArgs, {"c:a": "aac", "vf": "pad=ceil(iw/2)*2:ceil(ih/2)*2, reverse", "af": "areverse"}})
		extension = "mp4"
		targetType = "video"
	case "animation_reverse":
		KwArgs = ffmpeg.MergeKwArgs([]ffmpeg.KwArgs{videoKwArgs, {"an": "", "vf": "pad=ceil(iw/2)*2:ceil(ih/2)*2, reverse"}})
		extension = "mp4"
		targetType = "animation"
	case "sticker_reverse", "webm":
		KwArgs = ffmpeg.KwArgs{"c:v": "libvpx-vp9", "an": "", "vf": "pad=ceil(iw/2)*2:ceil(ih/2)*2, reverse"}
		extension = "mp4"
		targetType = "sticker"
	case "audio_reverse":
		KwArgs = ffmpeg.KwArgs{"vn": "", "c:a": "libmp3lame", "af": "areverse"}
		extension = "mp3"
		targetType = "audio"
	case "voice_reverse":
		KwArgs = ffmpeg.KwArgs{"vn": "", "c:a": "libopus", "af": "areverse"}
		extension = "ogg"
		targetType = "voice"
	case "animation_loop", "loop":
		KwArgs = ffmpeg.MergeKwArgs([]ffmpeg.KwArgs{videoKwArgs, {"an": "", "filter_complex": "[0]reverse[r];[0][r]concat,loop=1:2,setpts=PTS-STARTPTS,pad=ceil(iw/2)*2:ceil(ih/2)*2"}})
		extension = "mp4"
		targetType = "animation"
	default:
		return fmt.Errorf("targetType %v not supported", targetType)
	}

	// The result name is unique per conversion: two conversions of the same
	// file in the same second used to write one and the same result file, and
	// the first to finish removed the file the other one was about to send.
	base := strings.ReplaceAll(filepath.Base(name), "*", "")
	if base == "" {
		base = "converted"
	}
	tmpFile, err := os.CreateTemp(os.TempDir(), base+"_*_converted."+extension)
	if err != nil {
		return err
	}
	resultFilePath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(resultFilePath)

	err = runMediaFFmpeg(jobCtx, ffmpeg.Input(media.FilePath).Output(resultFilePath, ffmpeg.MergeKwArgs([]ffmpeg.KwArgs{defaultKwArgs, KwArgs})).OverWriteOutput())
	if err != nil {
		return err
	}

	resultFile, err := os.Open(resultFilePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer resultFile.Close()

	inputFile := gotgbot.InputFileByReader(media.FileName, resultFile)

	switch targetType {
	case "video":
		_, err := Bot.SendVideo(context.Message.Chat.Id, inputFile, &gotgbot.SendVideoOpts{
			SupportsStreaming: true,
			Width:             int64(width),
			Height:            int64(height),
			Duration:          int64(duration),
			ReplyParameters:   &gotgbot.ReplyParameters{MessageId: context.Message.MessageId, AllowSendingWithoutReply: true},
		})
		return err
	case "animation":
		_, err := Bot.SendAnimation(context.Message.Chat.Id, inputFile, &gotgbot.SendAnimationOpts{
			Width:           int64(width),
			Height:          int64(height),
			Duration:        int64(duration),
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.MessageId, AllowSendingWithoutReply: true},
		})
		return err
	case "audio":
		_, err := Bot.SendAudio(context.Message.Chat.Id, inputFile, &gotgbot.SendAudioOpts{
			Duration:        int64(duration),
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.MessageId, AllowSendingWithoutReply: true},
		})
		return err
	case "voice":
		_, err := Bot.SendVoice(context.Message.Chat.Id, inputFile, &gotgbot.SendVoiceOpts{
			Duration:        int64(duration),
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.MessageId, AllowSendingWithoutReply: true},
		})
		return err
	default:
		_, err := Bot.SendDocument(context.Message.Chat.Id, inputFile, &gotgbot.SendDocumentOpts{
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.MessageId, AllowSendingWithoutReply: true},
		})
		return err
	}
}
