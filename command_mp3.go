package main

import (
	cntx "context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/NexonSU/go-ytdlp"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Convert given  file
func Mp3(bot *gotgbot.Bot, context *ext.Context) error {
	filePath := fmt.Sprintf("%v/%v.mp3", os.TempDir(), context.Message.MessageId)

	if context.Message.ReplyToMessage == nil && len(context.Args()) < 2 {
		return ReplyAndRemoveWithTarget("Пример использования: <code>/mp3 {ссылка на ютуб/ресурс}</code>\nИли отправь в ответ на какое-либо сообщение с ссылкой <code>/mp3</code>", *context)
	}

	if strings.Contains(context.EffectiveMessage.Text, " remove") {
		context.EffectiveMessage.Delete(Bot, nil)
	} else {
		context.EffectiveMessage.SetReaction(Bot, &gotgbot.SetMessageReactionOpts{
			Reaction: []gotgbot.ReactionType{gotgbot.ReactionTypeEmoji{Emoji: "👀"}},
		})
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

	if link == "" {
		return fmt.Errorf("каких-либо ссылок не найдено")
	}

	ytdlpDownload := ytdlp.
		New().
		PrintJSON().
		NoProgress().
		NoPlaylist().
		NoOverwrites().
		Format("bestaudio[ext=m4a]").
		EmbedMetadata().
		NoEmbedChapters().
		ExtractAudio().
		AudioFormat("mp3").
		Output(filePath).
		MaxFileSize("64M")

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
				context.EffectiveChat.SendAction(bot, gotgbot.ChatActionUploadVoice, nil)
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
	soundOpts := &gotgbot.SendAudioOpts{
		ReplyParameters: &gotgbot.ReplyParameters{
			MessageId:                context.EffectiveMessage.MessageId,
			AllowSendingWithoutReply: true,
		}}
	if extInfo.Duration != nil {
		soundOpts.Duration = int64(*extInfo.Duration)
	}
	title := ""
	if extInfo.Track == nil && extInfo.Title != nil {
		soundOpts.Title = *extInfo.Title
		title = *extInfo.Title
	}
	if title == "" {
		title = f.Name()
	}
	_, err = bot.SendAudio(context.Message.Chat.Id, gotgbot.InputFileByReader(title+".mp3", f), soundOpts)
	f.Close()
	os.Remove(filePath)
	return err
}
