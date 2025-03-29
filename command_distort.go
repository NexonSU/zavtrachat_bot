package main

import (
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

	cntx "context"

	"github.com/Jeffail/tunny"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	tele "gopkg.in/telebot.v3"
	"gopkg.in/vansante/go-ffprobe.v2"
)

var DistortBusy bool

var DistortCache map[string]string

// Distort given file
func Distort(context tele.Context) error {
	if context.Message().ReplyTo == nil {
		return ReplyAndRemove("Пример использования: <code>/distort</code> в ответ на какое-либо сообщение с видео.", context)
	}
	if context.Message().ReplyTo.Media() == nil {
		return ReplyAndRemove("Какого-либо видео нет в указанном сообщении.", context)
	}

	media := context.Message().ReplyTo.Media()
	additionalInputArgs := ""
	options := &tele.SendOptions{AllowWithoutReply: true}
	var resultMessage *tele.Message
	var err error
	var recepient tele.Recipient

	ChatMember, err := Bot.ChatMemberOf(context.Chat(), context.Sender())
	if err != nil {
		return err
	}
	if time.Now().Local().Hour() > 21 || time.Now().Local().Hour() < 7 || ChatMember.Role == "administrator" || ChatMember.Role == "creator" {
		recepient = context.Chat()
		options = &tele.SendOptions{ReplyTo: context.Message(), AllowWithoutReply: true}
	} else {
		recepient = context.Sender()
	}

	if fileId, ok := DistortCache[media.MediaFile().FileID]; ok {
		_, err = Bot.Send(recepient, &tele.Document{
			File:     tele.File{FileID: fileId},
			FileName: fileId + ".mp4",
		}, options)
		if recepient == context.Sender() {
			ReplyAndRemove("Результат отправлен в личку. Если не пришло, то нужно написать что-нибудь в личку @zavtrachat_bot.", context)
		}
		return err
	}

	switch media.MediaType() {
	case "video", "animation", "photo", "audio", "voice", "sticker":
		break
	default:
		return ReplyAndRemove("Неподдерживаемая операция", context)
	}

	if DistortBusy {
		return ReplyAndRemove("Команда занята", context)
	}

	var done = make(chan bool, 1)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				context.Notify(tele.ChatAction(tele.UploadingDocument))
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

	file, err := Bot.FileByID(media.MediaFile().FileID)
	if err != nil {
		return err
	}

	workdir := fmt.Sprintf("%v/telegram-go-chatbot-distort/%v", os.TempDir(), media.MediaFile().FileID)
	inputFile := file.FilePath
	outputFile := fmt.Sprintf("%v/output.mp4", workdir)

	ctx, cancelFn := cntx.WithTimeout(cntx.Background(), 5*time.Second)
	defer cancelFn()

	data, err := ffprobe.ProbeURL(ctx, inputFile)
	if err != nil {
		return err
	}

	framerate := "30/1"

	if media.MediaType() != "audio" && media.MediaType() != "voice" {
		frames := data.FirstVideoStream().NbFrames
		framerate = data.FirstVideoStream().AvgFrameRate

		if frames == "" {
			frames = "1"
		}

		framesInt, err := strconv.Atoi(frames)
		if err != nil {
			return err
		}

		if framesInt > 1000 && !IsAdminOrModer(context.Sender().ID) {
			return ReplyAndRemove("Видео слишком длинное. Максимум 1000 фреймов.", context)
		}
	}

	if err := os.Mkdir(workdir, os.ModePerm); err != nil {
		return ReplyAndRemove("Обработка файла уже выполняется", context)
	}
	defer func(workdir string) {
		os.RemoveAll(workdir)
	}(workdir)

	if media.MediaType() == "video" && data.FirstAudioStream() != nil {
		ffmpeg.Input(inputFile).Output(workdir + "/input_audio.mp3").OverWriteOutput().ErrorToStdOut().Run()
		ffmpeg.Input(workdir+"/input_audio.mp3").Output(workdir+"/audio.mp3", ffmpeg.KwArgs{"filter_complex": "vibrato=f=10:d=0.7"}).OverWriteOutput().ErrorToStdOut().Run()
		additionalInputArgs = "-i " + workdir + "/audio.mp3 -c:a aac"
	}

	if media.MediaType() == "audio" || media.MediaType() == "voice" {
		ffmpeg.Input(inputFile).Output(workdir + "/input_audio.mp3").OverWriteOutput().ErrorToStdOut().Run()
		err = ffmpeg.Input(workdir+"/input_audio.mp3").Output(workdir+"/audio.mp3", ffmpeg.KwArgs{"filter_complex": "vibrato=f=10:d=0.7"}).OverWriteOutput().ErrorToStdOut().Run()
		if err != nil {
			return err
		}
		resultMessage, err = Bot.Send(recepient, &tele.Audio{
			File:     tele.FromDisk(workdir + "/audio.mp3"),
			FileName: media.MediaFile().FileID + ".mp3",
			MIME:     "video/mp3",
		}, options)
		DistortCache[media.MediaFile().FileID] = resultMessage.Media().MediaFile().FileID
		if recepient == context.Sender() {
			ReplyAndRemove("Результат отправлен в личку. Если не пришло, то нужно написать что-нибудь в личку @zavtrachat_bot.", context)
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

	if media.MediaType() == "photo" || (media.MediaType() == "sticker" && !context.Message().ReplyTo.Sticker.Animated && !context.Message().ReplyTo.Sticker.Video) {
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
			return ReplyAndRemove("Слишком долгое выполнение операции", context)
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
	resultMessage, err = Bot.Send(recepient, &tele.Document{
		File:     tele.FromDisk(outputFile),
		FileName: media.MediaFile().FileID + ".mp4",
	}, options)
	DistortCache[media.MediaFile().FileID] = resultMessage.Media().MediaFile().FileID
	if recepient == context.Sender() {
		ReplyAndRemove("Результат отправлен в личку. Если не пришло, то нужно написать что-нибудь в личку @zavtrachat_bot.", context)
	}
	return err
}

func init() {
	dir := fmt.Sprintf("%v/telegram-go-chatbot-distort", os.TempDir())
	if err := os.Mkdir(dir, os.ModePerm); err != nil {
		os.RemoveAll(dir)
		os.Mkdir(dir, os.ModePerm)
	}
	DistortBusy = false
}

func init() {
	DistortCache = make(map[string]string)
}
