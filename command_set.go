package main

import (
	"encoding/json"
	"fmt"
	"strings"

	tele "gopkg.in/telebot.v3"
	"gorm.io/gorm/clause"
)

// Save Get to DB on /set
func Set(context tele.Context) error {
	var get Get
	var inputGet string
	//args check
	if (context.Message().ReplyTo == nil && len(context.Args()) < 2) || (context.Message().ReplyTo != nil && len(context.Args()) == 0) {
		return ReplyAndRemove("Пример использования: <code>/set {гет} {значение}</code>\nИли отправь в ответ на какое-либо сообщение <code>/set {гет}</code>", context)
	}
	if context.Message().ReplyTo == nil {
		inputGet = context.Args()[1]
	} else {
		inputGet = context.Data()
	}
	//ownership check
	result := DB.Where(&Get{Name: strings.ToLower(inputGet)}).First(&get)
	if result.RowsAffected != 0 {
		creator, err := GetUserFromDB(fmt.Sprint(get.Creator))
		if err != nil {
			return err
		}
		if get.Creator != context.Sender().ID && !IsAdminOrModer(context.Sender().ID) {
			return ReplyAndRemove(fmt.Sprintf("Данный гет могут изменять либо администраторы, либо %v.", UserFullName(&creator)), context)
		}
	}
	//filling Get from message
	if context.Message().ReplyTo == nil {
		get.Name = strings.ToLower(inputGet)
		get.Title = inputGet
		get.Type = "Text"
		for i := range context.Message().Entities {
			context.Message().Entities[i].Offset = context.Message().Entities[i].Offset - len(strings.Join(context.Args()[:2], " ")) - 1
		}
		get.Data = strings.Join(context.Args()[1:], " ")
		get.Entities, _ = json.Marshal(context.Message().Entities)
	} else {
		get.Name = strings.ToLower(inputGet)
		get.Title = inputGet
		get.Caption = context.Message().ReplyTo.Text
		get.Entities, _ = json.Marshal(context.Message().ReplyTo.CaptionEntities)
		switch {
		case context.Message().ReplyTo.Animation != nil:
			get.Type = "Animation"
			get.Data = context.Message().ReplyTo.Animation.FileID
		case context.Message().ReplyTo.Audio != nil:
			get.Type = "Audio"
			get.Data = context.Message().ReplyTo.Audio.FileID
		case context.Message().ReplyTo.Photo != nil:
			get.Type = "Photo"
			get.Data = context.Message().ReplyTo.Photo.FileID
		case context.Message().ReplyTo.Video != nil:
			get.Type = "Video"
			get.Data = context.Message().ReplyTo.Video.FileID
		case context.Message().ReplyTo.Voice != nil:
			get.Type = "Voice"
			get.Data = context.Message().ReplyTo.Voice.FileID
		case context.Message().ReplyTo.Document != nil:
			get.Type = "Document"
			get.Data = context.Message().ReplyTo.Document.FileID
		case context.Message().ReplyTo.Text != "":
			get.Type = "Text"
			get.Data = context.Message().ReplyTo.Text
			get.Entities, _ = json.Marshal(context.Message().ReplyTo.Entities)
		default:
			return ReplyAndRemove("Не удалось распознать файл в сообщении, возможно, он не поддерживается.", context)
		}
	}
	get.Creator = context.Sender().ID
	//writing get to DB
	result = DB.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&get)
	if result.Error != nil {
		return result.Error
	}
	return ReplyAndRemove(fmt.Sprintf("Гет <code>%v</code> сохранён как <code>%v</code>.", get.Name, get.Type), context)
}
