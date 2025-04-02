package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Send Get to user on /get
func GetGet(bot *gotgbot.Bot, context *ext.Context) error {
	var get Get
	if len(context.Args()) == 1 {
		return ReplyAndRemove("Пример использования: <code>/get {гет}</code>", *context)
	}
	result := DB.Where(&Get{Name: strings.ToLower(strings.Join(context.Args()[1:], " "))}).First(&get)
	if result.RowsAffected != 0 {
		options := &gotgbot.ReplyParameters{MessageId: context.Message.MessageId}
		switch {
		case get.Type == "Animation":
			_, err := bot.SendAnimation(context.Message.Chat.Id, gotgbot.InputFileByID(get.Data), &gotgbot.SendAnimationOpts{Caption: get.Caption, ReplyParameters: options})
			return err
		case get.Type == "Audio":
			_, err := bot.SendAudio(context.Message.Chat.Id, gotgbot.InputFileByID(get.Data), &gotgbot.SendAudioOpts{Caption: get.Caption, ReplyParameters: options})
			return err
		case get.Type == "Photo":
			_, err := bot.SendPhoto(context.Message.Chat.Id, gotgbot.InputFileByID(get.Data), &gotgbot.SendPhotoOpts{Caption: get.Caption, ReplyParameters: options})
			return err
		case get.Type == "Video":
			_, err := bot.SendVideo(context.Message.Chat.Id, gotgbot.InputFileByID(get.Data), &gotgbot.SendVideoOpts{Caption: get.Caption, ReplyParameters: options})
			return err
		case get.Type == "Voice":
			_, err := bot.SendVoice(context.Message.Chat.Id, gotgbot.InputFileByID(get.Data), &gotgbot.SendVoiceOpts{Caption: get.Caption, ReplyParameters: options})
			return err
		case get.Type == "Document":
			_, err := bot.SendDocument(context.Message.Chat.Id, gotgbot.InputFileByID(get.Data), &gotgbot.SendDocumentOpts{Caption: get.Caption, ReplyParameters: options})
			return err
		case get.Type == "Text":
			var entities []gotgbot.MessageEntity
			json.Unmarshal(get.Entities, &entities)
			_, err := context.Message.Reply(bot, get.Data, &gotgbot.SendMessageOpts{LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: false}, Entities: entities})
			return err
		default:
			return ReplyAndRemove(fmt.Sprintf("Ошибка при определении типа гета, я не знаю тип <code>%v</code>.", get.Type), *context)
		}
	} else {
		return ReplyAndRemove(fmt.Sprintf("Гет <code>%v</code> не найден.\nИспользуйте inline-режим бота, чтобы найти гет.", context.Message.Text), *context)
	}
}

// Answer on inline get query
func GetInline(bot *gotgbot.Bot, context *ext.Context) error {
	var count int64
	query := strings.ToLower(context.InlineQuery.Query)
	if query == "" {
		_, err := context.InlineQuery.Answer(bot, []gotgbot.InlineQueryResult{}, nil)
		return err
	}
	gets := DB.Limit(10).Model(Get{}).Where("name LIKE ?", "%"+query+"%").Count(&count)
	get_rows, err := gets.Rows()
	if err != nil {
		return err
	}
	defer get_rows.Close()
	if count > 10 {
		count = 10
	}
	results := make([]gotgbot.InlineQueryResult, count)
	var i int
	for get_rows.Next() {
		var get Get
		err := DB.ScanRows(get_rows, &get)
		if err != nil {
			return err
		}
		if get.Title == "" {
			get.Title = get.Name
		}
		switch {
		case get.Type == "Animation":
			results[i] = &gotgbot.InlineQueryResultCachedGif{
				Id:        strconv.Itoa(i),
				Title:     get.Title,
				Caption:   get.Caption,
				GifFileId: get.Data,
				ParseMode: gotgbot.ParseModeHTML,
			}
		case get.Type == "Audio":
			results[i] = &gotgbot.InlineQueryResultCachedAudio{
				Id:          strconv.Itoa(i),
				Caption:     get.Caption,
				AudioFileId: get.Data,
				ParseMode:   gotgbot.ParseModeHTML,
			}
		case get.Type == "Photo":
			results[i] = &gotgbot.InlineQueryResultCachedPhoto{
				Id:          strconv.Itoa(i),
				Title:       get.Title,
				Caption:     get.Caption,
				PhotoFileId: get.Data,
				Description: get.Caption,
				ParseMode:   gotgbot.ParseModeHTML,
			}
		case get.Type == "Video":
			results[i] = &gotgbot.InlineQueryResultCachedVideo{
				Id:          strconv.Itoa(i),
				Title:       get.Title,
				Caption:     get.Caption,
				VideoFileId: get.Data,
				Description: get.Caption,
				ParseMode:   gotgbot.ParseModeHTML,
			}
		case get.Type == "Voice":
			results[i] = &gotgbot.InlineQueryResultCachedVoice{
				Id:          strconv.Itoa(i),
				Title:       get.Title,
				Caption:     get.Caption,
				VoiceFileId: get.Data,
				ParseMode:   gotgbot.ParseModeHTML,
			}
		case get.Type == "Document":
			results[i] = &gotgbot.InlineQueryResultCachedDocument{
				Id:             strconv.Itoa(i),
				Title:          get.Title,
				Caption:        get.Caption,
				DocumentFileId: get.Data,
				Description:    get.Caption,
				ParseMode:      gotgbot.ParseModeHTML,
			}
		case get.Type == "Text":
			results[i] = &gotgbot.InlineQueryResultArticle{
				Id:          strconv.Itoa(i),
				Title:       get.Title,
				Description: get.Data,
				InputMessageContent: &gotgbot.InputTextMessageContent{
					MessageText: fmt.Sprintf("<b>%v</b>\n%v", get.Title, get.Data),
					ParseMode:   gotgbot.ParseModeHTML,
				},
			}
		default:
			log.Printf("Не удалось отправить гет %v через inline.", get.Name)
		}

		i++
		if i >= int(count) {
			continue
		}
	}

	_, err = context.InlineQuery.Answer(bot, results, nil)
	return err
}
