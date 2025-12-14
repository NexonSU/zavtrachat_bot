package main

import (
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Convert given file
func Convert(bot *gotgbot.Bot, context *ext.Context) error {
	if context.Message.ReplyToMessage == nil {
		for _, entity := range context.Message.Entities {
			if entity.Type == "url" {
				return Download(bot, context)
			}
		}
		return ReplyAndRemoveWithTarget("Пример использования: <code>/convert</code> в ответ на какое-либо сообщение с медиа-файлом.\nДопольнительные параметры: gif,mp3,ogg,jpg.", *context)
	}
	if !IsContainsMedia(context.Message.ReplyToMessage) {
		for _, entity := range context.Message.ReplyToMessage.Entities {
			if entity.Type == "url" {
				return Download(bot, context)
			}
		}
		return ReplyAndRemoveWithTarget("Какого-либо медиа файла нет в указанном сообщении.", *context)
	}

	media, err := GetMedia(context.Message.ReplyToMessage)
	if err != nil {
		return err
	}

	var targetArg string

	targetArg = media.Type

	if targetArg == "sticker" {
		if context.Message.ReplyToMessage.Sticker.IsVideo {
			targetArg = "gif"
		}
	}

	if len(context.Args()) == 2 {
		targetArg = strings.ToLower(context.Args()[1])
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
