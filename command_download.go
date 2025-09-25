package main

import (
	cntx "context"
	"fmt"
	"html"
	"log"
	"os"
	"strings"
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
	filePath := fmt.Sprintf("%v/%v.mp4", os.TempDir(), context.Message.MessageId)

	if context.Message.ReplyToMessage == nil && len(context.Args()) < 2 {
		return ReplyAndRemove("Пример использования: <code>/download {ссылка на ютуб/твиттер}</code>\nИли отправь в ответ на какое-либо сообщение с ссылкой <code>/download</code>", *context)
	}

	if strings.Contains(context.EffectiveMessage.Text, " remove") {
		context.EffectiveMessage.Delete(Bot, nil)
	} else {
		context.EffectiveMessage.SetReaction(Bot, &gotgbot.SetMessageReactionOpts{
			Reaction: []gotgbot.ReactionType{gotgbot.ReactionTypeEmoji{Emoji: "👀"}},
		})
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

	if link == "" {
		return fmt.Errorf("каких-либо ссылок не найдено")
	}

	ytdlpDownload := ytdlp.
		New().
		PrintJSON().
		NoProgress().
		NoPlaylist().
		NoOverwrites().
		Format("bestvideo[height<=?720]+bestaudio/best").
		RecodeVideo("mp4").
		EmbedMetadata().
		NoEmbedChapters().
		Output(filePath).
		MaxFileSize("512M")

	if Config.Proxy != "" {
		ytdlpDownload.Proxy(Config.Proxy)
	}

	result, err := ytdlpDownload.Run(cntx.TODO(), link)
	if err != nil {
		return err
	}

	extInfos, err := result.GetExtractedInfo()
	if err != nil {
		return err
	}

	if len(extInfos) == 0 {
		return fmt.Errorf("невозможно извлечь информацию из yt-dlp")
	}

	extInfo := extInfos[0]

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
	videoOpts := &gotgbot.SendVideoOpts{
		SupportsStreaming: true,
		ReplyParameters: &gotgbot.ReplyParameters{
			MessageId:                context.EffectiveMessage.MessageId,
			AllowSendingWithoutReply: true,
		}}
	if extInfo.Duration != nil {
		videoOpts.Duration = int64(*extInfo.Duration)
	}
	if extInfo.Width != nil {
		videoOpts.Width = int64(*extInfo.Width)
	}
	if extInfo.Height != nil {
		videoOpts.Height = int64(*extInfo.Height)
	}
	if extInfo.Thumbnail != nil {
		videoOpts.Cover = gotgbot.InputFileByURL(*extInfo.Thumbnail)
	}
	caption := ""
	title := ""
	if extInfo.Title != nil {
		title = html.EscapeString(*extInfo.Title)
		caption += html.EscapeString(*extInfo.Title)
		if extInfo.Description != nil {
			caption += "\n<blockquote expandable>" + html.EscapeString(*extInfo.Description)
			if len([]rune(caption)) > 1000 {
				caption = string([]rune(caption)[:1000]) + "..."
			}
			caption += "</blockquote>"
		}
	}
	if title == "" {
		title = f.Name()
	}
	if strings.Contains(context.EffectiveMessage.Text, " caption") && caption != "" {
		videoOpts.Caption = caption
	}
	_, err = bot.SendVideo(context.Message.Chat.Id, gotgbot.InputFileByReader(title+".mp4", f), videoOpts)
	f.Close()
	os.Remove(filePath)
	return err
}
