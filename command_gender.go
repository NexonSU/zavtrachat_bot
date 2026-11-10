package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserPhraseGender struct {
	UserID int64 `gorm:"primaryKey;autoIncrement:false"`
	Female bool  `gorm:"not null"`
}

func phraseGender(userID int64) (bool, error) {
	var setting UserPhraseGender
	result := DB.Where("user_id = ?", userID).Find(&setting)
	return setting.Female, result.Error
}

// The upsert toggles atomically and returns the value from this invocation.
func swapPhraseGender(userID int64) (bool, error) {
	setting := UserPhraseGender{UserID: userID, Female: true}
	result := DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"female": gorm.Expr("NOT user_phrase_genders.female"),
		}),
	}, clause.Returning{}).Create(&setting)
	return setting.Female, result.Error
}

func canManagePhraseGender(bot *gotgbot.Bot, userID int64) bool {
	return IsAdmin(userID) || userID == Config.SysAdmin || isTelegramChatAdmin(bot, userID)
}

func GenderCheck(bot *gotgbot.Bot, ctx *ext.Context) error {
	return genderCommand(bot, ctx, "gendercheck", false)
}

func GenderSwap(bot *gotgbot.Bot, ctx *ext.Context) error {
	return genderCommand(bot, ctx, "genderswap", true)
}

func genderCommand(bot *gotgbot.Bot, ctx *ext.Context, command string, swap bool) error {
	chat, sender, msg := ctx.EffectiveChat, ctx.EffectiveUser, ctx.EffectiveMessage
	if chat == nil || sender == nil || msg == nil {
		return nil
	}
	inPrivate := chat.Type == "private" && chat.Id == sender.Id
	inMainChat := chat.Id == Config.Chat && (chat.Type == "group" || chat.Type == "supergroup")
	if (!inPrivate && !inMainChat) || !canManagePhraseGender(bot, sender.Id) {
		return nil
	}
	var opts *gotgbot.SendMessageOpts
	if inMainChat {
		opts = &gotgbot.SendMessageOpts{
			EphemeralMessageParameters: &gotgbot.EphemeralMessageParameters{ReceiverUserId: sender.Id},
		}
		if msg.EphemeralMessageId == 0 && msg.MessageId != 0 {
			_, _ = bot.DeleteMessage(chat.Id, msg.MessageId, nil)
		}
	}
	reply := func(text string) error {
		_, err := bot.SendMessage(chat.Id, text, opts)
		return err
	}
	args := ctx.Args()
	if len(args) != 2 || !strings.HasPrefix(args[1], "@") || len(args[1]) < 2 {
		return reply("Формат команды: /" + command + " @ник")
	}
	cached, err := GetUserFromDB(args[1])
	if err != nil {
		return reply("Участник не найден. Пусть напишет сообщение в основном чате, затем повтори команду.")
	}
	request, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	member, err := bot.GetChatMemberWithContext(request, Config.Chat, cached.Id, nil)
	if err != nil {
		return reply("Не удалось проверить участника основного чата.")
	}
	user := member.GetUser()
	if user.IsBot || member.GetStatus() == "left" || member.GetStatus() == "kicked" ||
		!strings.EqualFold("@"+user.Username, args[1]) {
		return reply("Участник с таким ником не найден в основном чате.")
	}

	var female bool
	now := ""
	if swap {
		female, err = swapPhraseGender(user.Id)
		now = " теперь"
	} else {
		female, err = phraseGender(user.Id)
	}
	if err != nil {
		return reply("Не удалось прочитать или сохранить настройку. Попробуй позже.")
	}
	gender := "мальчик"
	if female {
		gender = "девочка"
	}
	return reply(fmt.Sprintf("@%s%s %s", user.Username, now, gender))
}
