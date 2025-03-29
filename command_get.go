package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v3"
)

// Send Get to user on /get
func GetGet(context tele.Context) error {
	var get Get
	if len(context.Args()) == 0 {
		return ReplyAndRemove("Пример использования: <code>/get {гет}</code>", context)
	}
	result := DB.Where(&Get{Name: strings.ToLower(context.Data())}).First(&get)
	if result.RowsAffected != 0 {
		switch {
		case get.Type == "Animation":
			return context.Reply(&tele.Animation{
				File:    tele.File{FileID: get.Data},
				Caption: get.Caption,
			})
		case get.Type == "Audio":
			return context.Reply(&tele.Audio{
				File:    tele.File{FileID: get.Data},
				Caption: get.Caption,
			})
		case get.Type == "Photo":
			return context.Reply(&tele.Photo{
				File:    tele.File{FileID: get.Data},
				Caption: get.Caption,
			})
		case get.Type == "Video":
			return context.Reply(&tele.Video{
				File:    tele.File{FileID: get.Data},
				Caption: get.Caption,
			})
		case get.Type == "Voice":
			return context.Reply(&tele.Voice{
				File:    tele.File{FileID: get.Data},
				Caption: get.Caption,
			})
		case get.Type == "Document":
			return context.Reply(&tele.Document{
				File:    tele.File{FileID: get.Data},
				Caption: get.Caption,
			})
		case get.Type == "Text":
			var entities tele.Entities
			json.Unmarshal(get.Entities, &entities)
			return context.Reply(get.Data, &tele.SendOptions{DisableWebPagePreview: true, Entities: entities})
		default:
			return ReplyAndRemove(fmt.Sprintf("Ошибка при определении типа гета, я не знаю тип <code>%v</code>.", get.Type), context)
		}
	} else {
		return ReplyAndRemove(fmt.Sprintf("Гет <code>%v</code> не найден.\nИспользуйте inline-режим бота, чтобы найти гет.", context.Data()), context)
	}
}

// Answer on inline get query
func GetInline(context tele.Context) error {
	var count int64
	query := strings.ToLower(context.Query().Text)
	if query == "" {
		return context.Answer(&tele.QueryResponse{})
	}
	gets := DB.Limit(10).Model(Get{}).Where("name LIKE ?", "%"+query+"%").Count(&count)
	get_rows, err := gets.Rows()
	if err != nil {
		return err
	}
	if count > 10 {
		count = 10
	}
	results := make(tele.Results, count)
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
			results[i] = &tele.GifResult{
				Title:   get.Title,
				Caption: get.Caption,
				Cache:   get.Data,
			}
		case get.Type == "Audio":
			results[i] = &tele.DocumentResult{
				Title:       get.Title,
				Caption:     get.Caption,
				Cache:       get.Data,
				Description: get.Caption,
			}
		case get.Type == "Photo":
			results[i] = &tele.PhotoResult{
				Title:       get.Title,
				Caption:     get.Caption,
				Cache:       get.Data,
				Description: get.Caption,
			}
		case get.Type == "Video":
			results[i] = &tele.VideoResult{
				Title:       get.Title,
				Caption:     get.Caption,
				Cache:       get.Data,
				Description: get.Caption,
			}
		case get.Type == "Voice":
			results[i] = &tele.VoiceResult{
				Title:   get.Title,
				Caption: get.Caption,
				Cache:   get.Data,
			}
		case get.Type == "Document":
			results[i] = &tele.DocumentResult{
				Title:       get.Title,
				Caption:     get.Caption,
				Cache:       get.Data,
				Description: get.Caption,
			}
		case get.Type == "Text":
			results[i] = &tele.ArticleResult{
				Title:       get.Title,
				Description: get.Data,
			}
			results[i].SetContent(tele.InputMessageContent(&tele.InputTextMessageContent{
				Text:      fmt.Sprintf("<b>%v</b>\n%v", get.Title, get.Data),
				ParseMode: tele.ModeHTML,
			}))
		default:
			log.Printf("Не удалось отправить гет %v через inline.", get.Name)
		}

		results[i].SetResultID(strconv.Itoa(i))

		i++
		if i >= int(count) {
			continue
		}
	}

	return context.Answer(&tele.QueryResponse{
		Results: results,
	})
}
