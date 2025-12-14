package main

import (
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Invert given file
func Loop(bot *gotgbot.Bot, context *ext.Context) error {
	if context.Message.ReplyToMessage == nil {
		return ReplyAndRemoveWithTarget("Пример использования: <code>/loop</code> в ответ на какое-либо сообщение с видео.", *context)
	}

	if !IsContainsMedia(context.Message.ReplyToMessage) {
		return ReplyAndRemoveWithTarget("Какого-либо медиа нет в указанном сообщении.", *context)
	}

	media, err := GetMedia(context.Message.ReplyToMessage)
	if err != nil {
		return err
	}

	targetArg := strings.ToLower(media.Type)
	if len(context.Args()) == 2 {
		targetArg = strings.ToLower(context.Args()[1])
	}

	switch targetArg {
	case "animation":
		targetArg = "animation"
	default:
		return ReplyAndRemoveWithTarget("Неподдерживаемая операция", *context)
	}

	targetArg = targetArg + "_loop"

	var done = make(chan bool, 1)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				context.EffectiveChat.SendAction(bot, gotgbot.ChatActionRecordVideo, nil)
			}
			time.Sleep(time.Second * 5)
		}
	}()
	defer func() {
		done <- true
	}()

	return FFmpegConvert(context, media, targetArg)
}
