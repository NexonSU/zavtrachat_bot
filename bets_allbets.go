package main

import (
	"errors"
	"fmt"
	"html"
	"strconv"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm"
)

// List all bets
func AllBets(bot *gotgbot.Bot, context *ext.Context) error {
	var betlist string
	var bet Bets
	var user gotgbot.User
	var listUnits int
	var from int64
	var to int64
	// "all" used to be overwritten by the next line, making the option a no-op.
	from = betCalendarDay(time.Now()).Unix()
	if len(context.Args()) > 1 && context.Args()[1] == "all" {
		from = 0
	}
	to = time.Now().Local().Add(43800 * time.Hour).Unix()
	result, err := DB.Model(&Bets{}).Where("timestamp >= ? AND timestamp < ?", from, to).Order("timestamp ASC").Rows()
	if err != nil {
		return err
	}
	defer result.Close()
	for result.Next() {
		err := DB.ScanRows(result, &bet)
		if err != nil {
			return err
		}
		user, err = GetUserFromDB(strconv.FormatInt(bet.UserID, 10))
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		name := UserFullName(&user)
		if name == "" {
			name = bet.AuthorName
		}
		if name == "" {
			name = strconv.FormatInt(bet.UserID, 10)
		}
		created := "Дата создания неизвестна"
		if bet.CreatedTimestamp > 0 {
			created = "Записана " + time.Unix(bet.CreatedTimestamp, 0).In(betMoscow).Format("02.01.2006 15:04") + " МСК"
		}
		header := fmt.Sprintf("%v, %v (%s):\n", time.Unix(bet.Timestamp, 0).UTC().Format("02.01.2006"), name, created)
		for _, part := range betReminderParts(bet.Text) {
			units := betTextUnits(header) + betTextUnits(part) + 1
			if betlist != "" && listUnits+units > 3900 {
				if err := Reply(betlist, *context); err != nil {
					return err
				}
				betlist, listUnits = "", 0
			}
			betlist += html.EscapeString(header) + "<pre>" + html.EscapeString(part) + "</pre>\n"
			listUnits += units
		}
	}
	if err := result.Err(); err != nil {
		return err
	}
	return Reply(betlist, *context)
}
