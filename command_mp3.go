package main

import (
	cntx "context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/lrstanley/go-ytdlp"
)

// Convert given  file
func Mp3(bot *gotgbot.Bot, context *ext.Context) error {
	filePath := fmt.Sprintf("%v/%v.mp3", os.TempDir(), context.Message.MessageId)

	context.Message.Delete(bot, nil)

	if context.Message.ReplyToMessage == nil && len(context.Args()) < 2 {
		return ReplyAndRemove("Пример использования: <code>/mp3 {ссылка на ютуб/ресурс}</code>\nИли отправь в ответ на какое-либо сообщение с ссылкой <code>/mp3</code>", *context)
	}

	link := ""
	message := &gotgbot.Message{}

	if context.Message.ReplyToMessage == nil {
		message = context.Message
	} else {
		message = context.Message.ReplyToMessage
	}

	var downloadNotify = make(chan bool, 1)
	go func() {
		for {
			select {
			case <-downloadNotify:
				return
			default:
				context.EffectiveChat.SendAction(bot, gotgbot.ChatActionRecordVoice, nil)
			}
			time.Sleep(time.Second * 5)
		}
	}()
	defer func() {
		downloadNotify <- true
	}()

	for _, entity := range message.Entities {
		if entity.Type == "url" || entity.Type == "text_link" {
			link = entity.Url
			if link == "" {
				link = gotgbot.ParseEntity(message.Text, entity).Text
			}
		}
	}

	ytdlp.MustInstall(cntx.TODO(), nil)

	ytdlpDownload := ytdlp.New().Impersonate("Chrome-124").ExtractAudio().AudioFormat("mp3").EmbedMetadata().Output(filePath).MaxFileSize("512M")

	_, err := ytdlpDownload.Run(cntx.TODO(), link)
	if err != nil {
		return err
	}

	downloadNotify <- true

	var uploadNotify = make(chan bool, 1)
	go func() {
		for {
			select {
			case <-uploadNotify:
				return
			default:
				context.EffectiveChat.SendAction(bot, gotgbot.ChatActionUploadVoice, nil)
			}
			time.Sleep(time.Second * 5)
		}
	}()
	defer func() {
		uploadNotify <- true
		os.Remove(filePath)
	}()
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	_, err = bot.SendAudio(context.Message.Chat.Id, gotgbot.InputFileByReader(filepath.Base(filePath), f), &gotgbot.SendAudioOpts{})
	f.Close()
	return err
}
