package main

import (
	cntx "context"
	"fmt"
	"os"
	"time"

	"github.com/lrstanley/go-ytdlp"
	tele "gopkg.in/telebot.v3"
)

// Convert given  file
func Download(context tele.Context) error {
	filePath := fmt.Sprintf("%v/%v.mp4", os.TempDir(), context.Message().ID)

	context.Delete()

	if context.Message().ReplyTo == nil && len(context.Args()) < 1 {
		return ReplyAndRemove("Пример использования: <code>/download {ссылка на ютуб/твиттер}</code>\nИли отправь в ответ на какое-либо сообщение с ссылкой <code>/download</code>", context)
	}

	link := ""
	message := &tele.Message{}

	if context.Message().ReplyTo == nil {
		message = context.Message()
	} else {
		message = context.Message().ReplyTo
	}

	var downloadNotify = make(chan bool, 1)
	go func() {
		for {
			select {
			case <-downloadNotify:
				return
			default:
				context.Notify(tele.RecordingVideo)
			}
			time.Sleep(time.Second * 5)
		}
	}()
	defer func() {
		downloadNotify <- true
	}()

	for _, entity := range message.Entities {
		if entity.Type == tele.EntityURL || entity.Type == tele.EntityTextLink {
			link = entity.URL
			if link == "" {
				link = message.EntityText(entity)
			}
		}
	}

	ytdlp.MustInstall(cntx.TODO(), nil)

	ytdlpDownload := ytdlp.New().Downloader("aria2c").Downloader("dash,m3u8:native").Impersonate("Chrome-124").Format("bestvideo[height<=?720]+bestaudio/best").RecodeVideo("mp4").Output(filePath).MaxFileSize("512M").PrintJSON().EmbedThumbnail().EmbedMetadata()

	ytdlpResult, err := ytdlpDownload.Run(cntx.TODO(), link)
	if err != nil {
		return err
	}

	ytdlpInfo, err := ytdlpResult.GetExtractedInfo()
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
				context.Notify(tele.UploadingVideo)
			}
			time.Sleep(time.Second * 5)
		}
	}()
	defer func() {
		uploadNotify <- true
		os.Remove(filePath)
	}()

	return context.Send(&tele.Video{
		FileName:  *ytdlpInfo[0].Title + ".mp4",
		Streaming: true,
		File: tele.File{
			FileLocal: filePath,
		},
	})
}
