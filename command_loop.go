package main

import (
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"
)

// Invert given file
func Loop(context tele.Context) error {
	if context.Message().ReplyTo == nil {
		return ReplyAndRemove("Пример использования: <code>/loop</code> в ответ на какое-либо сообщение с видео.", context)
	}
	if context.Message().ReplyTo.Media() == nil {
		return ReplyAndRemove("Какого-либо видео нет в указанном сообщении.", context)
	}

	media := context.Message().ReplyTo.Media()

	targetArg := media.MediaType()
	if len(context.Args()) == 1 {
		targetArg = strings.ToLower(context.Args()[0])
	}

	switch targetArg {
	case "animation":
		targetArg = "animation"
	default:
		return ReplyAndRemove("Неподдерживаемая операция", context)
	}

	targetArg = targetArg + "_loop"

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
		done <- true
	}()

	file, err := Bot.FileByID(media.MediaFile().FileID)
	if err != nil {
		return err
	}

	return FFmpegConvert(context, file.FilePath, targetArg)
}
