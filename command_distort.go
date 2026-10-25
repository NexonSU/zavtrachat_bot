package main

import (
	cntx "context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	_ "image/png"

	"github.com/Jeffail/tunny"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	ffprobe "gopkg.in/vansante/go-ffprobe.v2"
)

var DistortBusy bool

var DistortCache map[string]string

// DistortTempDir is the parent of the distort work directories. It is a
// package variable so tests can point the work directories at their own temp
// dir instead of sharing the system one with every other process.
var DistortTempDir = os.TempDir()

// Distort given file
func Distort(bot *gotgbot.Bot, context *ext.Context) error {
	dir := fmt.Sprintf("%v/telegram-go-chatbot-distort", DistortTempDir)
	if err := os.Mkdir(dir, os.ModePerm); err != nil {
		os.RemoveAll(dir)
		os.Mkdir(dir, os.ModePerm)
	}
	if DistortCache == nil {
		DistortCache = make(map[string]string)
	}

	if context.Message.ReplyToMessage == nil {
		return Reply("Пример использования: <code>/distort</code> в ответ на какое-либо сообщение с видео.", *context)
	}
	if !IsContainsMedia(context.Message.ReplyToMessage) {
		return Reply("Какого-либо видео нет в указанном сообщении.", *context)
	}

	media, err := GetMedia(context.Message.ReplyToMessage)
	if err != nil {
		return err
	}
	var additionalInputArgs []string
	options := &gotgbot.ReplyParameters{AllowSendingWithoutReply: true}
	var resultMessage *gotgbot.Message
	var recepient int64

	ChatMember, err := Bot.GetChatMember(context.Message.Chat.Id, context.Message.From.Id, nil)
	if err != nil {
		return err
	}
	if time.Now().Local().Hour() > 21 || time.Now().Local().Hour() < 7 || ChatMember.GetStatus() == "administrator" || ChatMember.GetStatus() == "creator" {
		recepient = context.Message.Chat.Id
		options = &gotgbot.ReplyParameters{MessageId: context.Message.MessageId, AllowSendingWithoutReply: true}
	} else {
		recepient = context.Message.From.Id
	}

	if fileId, ok := DistortCache[media.FileID]; ok {
		_, err = Bot.SendDocument(recepient, gotgbot.InputFileByID(fileId), &gotgbot.SendDocumentOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{}})
		if recepient == context.Message.From.Id {
			Reply("Результат отправлен в личку. Если не пришло, то нужно написать что-нибудь в личку @zavtrachat_bot.", *context)
		}
		return err
	}

	switch media.Type {
	case "video", "animation", "photo", "audio", "voice", "sticker":
		break
	default:
		return Reply("Неподдерживаемая операция", *context)
	}

	if DistortBusy {
		return Reply("Команда занята", *context)
	}

	var done = make(chan bool, 1)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				context.EffectiveChat.SendAction(bot, gotgbot.ChatActionUploadDocument, nil)
			}
			time.Sleep(time.Second * 5)
		}
	}()
	defer func() {
		DistortBusy = false
		done <- true
	}()
	DistortBusy = true

	jobStarted := time.Now().Unix()

	workdir := fmt.Sprintf("%v/telegram-go-chatbot-distort/%v", DistortTempDir, media.FileID)
	inputFile := media.FilePath
	outputFile := fmt.Sprintf("%v/output.mp4", workdir)

	_, err = os.Stat(inputFile)
	if err != nil {
		inputFile = media.FileURL
	}

	ctx, cancelFn := cntx.WithTimeout(cntx.Background(), 5*time.Second)
	defer cancelFn()

	data, err := ffprobe.ProbeURL(ctx, inputFile)
	if err != nil {
		return err
	}

	framerate := "30/1"

	if media.Type != "audio" && media.Type != "voice" {
		if data.FirstVideoStream() == nil {
			return fmt.Errorf("видео-дорожка не найдена")
		}
		frames := data.FirstVideoStream().NbFrames
		framerate = data.FirstVideoStream().AvgFrameRate

		if frames == "" {
			frames = "1"
		}

		framesInt, err := strconv.Atoi(frames)
		if err != nil {
			return err
		}

		if framesInt > 1000 && !IsAdminOrModer(context.Message.From.Id) {
			return Reply("Видео слишком длинное. Максимум 1000 фреймов.", *context)
		}
	}

	if err := os.Mkdir(workdir, os.ModePerm); err != nil {
		return Reply("Обработка файла уже выполняется", *context)
	}
	defer func(workdir string) {
		os.RemoveAll(workdir)
	}(workdir)

	if media.Type == "video" && data.FirstAudioStream() != nil {
		if err = ffmpeg.Input(inputFile).Output(workdir+"/input_audio.aac", ffmpeg.KwArgs{"vn": "", "acodec": "copy"}).OverWriteOutput().ErrorToStdOut().Run(); err != nil {
			return err
		}
		if err = ffmpeg.Input(workdir+"/input_audio.aac").Output(workdir+"/audio.aac", ffmpeg.KwArgs{"filter_complex": "vibrato=f=10:d=0.7"}).OverWriteOutput().ErrorToStdOut().Run(); err != nil {
			return err
		}
		additionalInputArgs = []string{"-i", workdir + "/audio.aac", "-c:a", "aac"}
	}

	if media.Type == "audio" || media.Type == "voice" {
		if err = ffmpeg.Input(inputFile).Output(workdir+"/input_audio.mp3", ffmpeg.KwArgs{"vn": ""}).OverWriteOutput().ErrorToStdOut().Run(); err != nil {
			return err
		}
		err = ffmpeg.Input(workdir+"/input_audio.mp3").Output(workdir+"/audio.mp3", ffmpeg.KwArgs{"filter_complex": "vibrato=f=10:d=0.7"}).OverWriteOutput().ErrorToStdOut().Run()
		if err != nil {
			return err
		}
		f, err := os.Open(workdir + "/audio.mp3")
		if err != nil {
			return err
		}
		defer f.Close()
		resultMessage, err = Bot.SendAudio(recepient, gotgbot.InputFileByReader(media.FileName+"_distorted.mp3", f), &gotgbot.SendAudioOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: options})
		if err != nil {
			return err
		}
		if resultMessage.Audio == nil {
			return fmt.Errorf("Telegram не вернул отправленное аудио")
		}
		DistortCache[media.FileID] = resultMessage.Audio.FileId
		if recepient == context.Message.From.Id {
			Reply("Результат отправлен в личку. Если не пришло, то нужно написать что-нибудь в личку @zavtrachat_bot.", *context)
		}
		return err
	}

	width := data.FirstVideoStream().Width
	height := data.FirstVideoStream().Height

	if width > 640 {
		height = height * 640 / width
		width = 640
	}

	if height > 480 {
		width = width * 480 / height
		height = 480
	}

	if width%2 != 0 {
		width++
	}

	if height%2 != 0 {
		height++
	}

	err = ffmpeg.Input(inputFile).Output(workdir+"/%09d.jpg", ffmpeg.KwArgs{"vf": fmt.Sprintf("scale=%d:%d", width, height)}).OverWriteOutput().ErrorToStdOut().Run()
	if err != nil {
		return err
	}

	if media.Type == "photo" || (media.Type == "sticker" && !context.Message.ReplyToMessage.Sticker.IsAnimated && !context.Message.ReplyToMessage.Sticker.IsVideo) {
		framerate = "15/1"
		src := workdir + "/000000001.jpg"
		for i := 2; i < 31; i++ {
			dst := fmt.Sprintf("%v/%09d.jpg", workdir, i)

			sourceFileStat, err := os.Stat(src)
			if err != nil {
				return err
			}

			if !sourceFileStat.Mode().IsRegular() {
				return fmt.Errorf("%s is not a regular file", src)
			}

			source, err := os.Open(src)
			if err != nil {
				return err
			}
			defer source.Close()

			destination, err := os.Create(dst)
			if err != nil {
				return err
			}
			defer destination.Close()
			_, err = io.Copy(destination, source)
			if err != nil {
				return err
			}
		}
	}

	files, err := filepath.Glob(workdir + "/*.jpg")
	if err != nil {
		return err
	}

	workCtx, cancelWork := cntx.WithDeadline(cntx.Background(), time.Unix(jobStarted+300, 0))
	defer cancelWork()
	pool := tunny.NewFunc(max(1, runtime.NumCPU()-1), func(payload interface{}) interface{} {
		args := payload.([]string)
		return exec.CommandContext(workCtx, args[0], args[1:]...).Run()
	})
	defer pool.Close()

	results := make(chan error, len(files))
	for i, file := range files {
		args := []string{"convert", file, "-liquid-rescale", fmt.Sprintf("%d%%", 90-i*65/len(files)), "-resize", fmt.Sprintf("%dx%d!", width, height), file}
		go func(args []string) {
			if result := pool.Process(args); result != nil {
				results <- result.(error)
			} else {
				results <- nil
			}
		}(args)
	}

	// QueueLength excludes work already running. Join every frame before
	// encoding or cleanup, and report worker failures without racing on err.
	for range files {
		if frameErr := <-results; frameErr != nil && err == nil {
			err = frameErr
		}
	}
	if workCtx.Err() != nil {
		return Reply("Слишком долгое выполнение операции", *context)
	}
	if err != nil {
		return err
	}

	args := []string{"-y", "-framerate", framerate, "-i", workdir + "/%09d.jpg"}
	args = append(args, additionalInputArgs...)
	args = append(args, "-c:v", "libx264", "-preset", "fast", "-crf", "26", "-pix_fmt", "yuv420p", "-movflags", "+faststart", "-hide_banner", "-loglevel", "fatal", outputFile)
	err = exec.CommandContext(workCtx, "ffmpeg", args...).Run()
	if err != nil {
		return err
	}

	f, err := os.Open(outputFile)
	if err != nil {
		return err
	}
	defer f.Close()
	resultMessage, err = Bot.SendDocument(recepient, gotgbot.InputFileByReader(media.FileName+"_distorted.mp4", f), &gotgbot.SendDocumentOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: options})
	if err != nil {
		return err
	}
	if resultMessage.Document == nil {
		return fmt.Errorf("Telegram не вернул отправленный документ")
	}
	DistortCache[media.FileID] = resultMessage.Document.FileId
	if recepient == context.Message.From.Id {
		Reply("Результат отправлен в личку. Если не пришло, то нужно написать что-нибудь в личку @zavtrachat_bot.", *context)
	}
	return err
}
