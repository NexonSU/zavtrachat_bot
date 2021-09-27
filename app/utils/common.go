package utils

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"math/big"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/NexonSU/telebot"
	"gorm.io/gorm/clause"
)

func RandInt(min int, max int) int {
	b, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0
	}
	return min + int(b.Int64())
}

func IsAdmin(username string) bool {
	for _, b := range Config.Admins {
		if b == username {
			return true
		}
	}
	return false
}

func IsAdminOrModer(username string) bool {
	for _, b := range Config.Admins {
		if b == username {
			return true
		}
	}
	for _, b := range Config.Moders {
		if b == username {
			return true
		}
	}
	return false
}

func RestrictionTimeMessage(seconds int64) string {
	var message = ""
	if seconds-30 > time.Now().Unix() {
		message = fmt.Sprintf(" до %v", time.Unix(seconds, 0).Format("02.01.2006 15:04:05"))
	}
	return message
}

func ErrorReporting(err error, context telebot.Context) {
	_, fn, line, _ := runtime.Caller(1)
	log.Printf("[%s:%d] %v", fn, line, err)
	text := fmt.Sprintf("<pre>[%s:%d]\n%v</pre>", fn, line, err)
	if context.Message() != nil {
		MarshalledMessage, _ := json.MarshalIndent(context.Message(), "", "    ")
		JsonMessage := html.EscapeString(string(MarshalledMessage))
		text += fmt.Sprintf("\n\nMessage:\n<pre>%v</pre>", JsonMessage)
	}
	Bot.Send(telebot.ChatID(Config.SysAdmin), text)
}

func FindUserInMessage(context telebot.Context) (telebot.User, int64, error) {
	var user telebot.User
	var err error = nil
	var untildate = time.Now().Unix() + 86400
	for _, entity := range context.Message().Entities {
		if entity.Type == telebot.EntityTMention {
			user = *entity.User
			if len(context.Args()) == 2 {
				addtime, err := strconv.ParseInt(context.Args()[1], 10, 64)
				if err != nil {
					return user, untildate, err
				}
				untildate += addtime - 86400
			}
			return user, untildate, err
		}
	}
	if context.Message().ReplyTo != nil {
		user = *context.Message().ReplyTo.Sender
		if len(context.Args()) == 1 {
			addtime, err := strconv.ParseInt(context.Args()[0], 10, 64)
			if err != nil {
				return user, untildate, errors.New("время указано неверно")
			}
			untildate += addtime - 86400
		}
	} else {
		if len(context.Args()) == 0 {
			err = errors.New("пользователь не найден")
			return user, untildate, err
		}
		if user.ID == 0 {
			user, err = GetUserFromDB(context.Args()[0])
			if err != nil {
				return user, untildate, err
			}
		}
		if len(context.Args()) == 2 {
			addtime, err := strconv.ParseInt(context.Args()[1], 10, 64)
			if err != nil {
				return user, untildate, errors.New("время указано неверно")
			}
			untildate += addtime - 86400
		}
	}
	return user, untildate, err
}

func GatherData(update *telebot.Update) error {
	if update.Message == nil || update.Message.Sender == nil {
		return nil
	}
	//User update
	UserResult := DB.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(update.Message.Sender)
	if UserResult.Error != nil {
		return UserResult.Error
	}
	if update.Message.Sender.IsBot {
		return nil
	}
	//Message insert
	var Message Message
	Message.ID = update.Message.ID
	Message.UserID = update.Message.Sender.ID
	Message.Date = time.Unix(update.Message.Unixtime, 0)
	Message.ChatID = update.Message.Chat.ID
	if update.Message.ReplyTo != nil {
		Message.ReplyTo = update.Message.ReplyTo.ID
	}
	Message.Text = update.Message.Text
	switch {
	case update.Message.Animation != nil:
		Message.FileType = "Animation"
		Message.FileID = update.Message.Animation.FileID
		Message.Text = update.Message.Caption
	case update.Message.Audio != nil:
		Message.FileType = "Audio"
		Message.FileID = update.Message.Audio.FileID
		Message.Text = update.Message.Caption
	case update.Message.Photo != nil:
		Message.FileType = "Photo"
		Message.FileID = update.Message.Photo.FileID
		Message.Text = update.Message.Caption
	case update.Message.Video != nil:
		Message.FileType = "Video"
		Message.FileID = update.Message.Video.FileID
		Message.Text = update.Message.Caption
	case update.Message.Voice != nil:
		Message.FileType = "Voice"
		Message.FileID = update.Message.Voice.FileID
		Message.Text = update.Message.Caption
	case update.Message.Document != nil:
		Message.FileType = "Document"
		Message.FileID = update.Message.Document.FileID
		Message.Text = update.Message.Caption
	}
	MessageResult := DB.Create(&Message)
	if MessageResult.Error != nil {
		return MessageResult.Error
	}
	//Words insert
	var Word Word
	Word.ChatID = Message.ChatID
	Word.UserID = Message.UserID
	Word.Date = Message.Date
	for _, Word.Text = range strings.Fields(Message.Text) {
		WordResult := DB.Create(&Word)
		if WordResult.Error != nil {
			return WordResult.Error
		}
	}
	return nil
}

func GetUserFromDB(findstring string) (telebot.User, error) {
	var user telebot.User
	var err error = nil
	if string(findstring[0]) == "@" {
		user.Username = findstring[1:]
	} else {
		user.ID, err = strconv.ParseInt(findstring, 10, 64)
	}
	result := DB.Where("lower(username) = ? OR id = ?", strings.ToLower(user.Username), user.ID).First(&user)
	if result.Error != nil {
		err = result.Error
	}
	return user, err
}

type ForwardedMesssage struct {
	ChannelMessage *telebot.Message
	ChatMessage    telebot.Message
}
type ForwardMesssage struct {
	AlbumID            string
	Messages           []*telebot.Message
	Caption            string
	ForwardedMesssages []ForwardedMesssage
}

var Forward ForwardMesssage

//Repost channel post to chat
func Repost(context telebot.Context) error {
	chat, err := Bot.ChatByID("@" + Config.Chat)
	if err != nil {
		return err
	}
	if context.Message().AlbumID != "" {
		Forward.Messages = append(Forward.Messages, context.Message())
		if context.Message().Caption != "" {
			Forward.Caption = context.Message().Caption
		}
		if context.Message().AlbumID != Forward.AlbumID {
			Forward.AlbumID = context.Message().AlbumID
			time.Sleep(5 * time.Second)
			sort.SliceStable(Forward.Messages, func(i, j int) bool {
				return Forward.Messages[i].ID < Forward.Messages[j].ID
			})
			var Album []telebot.InputMedia
			for i, message := range Forward.Messages {
				switch {
				case context.Message().Audio != nil:
					message.Audio.Caption = ""
					if i == 0 {
						message.Audio.Caption = Forward.Caption
					}
					Album = append(Album, message.Audio)
				case context.Message().Document != nil:
					message.Document.Caption = ""
					if i == 0 {
						message.Document.Caption = Forward.Caption
					}
					Album = append(Album, message.Document)
				case context.Message().Photo != nil:
					message.Photo.Caption = ""
					if i == 0 {
						message.Photo.Caption = Forward.Caption
						message.Photo.ParseMode = telebot.ModeHTML
					}
					Album = append(Album, message.Photo)
				case context.Message().Video != nil:
					message.Video.Caption = ""
					if i == 0 {
						message.Video.Caption = Forward.Caption
					}
					Album = append(Album, message.Video)
				}
			}
			ChatMessage, err := Bot.SendAlbum(chat, Album)
			for i, message := range Forward.Messages {
				Forward.ForwardedMesssages = append(Forward.ForwardedMesssages, ForwardedMesssage{message, ChatMessage[i]})
			}
			Forward.AlbumID = ""
			Forward.Messages = []*telebot.Message{}
			Forward.Caption = ""
			return err
		}
		return nil
	}

	var ChatMessage *telebot.Message
	switch {
	case context.Message().Animation != nil:
		ChatMessage, err = Bot.Send(chat, &telebot.Animation{File: context.Message().Animation.File, Caption: context.Message().Caption})
		if Config.StreamChannel != 0 && strings.Contains(context.Message().Caption, "zavtracast/live") {
			Bot.Send(&telebot.Chat{ID: Config.StreamChannel}, &telebot.Animation{File: context.Message().Animation.File, Caption: context.Message().Caption})
		}
	case context.Message().Audio != nil:
		ChatMessage, err = Bot.Send(chat, &telebot.Audio{File: context.Message().Audio.File, Caption: context.Message().Caption})
		if Config.StreamChannel != 0 && strings.Contains(context.Message().Caption, "zavtracast/live") {
			Bot.Send(&telebot.Chat{ID: Config.StreamChannel}, &telebot.Audio{File: context.Message().Audio.File, Caption: context.Message().Caption})
		}
	case context.Message().Photo != nil:
		ChatMessage, err = Bot.Send(chat, &telebot.Photo{File: context.Message().Photo.File, Caption: GetHtmlText(*context.Message()), ParseMode: telebot.ModeHTML})
		if Config.StreamChannel != 0 && strings.Contains(context.Message().Caption, "zavtracast/live") {
			Bot.Send(&telebot.Chat{ID: Config.StreamChannel}, &telebot.Photo{File: context.Message().Photo.File, Caption: GetHtmlText(*context.Message()), ParseMode: telebot.ModeHTML})
		}
	case context.Message().Video != nil:
		ChatMessage, err = Bot.Send(chat, &telebot.Video{File: context.Message().Video.File, Caption: context.Message().Caption})
		if Config.StreamChannel != 0 && strings.Contains(context.Message().Caption, "zavtracast/live") {
			Bot.Send(&telebot.Chat{ID: Config.StreamChannel}, &telebot.Video{File: context.Message().Video.File, Caption: context.Message().Caption})
		}
	case context.Message().Voice != nil:
		ChatMessage, err = Bot.Send(chat, &telebot.Voice{File: context.Message().Voice.File, Caption: context.Message().Caption})
		if Config.StreamChannel != 0 && strings.Contains(context.Message().Caption, "zavtracast/live") {
			Bot.Send(&telebot.Chat{ID: Config.StreamChannel}, &telebot.Voice{File: context.Message().Voice.File, Caption: context.Message().Caption})
		}
	case context.Message().Document != nil:
		ChatMessage, err = Bot.Send(chat, &telebot.Document{File: context.Message().Document.File, Caption: context.Message().Caption})
		if Config.StreamChannel != 0 && strings.Contains(context.Message().Caption, "zavtracast/live") {
			Bot.Send(&telebot.Chat{ID: Config.StreamChannel}, &telebot.Document{File: context.Message().Document.File, Caption: context.Message().Caption})
		}
	default:
		ChatMessage, err = Bot.Send(chat, GetHtmlText(*context.Message()))
		if Config.StreamChannel != 0 && strings.Contains(context.Message().Text, "zavtracast/live") {
			Bot.Send(&telebot.Chat{ID: Config.StreamChannel}, GetHtmlText(*context.Message()))
		}
	}
	Forward.ForwardedMesssages = append(Forward.ForwardedMesssages, ForwardedMesssage{context.Message(), *ChatMessage})
	return err
}

//Edit reposted post
func EditRepost(context telebot.Context) error {
	var err error
	for _, ForwardedMesssage := range Forward.ForwardedMesssages {
		if ForwardedMesssage.ChannelMessage.ID == context.Message().ID {
			switch {
			case context.Message().Animation != nil:
				_, err = Bot.Edit(&ForwardedMesssage.ChatMessage, &telebot.Animation{File: context.Message().Animation.File, Caption: context.Message().Caption})
			case context.Message().Audio != nil:
				_, err = Bot.Edit(&ForwardedMesssage.ChatMessage, &telebot.Audio{File: context.Message().Audio.File, Caption: context.Message().Caption})
			case context.Message().Photo != nil:
				_, err = Bot.Edit(&ForwardedMesssage.ChatMessage, &telebot.Photo{File: context.Message().Photo.File, Caption: context.Message().Caption})
			case context.Message().Video != nil:
				_, err = Bot.Edit(&ForwardedMesssage.ChatMessage, &telebot.Video{File: context.Message().Video.File, Caption: context.Message().Caption})
			case context.Message().Voice != nil:
				_, err = Bot.Edit(&ForwardedMesssage.ChatMessage, &telebot.Voice{File: context.Message().Voice.File, Caption: context.Message().Caption})
			case context.Message().Document != nil:
				_, err = Bot.Edit(&ForwardedMesssage.ChatMessage, &telebot.Document{File: context.Message().Document.File, Caption: context.Message().Caption})
			default:
				_, err = Bot.Edit(&ForwardedMesssage.ChatMessage, GetHtmlText(*context.Message()))
			}
		}
	}
	return err
}

//Remove message
func Remove(context telebot.Context) error {
	context.Delete()
	time.Sleep(1 * time.Second)
	context.Delete()
	return nil
}

func GetNope() string {
	var nope Nope
	DB.Model(Nope{}).Order("RANDOM()").First(&nope)
	return nope.Text
}

func GetHtmlText(message telebot.Message) string {
	type entity struct {
		s string
		i int
	}

	entities := message.Entities
	textString := message.Text

	if len(message.Text) == 0 {
		entities = message.CaptionEntities
		textString = message.Caption
	}

	textString = strings.ReplaceAll(textString, "<", "˂")
	textString = strings.ReplaceAll(textString, ">", "˃")
	text := utf16.Encode([]rune(textString))

	ents := make([]entity, 0, len(entities)*2)

	for _, ent := range entities {
		var a, b string

		switch ent.Type {
		case telebot.EntityBold, telebot.EntityItalic,
			telebot.EntityUnderline, telebot.EntityStrikethrough:
			a = fmt.Sprintf("<%c>", ent.Type[0])
			b = a[:1] + "/" + a[1:]
		case telebot.EntityCode, telebot.EntityCodeBlock:
			a = fmt.Sprintf("<%s>", ent.Type)
			b = a[:1] + "/" + a[1:]
		case telebot.EntityTextLink:
			a = fmt.Sprintf("<a href='%s'>", ent.URL)
			b = "</a>"
		case telebot.EntityTMention:
			a = fmt.Sprintf("<a href='tg://user?id=%d'>", ent.User.ID)
			b = "</a>"
		default:
			continue
		}

		ents = append(ents, entity{a, ent.Offset})
		ents = append(ents, entity{b, ent.Offset + ent.Length})
	}

	// reverse entities
	for i, j := 0, len(ents)-1; i < j; i, j = i+1, j-1 {
		ents[i], ents[j] = ents[j], ents[i]
	}

	for _, ent := range ents {
		r := utf16.Encode([]rune(ent.s))
		text = append(text[:ent.i], append(r, text[ent.i:]...)...)
	}

	textString = string(utf16.Decode(text))

	if len(message.Entities) != 0 && message.Entities[0].Type == telebot.EntityCommand {
		if textString[1:4] == "set" {
			textString = strings.Join(strings.Split(textString, " ")[2:], " ")
		} else {
			textString = textString[message.Entities[0].Length+1:]
		}
	}

	return textString
}
