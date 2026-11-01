package main

import (
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm"
)

func birthdayCommandContext(ctx *ext.Context) bool {
	return ctx.EffectiveUser != nil && ctx.EffectiveMessage != nil && ctx.EffectiveChat != nil && (ctx.EffectiveChat.Id == Config.Chat || ctx.EffectiveChat.Type == "private")
}

func AddBirthday(bot *gotgbot.Bot, ctx *ext.Context) error {
	if !birthdayCommandContext(ctx) {
		return nil
	}
	args := strings.Fields(ctx.EffectiveMessage.Text)
	if len(args) != 2 {
		return sendZBHelp(bot, ctx, "Укажи свой день рождения: /addbirthday ДД.ММ. Например: /addbirthday 23.06")
	}
	date, err := time.Parse("02.01.2006", args[1]+".2000")
	if err != nil {
		return sendZBHelp(bot, ctx, "Неверная дата. Используй ДД.ММ, например 23.06.")
	}
	added, err := addBirthday(&DB, *ctx.EffectiveUser, date.Day(), int(date.Month()))
	if err != nil {
		return err
	}
	if !added {
		return sendZBHelp(bot, ctx, "день рождения уже в базе")
	}
	return sendZBHelp(bot, ctx, "день рождения добавлен!")
}

func DelBirthday(bot *gotgbot.Bot, ctx *ext.Context) error {
	if !birthdayCommandContext(ctx) {
		return nil
	}
	deleted, err := deleteBirthday(&DB, *ctx.EffectiveUser)
	if err != nil {
		return err
	}
	if !deleted {
		return sendZBHelp(bot, ctx, "дня рождения нет в базе")
	}
	return sendZBHelp(bot, ctx, "день рождения удален!")
}

func WhenYourBirthday(bot *gotgbot.Bot, ctx *ext.Context) error {
	if !birthdayCommandContext(ctx) {
		return nil
	}
	args := strings.Fields(ctx.EffectiveMessage.Text)
	if len(args) != 2 || !strings.HasPrefix(args[1], "@") {
		return sendZBHelp(bot, ctx, "Укажи ник: /whenyourbirthday @username")
	}
	username := strings.ToLower(strings.TrimPrefix(args[1], "@"))
	var b Birthday
	// Prefer the current cached Telegram identity. Once bound, the old handle
	// is not allowed to identify another person's birthday after a rename.
	u, err := GetUserFromDB("@" + username)
	if strings.EqualFold(ctx.EffectiveUser.Username, username) {
		u, err = *ctx.EffectiveUser, nil
	}
	if err == nil {
		b, err = findOwnBirthday(&DB, u, false)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		err = DB.Where("username = ? AND user_id IS NULL", username).First(&b).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && b.Deleted) {
		return sendZBHelp(bot, ctx, "дня рождения нет в базе")
	}
	if err != nil {
		return err
	}
	return sendZBHelp(bot, ctx, fmt.Sprintf("@%s — %02d.%02d", html.EscapeString(username), b.Day, b.Month))
}

func Birthdays(bot *gotgbot.Bot, ctx *ext.Context) error {
	if !birthdayCommandContext(ctx) {
		return nil
	}
	now := time.Now()
	rows, err := upcomingBirthdays(&DB, now)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return sendZBHelp(bot, ctx, "В базе пока нет дней рождения.")
	}
	var text strings.Builder
	text.WriteString("🎂 <b>10 ближайших дней рождения</b>\n")
	for _, b := range rows {
		name, label := b.Name, ""
		if !strings.HasPrefix(b.Username, "id:") {
			label = " (@" + b.Username + ")"
		}
		if b.UserID != nil {
			var u gotgbot.User
			if err := DB.First(&u, *b.UserID).Error; err == nil {
				name = UserFullName(&u)
				label = ""
				if u.Username != "" {
					label = " (@" + u.Username + ")"
				}
			}
		}
		fmt.Fprintf(&text, "%s — %s%s\n", nextBirthday(b, now).Format("02.01"), html.EscapeString(name), html.EscapeString(label))
	}
	return sendZBHelp(bot, ctx, text.String())
}

func TestBirthday(bot *gotgbot.Bot, ctx *ext.Context) error {
	if !birthdayCommandContext(ctx) {
		return nil
	}
	if !canUseQuote(bot, ctx.EffectiveUser.Id) {
		return sendZBHelp(bot, ctx, "Команда доступна только администраторам.")
	}
	u, err := GetUserFromDB("@nakxwest")
	if err != nil {
		return sendZBHelp(bot, ctx, "Не удалось найти @nakxwest в базе пользователей бота.")
	}
	present, err := birthdayMember(bot, Config.Chat, u.Id)
	if err != nil {
		return err
	}
	if present == nil {
		return sendZBHelp(bot, ctx, "@nakxwest сейчас не состоит в чате.")
	}
	return sendBirthdayCard(bot, Config.Chat, *present)
}
