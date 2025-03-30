package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	cntx "context"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/neonxp/StemmerRu"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	ffprobe "gopkg.in/vansante/go-ffprobe.v2"
	"gorm.io/gorm/clause"
)

type Media struct {
	Type                  string
	FileID                string
	FileName              string
	FilePath              string
	FileSize              int64
	Height                int64
	Width                 int64
	Duration              int64
	Caption               string
	CaptionEntities       []gotgbot.MessageEntity
	HasSpoiler            bool
	ShowCaptionAboveMedia bool
}

var onlyWords = regexp.MustCompile(`[,.!?]+`)

func UserFullName(user *gotgbot.User) string {
	fullname := user.FirstName
	if user.LastName != "" {
		fullname = fmt.Sprintf("%v %v", user.FirstName, user.LastName)
	}
	return fullname
}

func UserName(user *gotgbot.User) string {
	username := user.Username
	if user.Username == "" {
		username = UserFullName(user)
	}
	return username
}

func MentionUser(user *gotgbot.User) string {
	return fmt.Sprintf("<a href=\"tg://user?id=%v\">%v</a>", user.Id, UserFullName(user))
}

func RandInt(min int, max int) int {
	b, err := rand.Int(rand.Reader, big.NewInt(int64(max+1)))
	if err != nil {
		return 0
	}
	return min + int(b.Int64())
}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func IsAdmin(userid int64) bool {
	for _, b := range Config.Admins {
		if b == userid {
			return true
		}
	}
	return false
}

func IsAdminOrModer(userid int64) bool {
	for _, b := range Config.Admins {
		if b == userid {
			return true
		}
	}
	for _, b := range Config.Moders {
		if b == userid {
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

func FindUserInMessage(context ext.Context) (gotgbot.User, int64, error) {
	var user gotgbot.User
	var err error = nil
	var untildate = time.Now().Unix() + 86400
	for _, entity := range context.Message.Entities {
		if entity.Type == "text_mention" {
			user = *entity.User
			if len(context.Args()) == 3 {
				addtime, err := strconv.ParseInt(context.Args()[2], 10, 64)
				if err != nil {
					return user, untildate, err
				}
				untildate += addtime - 86400
			}
			return user, untildate, err
		}
	}
	if context.Message.ReplyToMessage != nil {
		user = *context.Message.ReplyToMessage.From
		if len(context.Args()) == 2 {
			addtime, err := strconv.ParseInt(context.Args()[1], 10, 64)
			if err != nil {
				return user, untildate, errors.New("время указано неверно")
			}
			untildate += addtime - 86400
		}
	} else {
		if len(context.Args()) == 1 {
			err = errors.New("пользователь не найден")
			return user, untildate, err
		}
		if user.Id == 0 {
			user, err = GetUserFromDB(context.Args()[1])
			if err != nil {
				return user, untildate, err
			}
		}
		if len(context.Args()) == 3 {
			addtime, err := strconv.ParseInt(context.Args()[2], 10, 64)
			if err != nil {
				return user, untildate, errors.New("время указано неверно")
			}
			untildate += addtime - 86400
		}
	}
	return user, untildate, err
}

func GetUserFromDB(findstring string) (gotgbot.User, error) {
	var user gotgbot.User
	var err error = nil
	if string(findstring[0]) == "@" {
		user.Username = findstring[1:]
	} else {
		user.Id, err = strconv.ParseInt(findstring, 10, 64)
	}
	result := DB.Where("lower(username) = ? OR id = ?", strings.ToLower(user.Username), user.Id).First(user)
	if result.Error != nil {
		err = result.Error
	}
	return user, err
}

// Forward channel post to chat
func ForwardPost(bot *gotgbot.Bot, context *ext.Context) error {
	if context.Message == nil || context.Message.Chat.Id != Config.Channel {
		return nil
	}
	var err error
	if context.Message.Text != "" || context.Message.Caption != "" {
		_, err = Bot.ForwardMessage(Config.Chat, context.Message.Chat.Id, context.Message.MessageId, nil)
	}
	if Config.StreamChannel != 0 {
		if strings.Contains(context.Message.Text, "zavtracast/live") {
			_, err = Bot.ForwardMessage(Config.StreamChannel, context.Message.Chat.Id, context.Message.MessageId, nil)
			return err
		}
		for _, entity := range append(context.Message.CaptionEntities, context.Message.Entities...) {
			if entity.Type == "url" || entity.Type == "text_link" {
				if strings.Contains(entity.Url, "zavtracast/live") {
					_, err = Bot.ForwardMessage(Config.StreamChannel, context.Message.Chat.Id, context.Message.MessageId, nil)
					return err
				}
			}
		}
	}
	return err
}

// Remove message
func Remove(bot *gotgbot.Bot, context *ext.Context) error {
	_, err := context.Message.Delete(bot, nil)
	return err
}

func OnChatMember(bot *gotgbot.Bot, context *ext.Context) error {
	if context.Message.Chat.Id == Config.ReserveChat {
		context.EffectiveChat.Unban(bot, context.Message.From.Id, nil)
	}
	//User update
	UserResult := DB.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(context.ChatMember.NewChatMember.GetUser())
	if UserResult.Error != nil {
		ErrorReporting(UserResult.Error)
	}
	return nil
}

func OnUserJoined(bot *gotgbot.Bot, context *ext.Context) error {
	if context.Message.Chat.Id == Config.ReserveChat {
		context.Message.Delete(bot, nil)
	}
	return nil
}

func OnUserLeft(bot *gotgbot.Bot, context *ext.Context) error {
	if context.Message.Chat.Id == Config.ReserveChat {
		context.Message.Delete(bot, nil)
	}
	return nil
}

func OnText(bot *gotgbot.Bot, context *ext.Context) error {
	//remove message from reservechat
	if context.Message.Chat.Id == Config.ReserveChat {
		context.Message.Delete(bot, nil)
	}

	//User update
	UserResult := DB.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(context.Message.From)
	if UserResult.Error != nil {
		ErrorReporting(UserResult.Error)
	}

	//update StatsDays(1), StatsHours(2), StatsUsers(3), StatsWords(4), StatsWeekday(5)
	startOfDay := GetStartOfDay()
	timeNow := time.Now().Local()
	statsIncrease(1, startOfDay, int64(timeNow.Day()))
	statsIncrease(2, startOfDay, int64(timeNow.Hour()))
	statsIncrease(3, startOfDay, context.Message.From.Id)
	statsIncrease(5, startOfDay, int64(timeNow.Weekday()))
	if context.Message.Text == "" || string(context.Message.Text[0]) == "/" {
		return nil
	}
	text := strings.ToLower(onlyWords.ReplaceAllString(context.Message.Text, ""))
	for _, word := range strings.Split(text, " ") {
		if len([]rune(word)) > 0 {
			statsIncrease(4, startOfDay, getWordID(word))
		}
	}
	return nil
}

func statsIncrease(statType int64, dayTimestamp int64, contextID int64) {
	if DB.Exec("UPDATE stats SET count = count + 1, last_update = ? WHERE context_id = ? AND stat_type = ? AND day_timestamp = ?;", time.Now().Local().Unix(), contextID, statType, dayTimestamp).RowsAffected == 0 {
		DB.Create(Stats{StatType: statType, DayTimestamp: dayTimestamp, ContextID: contextID, Count: 1, LastUpdate: time.Now().Local().Unix()})
	}
}

func getWordID(searchWord string) int64 {
	shortWord := StemmerRu.Stem(searchWord)
	wordResult := StatsWords{}
	if DB.Model(StatsWords{}).Select("id").Where("short_word = ?", shortWord).Find(&wordResult).RowsAffected == 0 {
		wordResult.ShortWord = shortWord
		wordResult.Word = searchWord
		DB.Create(&wordResult)
	}
	return wordResult.ID
}

func GetStartOfDay() int64 {
	unixTS := time.Now().Local().Unix()
	tm := time.Unix(unixTS, 0).In(time.Local)
	hour, minute, second := tm.Clock()

	return unixTS - int64(hour*3600+minute*60+second)
}

func GetNope() string {
	var nope Nope
	DB.Model(Nope{}).Order("RANDOM()").First(&nope)
	return nope.Text
}

func GetBless() string {
	var bless Bless
	DB.Model(Bless{}).Order("RANDOM()").First(&bless)
	return bless.Text
}

func FFmpegConvert(context *ext.Context, media Media, targetType string) error {
	var KwArgs ffmpeg.KwArgs
	var extension string
	var height int
	var width int
	var duration float64
	var err error

	videoKwArgs := ffmpeg.KwArgs{}
	defaultKwArgs := ffmpeg.KwArgs{"loglevel": "fatal", "hide_banner": ""}
	name := media.FileName

	ctx, cancelFn := cntx.WithTimeout(cntx.Background(), 5*time.Second)
	defer cancelFn()

	data, err := ffprobe.ProbeURL(ctx, media.FilePath)
	if err != nil {
		return err
	}
	inputVideoFormat := data.FirstVideoStream()
	inputAudioFormat := data.FirstAudioStream()

	if inputVideoFormat != nil {
		width = inputVideoFormat.Width
		height = inputVideoFormat.Height
		duration, err = strconv.ParseFloat(inputVideoFormat.Duration, 32)
		if err != nil {
			duration = 0
		}
	} else {
		switch targetType {
		case "animation", "gif", "webm", "animation_reverse", "sticker_reverse", "animation_loop", "loop":
			return fmt.Errorf("видео-дорожка не найдена")
		case "video", "mp4":
			targetType = "audio"
		case "video_reverse", "reverse", "invert":
			targetType = "audio_reverse"
		}
	}

	if inputAudioFormat != nil {
		if duration == 0 {
			duration, err = strconv.ParseFloat(inputAudioFormat.Duration, 32)
			if err != nil {
				duration = 0
			}
		}
	} else {
		switch targetType {
		case "audio", "mp3", "voice", "ogg", "audio_reverse", "voice_reverse", "audio_loop", "voice_loop":
			return fmt.Errorf("аудио-дорожка не найдена")
		case "video", "mp4", "webm":
			targetType = "animation"
		case "video_reverse", "reverse", "invert":
			targetType = "animation_reverse"
		case "video_loop", "loop":
			targetType = "animation_loop"
		}
	}

	if (strings.Contains(targetType, "reverse") || strings.Contains(targetType, "loop")) && duration > 120 {
		return fmt.Errorf("слишком длинное видео для эффекта")
	}

	if inputAudioFormat == nil && inputVideoFormat == nil {
		return fmt.Errorf("медиа-дорожек не найдено")
	}

	switch targetType {
	case "audio", "mp3":
		KwArgs = ffmpeg.KwArgs{"vn": "", "c:a": "libmp3lame"}
		extension = "mp3"
		targetType = "audio"
	case "voice", "ogg":
		KwArgs = ffmpeg.KwArgs{"vn": "", "c:a": "libopus"}
		extension = "ogg"
		targetType = "voice"
	case "photo", "jpg":
		KwArgs = ffmpeg.KwArgs{"vf": "select=eq(n\\,0)", "format": "image2"}
		extension = "jpg"
		targetType = "photo"
	case "sticker", "webp":
		KwArgs = ffmpeg.KwArgs{"vf": "select=eq(n\\,0)", "format": "image2"}
		extension = "webp"
		targetType = "sticker"
	case "animation", "gif":
		KwArgs = ffmpeg.MergeKwArgs([]ffmpeg.KwArgs{videoKwArgs, {"an": ""}})
		extension = "mp4"
		targetType = "animation"
	case "video", "mp4":
		KwArgs = ffmpeg.MergeKwArgs([]ffmpeg.KwArgs{videoKwArgs, {"c:a": "aac"}})
		extension = "mp4"
		targetType = "video"
	case "video_reverse", "reverse", "invert":
		KwArgs = ffmpeg.MergeKwArgs([]ffmpeg.KwArgs{videoKwArgs, {"c:a": "aac", "vf": "reverse", "af": "areverse"}})
		extension = "mp4"
		targetType = "video"
	case "animation_reverse":
		KwArgs = ffmpeg.MergeKwArgs([]ffmpeg.KwArgs{videoKwArgs, {"an": "", "vf": "reverse"}})
		extension = "mp4"
		targetType = "animation"
	case "sticker_reverse", "webm":
		KwArgs = ffmpeg.KwArgs{"c:v": "libvpx-vp9", "an": "", "vf": "reverse"}
		extension = "mp4"
		targetType = "sticker"
	case "audio_reverse":
		KwArgs = ffmpeg.KwArgs{"vn": "", "c:a": "libmp3lame", "af": "areverse"}
		extension = "mp3"
		targetType = "audio"
	case "voice_reverse":
		KwArgs = ffmpeg.KwArgs{"vn": "", "c:a": "libopus", "af": "areverse"}
		extension = "ogg"
		targetType = "voice"
	case "animation_loop", "loop":
		KwArgs = ffmpeg.MergeKwArgs([]ffmpeg.KwArgs{videoKwArgs, {"an": "", "filter_complex": "[0]reverse[r];[0][r]concat,loop=1:2,setpts=PTS-STARTPTS"}})
		extension = "mp4"
		targetType = "animation"
	default:
		return fmt.Errorf("targetType %v not supported", targetType)
	}

	resultFilePath := fmt.Sprintf("%v/%v_converted.%v", os.TempDir(), name, extension)
	resultFileUrl := fmt.Sprintf("file://%v/%v_converted.%v", os.TempDir(), name, extension)

	err = ffmpeg.Input(media.FilePath).Output(resultFilePath, ffmpeg.MergeKwArgs([]ffmpeg.KwArgs{defaultKwArgs, KwArgs})).OverWriteOutput().ErrorToStdOut().Run()
	if err != nil {
		return err
	}

	defer func(resultFilePath string) {
		os.Remove(resultFilePath)
	}(resultFilePath)

	switch targetType {
	case "video":
		_, err := Bot.SendVideo(context.Message.Chat.Id, gotgbot.InputFileByURL(resultFileUrl), &gotgbot.SendVideoOpts{
			SupportsStreaming: true,
			Width:             int64(width),
			Height:            int64(height),
			Duration:          int64(duration),
			ReplyParameters:   &gotgbot.ReplyParameters{MessageId: context.Message.MessageId, AllowSendingWithoutReply: true},
		})
		return err
	case "animation":
		_, err := Bot.SendAnimation(context.Message.Chat.Id, gotgbot.InputFileByURL(resultFileUrl), &gotgbot.SendAnimationOpts{
			Width:           int64(width),
			Height:          int64(height),
			Duration:        int64(duration),
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.MessageId, AllowSendingWithoutReply: true},
		})
		return err
	case "audio":
		_, err := Bot.SendAudio(context.Message.Chat.Id, gotgbot.InputFileByURL(resultFileUrl), &gotgbot.SendAudioOpts{
			Duration:        int64(duration),
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.MessageId, AllowSendingWithoutReply: true},
		})
		return err
	case "voice":
		_, err := Bot.SendVoice(context.Message.Chat.Id, gotgbot.InputFileByURL(resultFileUrl), &gotgbot.SendVoiceOpts{
			Duration:        int64(duration),
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.MessageId, AllowSendingWithoutReply: true},
		})
		return err
	default:
		_, err := Bot.SendDocument(context.Message.Chat.Id, gotgbot.InputFileByURL(resultFileUrl), &gotgbot.SendDocumentOpts{
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.MessageId, AllowSendingWithoutReply: true},
		})
		return err
	}
}

func DownloadFile(filepath string, url string) (err error) {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func ReplyAndRemove(message string, context ext.Context) error {
	message += "\n\nЭто сообщение самоуничтожится через 30 секунд."
	sentMessage, err := Bot.SendMessage(context.Message.Chat.Id, message, &gotgbot.SendMessageOpts{ParseMode: "HTML", ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.MessageId, AllowSendingWithoutReply: true}})
	if err != nil {
		return err
	}
	if context.Message == nil || context.Message.MessageId == 0 {
		return nil
	}
	go func(messages []int64) {
		time.Sleep(30 * time.Second)
		Bot.DeleteMessages(context.Message.Chat.Id, messages, nil)
	}([]int64{context.Message.MessageId, sentMessage.MessageId})
	return nil
}

func IsContainsMedia(message *gotgbot.Message) bool {
	switch {
	case message.Photo != nil:
		return true
	case message.Voice != nil:
		return true
	case message.Audio != nil:
		return true
	case message.Animation != nil:
		return true
	case message.Sticker != nil:
		return true
	case message.Document != nil:
		return true
	case message.Video != nil:
		return true
	case message.VideoNote != nil:
		return true
	default:
		return false
	}
}

func GetMedia(message *gotgbot.Message) (Media, error) {
	var result Media

	switch {
	case message.Photo != nil:
		result.Type = "photo"
		result.FileID = message.Photo[0].FileId
		result.Height = message.Photo[0].Height
		result.Width = message.Photo[0].Width
		result.FileSize = message.Photo[0].FileSize
	case message.Voice != nil:
		result.Type = "voice"
		result.FileID = message.Voice.FileId
		result.Duration = message.Voice.Duration
		result.FileSize = message.Voice.FileSize
	case message.Audio != nil:
		result.Type = "audio"
		result.FileID = message.Audio.FileId
		result.Duration = message.Audio.Duration
		result.FileSize = message.Audio.FileSize
	case message.Animation != nil:
		result.Type = "animation"
		result.FileID = message.Animation.FileId
		result.Height = message.Animation.Height
		result.Width = message.Animation.Width
		result.Duration = message.Animation.Duration
		result.FileSize = message.Animation.FileSize
	case message.Sticker != nil:
		result.Type = "sticker"
		result.FileID = message.Sticker.FileId
		result.Height = message.Sticker.Height
		result.Width = message.Sticker.Width
		result.FileSize = message.Sticker.FileSize
	case message.Video != nil:
		result.Type = "video"
		result.FileID = message.Video.FileId
		result.Height = message.Video.Height
		result.Width = message.Video.Width
		result.Duration = message.Video.Duration
		result.FileSize = message.Video.FileSize
	case message.VideoNote != nil:
		result.Type = "video_note"
		result.FileID = message.VideoNote.FileId
		result.Duration = message.VideoNote.Duration
		result.FileSize = message.VideoNote.FileSize
	default:
		result.Type = "document"
		result.FileID = message.Document.FileId
		result.FileSize = message.Document.FileSize
	}
	result.Caption = message.Caption
	result.CaptionEntities = message.CaptionEntities
	result.HasSpoiler = message.HasMediaSpoiler
	result.ShowCaptionAboveMedia = message.ShowCaptionAboveMedia

	file, err := Bot.GetFile(result.FileID, nil)
	if err != nil {
		return Media{}, err
	}

	result.FilePath = file.FilePath
	result.FileName = filepath.Base(file.FilePath)

	return result, nil
}
