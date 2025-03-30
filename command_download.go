package main

import (
	cntx "context"
	"fmt"
	"os"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/lrstanley/go-ytdlp"
)

// Convert given  file
func Download(bot *gotgbot.Bot, context *ext.Context) error {
	filePath := fmt.Sprintf("%v/%v.mp4", os.TempDir(), context.Message.MessageId)

	context.Message.Delete(bot, nil)

	if context.Message.ReplyToMessage == nil && len(context.Args()) < 2 {
		return ReplyAndRemove("Пример использования: <code>/download {ссылка на ютуб/твиттер}</code>\nИли отправь в ответ на какое-либо сообщение с ссылкой <code>/download</code>", *context)
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
				context.EffectiveChat.SendAction(bot, gotgbot.ChatActionRecordVideo, nil)
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

	ytdlpDownload := ytdlp.New().Downloader("aria2c").Downloader("dash,m3u8:native").Impersonate("Chrome-124").Format("bestvideo[height<=?720]+bestaudio/best").RecodeVideo("mp4").Output(filePath).MaxFileSize("512M").PrintJSON().EmbedThumbnail().EmbedMetadata()

	ytdlpResult, err := ytdlpDownload.Run(cntx.TODO(), link)
	if err != nil {
		return err
	}

	_, err = ytdlpResult.GetExtractedInfo()
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
				context.EffectiveChat.SendAction(bot, gotgbot.ChatActionUploadVideo, nil)
			}
			time.Sleep(time.Second * 5)
		}
	}()
	defer func() {
		uploadNotify <- true
		os.Remove(filePath)
	}()
	_, err = bot.SendVideo(context.Message.Chat.Id, gotgbot.InputFileByURL(fmt.Sprintf("file://%v", filePath)), &gotgbot.SendVideoOpts{SupportsStreaming: true})
	return err
}
