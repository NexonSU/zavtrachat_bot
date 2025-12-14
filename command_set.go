package main

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm/clause"
)

// Save Get to DB on /set
func Set(bot *gotgbot.Bot, context *ext.Context) error {

	var get Get
	var inputGet string
	//args check
	if (context.Message.ReplyToMessage == nil && len(context.Args()) < 3) || (context.Message.ReplyToMessage != nil && len(context.Args()) == 1) {
		return ReplyAndRemoveWithTarget("Пример использования: <code>/set {гет} {значение}</code>\nИли отправь в ответ на какое-либо сообщение <code>/set {гет}</code>", *context)
	}
	if context.Message.ReplyToMessage == nil {
		inputGet = context.Args()[2]
	} else {
		inputGet = strings.Join(context.Args()[1:], " ")
	}
	//ownership check
	result := DB.Where(&Get{Name: strings.ToLower(inputGet)}).First(&get)
	if result.RowsAffected != 0 {
		creator, err := GetUserFromDB(fmt.Sprint(get.Creator))
		if err != nil {
			return err
		}
		if get.Creator != context.Message.From.Id && !IsAdminOrModer(context.Message.From.Id) {
			return ReplyAndRemoveWithTarget(fmt.Sprintf("Данный гет могут изменять либо администраторы, либо %v.", UserFullName(&creator)), *context)
		}
	}
	//filling Get from message
	if context.Message.ReplyToMessage == nil {
		get.Name = strings.ToLower(inputGet)
		get.Title = inputGet
		get.Type = "Text"
		_, html, _ := strings.Cut(context.Message.OriginalHTML(), " ")
		get.Data = html
	} else {
		get.Name = strings.ToLower(inputGet)
		get.Title = inputGet
		_, html, _ := strings.Cut(context.Message.ReplyToMessage.OriginalCaptionHTML(), " ")
		get.Caption = html
		switch {
		case context.Message.ReplyToMessage.Animation != nil:
			get.Type = "Animation"
			get.Data = context.Message.ReplyToMessage.Animation.FileId
		case context.Message.ReplyToMessage.Audio != nil:
			get.Type = "Audio"
			get.Data = context.Message.ReplyToMessage.Audio.FileId
		case context.Message.ReplyToMessage.Photo != nil:
			get.Type = "Photo"
			get.Data = context.Message.ReplyToMessage.Photo[0].FileId
		case context.Message.ReplyToMessage.Video != nil:
			get.Type = "Video"
			get.Data = context.Message.ReplyToMessage.Video.FileId
		case context.Message.ReplyToMessage.Voice != nil:
			get.Type = "Voice"
			get.Data = context.Message.ReplyToMessage.Voice.FileId
		case context.Message.ReplyToMessage.Document != nil:
			get.Type = "Document"
			get.Data = context.Message.ReplyToMessage.Document.FileId
		case context.Message.ReplyToMessage.Text != "":
			get.Type = "Text"
			get.Data = context.Message.ReplyToMessage.OriginalHTML()
		default:
			return ReplyAndRemoveWithTarget("Не удалось распознать файл в сообщении, возможно, он не поддерживается.", *context)
		}
	}
	get.Creator = context.Message.From.Id
	//writing get to DB
	result = DB.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&get)
	if result.Error != nil {
		return result.Error
	}
	return ReplyAndRemoveWithTarget(fmt.Sprintf("Гет <code>%v</code> сохранён как <code>%v</code>.", get.Name, get.Type), *context)
}
