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
	"strings"
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

// Distort given file
func Distort(bot *gotgbot.Bot, context *ext.Context) error {
	dir := fmt.Sprintf("%v/telegram-go-chatbot-distort", os.TempDir())
	if err := os.Mkdir(dir, os.ModePerm); err != nil {
		os.RemoveAll(dir)
		os.Mkdir(dir, os.ModePerm)
	}
	if DistortCache == nil {
		DistortCache = make(map[string]string)
	}

	if context.Message.ReplyToMessage == nil {
		return ReplyAndRemove("Пример использования: <code>/distort</code> в ответ на какое-либо сообщение с видео.", *context)
	}
	if !IsContainsMedia(context.Message.ReplyToMessage) {
		return ReplyAndRemove("Какого-либо видео нет в указанном сообщении.", *context)
	}

	media, err := GetMedia(context.Message.ReplyToMessage)
	if err != nil {
		return err
	}
	additionalInputArgs := ""
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
		_, err = Bot.SendDocument(recepient, gotgbot.InputFileByID(fileId), &gotgbot.SendDocumentOpts{ReplyParameters: &gotgbot.ReplyParameters{}})
		if recepient == context.Message.From.Id {
			ReplyAndRemove("Результат отправлен в личку. Если не пришло, то нужно написать что-нибудь в личку @zavtrachat_bot.", *context)
		}
		return err
	}

	switch media.Type {
	case "video", "animation", "photo", "audio", "voice", "sticker":
		break
	default:
		return ReplyAndRemove("Неподдерживаемая операция", *context)
	}

	if DistortBusy {
		return ReplyAndRemove("Команда занята", *context)
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

	workdir := fmt.Sprintf("%v/telegram-go-chatbot-distort/%v", os.TempDir(), media.FileID)
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
			return ReplyAndRemove("Видео слишком длинное. Максимум 1000 фреймов.", *context)
		}
	}

	if err := os.Mkdir(workdir, os.ModePerm); err != nil {
		return ReplyAndRemove("Обработка файла уже выполняется", *context)
	}
	defer func(workdir string) {
		os.RemoveAll(workdir)
	}(workdir)

	if media.Type == "video" && data.FirstAudioStream() != nil {
		ffmpeg.Input(inputFile).Output(workdir + "/input_audio.mp3").OverWriteOutput().ErrorToStdOut().Run()
		ffmpeg.Input(workdir+"/input_audio.mp3").Output(workdir+"/audio.mp3", ffmpeg.KwArgs{"filter_complex": "vibrato=f=10:d=0.7"}).OverWriteOutput().ErrorToStdOut().Run()
		additionalInputArgs = "-i " + workdir + "/audio.mp3 -c:a aac"
	}

	if media.Type == "audio" || media.Type == "voice" {
		ffmpeg.Input(inputFile).Output(workdir + "/input_audio.mp3").OverWriteOutput().ErrorToStdOut().Run()
		err = ffmpeg.Input(workdir+"/input_audio.mp3").Output(workdir+"/audio.mp3", ffmpeg.KwArgs{"filter_complex": "vibrato=f=10:d=0.7"}).OverWriteOutput().ErrorToStdOut().Run()
		if err != nil {
			return err
		}
		resultMessage, err = Bot.SendAudio(recepient, gotgbot.InputFileByURL(fmt.Sprintf("file://%v", workdir+"/audio.mp3")), &gotgbot.SendAudioOpts{ReplyParameters: options})
		DistortCache[media.FileID] = resultMessage.Audio.FileId
		if recepient == context.Message.From.Id {
			ReplyAndRemove("Результат отправлен в личку. Если не пришло, то нужно написать что-нибудь в личку @zavtrachat_bot.", *context)
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

	err = ffmpeg.Input(inputFile).Output(workdir+"/%09d.png", ffmpeg.KwArgs{"vf": fmt.Sprintf("scale=%d:%d", width, height)}).OverWriteOutput().ErrorToStdOut().Run()
	if err != nil {
		return err
	}

	if media.Type == "photo" || (media.Type == "sticker" && !context.Message.ReplyToMessage.Sticker.IsAnimated && !context.Message.ReplyToMessage.Sticker.IsVideo) {
		framerate = "15/1"
		src := workdir + "/000000001.png"
		for i := 2; i < 31; i++ {
			dst := fmt.Sprintf("%v/%09d.png", workdir, i)

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

	files, err := filepath.Glob(workdir + "/*.png")
	if err != nil {
		return err
	}

	pool := tunny.NewFunc(runtime.NumCPU()-1, func(payload interface{}) interface{} {
		payloadCommand := strings.Fields(payload.(string))
		return exec.Command(payloadCommand[0], payloadCommand[1:]...).Run()
	})
	defer pool.Close()

	for i, file := range files {
		command := fmt.Sprintf("convert %v -liquid-rescale %v%% -resize %vx%v! %v", file, 90-(i*65/len(files)), width, height, file)
		go func(command string) {
			if pool.Process(command) != nil {
				err = pool.Process(command).(error)
			}
		}(command)
	}

	for {
		time.Sleep(1 * time.Second)
		if time.Now().Unix()-jobStarted > 300 {
			return ReplyAndRemove("Слишком долгое выполнение операции", *context)
		}
		if pool.QueueLength() == 0 {
			break
		}
	}
	if err != nil {
		return err
	}

	ffmpegCommand := fmt.Sprintf("ffmpeg -y -framerate %v -i %v/%%09d.png %v -c:v: libx264 -preset fast -crf 26 -pix_fmt yuv420p -movflags +faststart -hide_banner -loglevel fatal %v", framerate, workdir, additionalInputArgs, outputFile)
	ffmpegCommandExec := strings.Fields(ffmpegCommand)
	err = exec.Command(ffmpegCommandExec[0], ffmpegCommandExec[1:]...).Run()
	if err != nil {
		return err
	}

	DistortBusy = false
	resultMessage, err = Bot.SendDocument(recepient, gotgbot.InputFileByURL(fmt.Sprintf("file://%v", outputFile)), &gotgbot.SendDocumentOpts{ReplyParameters: options})
	DistortCache[media.FileID] = resultMessage.Document.FileId
	if recepient == context.Message.From.Id {
		ReplyAndRemove("Результат отправлен в личку. Если не пришло, то нужно написать что-нибудь в личку @zavtrachat_bot.", *context)
	}
	return err
}
