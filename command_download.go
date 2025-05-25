package main

import (
	cntx "context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/lrstanley/go-ytdlp"
)

func init() {
	_, err := ytdlp.Install(cntx.TODO(), &ytdlp.InstallOptions{AllowVersionMismatch: true})
	if err != nil {
		log.Println(err.Error())
	}
}

// Convert given  file
func Download(bot *gotgbot.Bot, context *ext.Context) error {
	if context.Message.ReplyToMessage == nil && len(context.Args()) < 2 {
		return ReplyAndRemove("Пример использования: <code>/download {ссылка на ютуб/твиттер}</code>\nИли отправь в ответ на какое-либо сообщение с ссылкой <code>/download</code>", *context)
	}

	context.EffectiveMessage.SetReaction(Bot, &gotgbot.SetMessageReactionOpts{
		Reaction: []gotgbot.ReactionType{gotgbot.ReactionTypeEmoji{Emoji: "👀"}},
	})

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

	ytdlpDownload := ytdlp.
		New().
		PrintJSON().
		NoProgress().
		NoPlaylist().
		NoOverwrites().
		Format("best[height<=720][ext=mp4]/bestvideo[height<=720][ext=mp4]+bestaudio[ext=m4a]/best[height<=720]").
		RecodeVideo("mp4").
		EmbedMetadata().
		Output(os.TempDir() + "/%(extractor)s - %(title)s.%(ext)s").
		MaxFileSize("512M")

	result, err := ytdlpDownload.Run(cntx.TODO(), link)
	if err != nil {
		return err
	}

	extInfos, err := result.GetExtractedInfo()
	if err != nil {
		return err
	}

	extInfo := extInfos[0]

	filePath := *extInfo.Filename

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
	}()
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	_, err = bot.SendVideo(context.Message.Chat.Id, gotgbot.InputFileByReader(filepath.Base(filePath), f), &gotgbot.SendVideoOpts{SupportsStreaming: true, ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.EffectiveMessage.MessageId, AllowSendingWithoutReply: true}})
	f.Close()
	os.Remove(filePath)
	return err
}
