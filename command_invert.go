package main

import (
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Invert given file
func Invert(bot *gotgbot.Bot, context *ext.Context) error {
	if context.Message.ReplyToMessage == nil {
		return ReplyAndRemove("Пример использования: <code>/invert</code> в ответ на какое-либо сообщение с видео.", *context)
	}
	if !IsContainsMedia(context.Message.ReplyToMessage) {
		return ReplyAndRemove("Какого-либо видео нет в указанном сообщении.", *context)
	}

	media, err := GetMedia(context.Message.ReplyToMessage)
	if err != nil {
		return err
	}

	targetArg := media.Type
	if len(context.Args()) == 2 {
		targetArg = strings.ToLower(context.Args()[1])
	}

	switch targetArg {
	case "video", "mp4":
		targetArg = "video"
	case "animation", "gif":
		targetArg = "animation"
	case "sticker", "webm":
		targetArg = "sticker"
	case "voice", "ogg":
		targetArg = "voice"
	case "audio", "mp3":
		targetArg = "audio"
	default:
		return ReplyAndRemove("Неподдерживаемая операция", *context)
	}

	targetArg = targetArg + "_reverse"

	if targetArg == "sticker_reverse" {
		if !context.Message.ReplyToMessage.Sticker.IsVideo {
			return ReplyAndRemove("Неподдерживаемая операция", *context)
		}
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
		done <- true
	}()

	return FFmpegConvert(context, media, targetArg)
}
