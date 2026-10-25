package main

// User names, mentions, the admin lists and the user lookup by mention, reply, username or id.

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

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
	return fmt.Sprintf("<a href=\"tg://user?id=%v\">%v</a>", user.Id, html.EscapeString(UserFullName(user)))
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

func FindUserInMessage(message gotgbot.Message) (gotgbot.User, error) {
	var user gotgbot.User
	var err error
	for _, entity := range message.Entities {
		if entity.Type == "text_mention" && entity.User != nil {
			return *entity.User, err
		}
	}
	if message.ReplyToMessage != nil && message.ReplyToMessage.From != nil {
		return *message.ReplyToMessage.From, nil
	} else {
		for _, arg := range strings.Split(message.Text, " ") {
			user, err = GetUserFromDB(arg)
			if err != nil {
				continue
			} else {
				return user, err
			}
		}
		return user, fmt.Errorf("пользователь не найден в сообщении")
	}
}

func GetUserFromDB(findstring string) (gotgbot.User, error) {
	var err error = nil
	user := &gotgbot.User{}
	if findstring == "" {
		return *user, fmt.Errorf("пользователь не найден")
	}
	if string(findstring[0]) == "@" {
		user.Username = findstring[1:]
	} else {
		user.Id, err = strconv.ParseInt(findstring, 10, 64)
		if err != nil {
			return *user, err
		}
	}
	if user.Username == "" && user.Id == 0 {
		return *user, fmt.Errorf("пользователь не найден")
	}
	result := DB.Where(" lower(username) = ? OR id = ?", strings.ToLower(user.Username), user.Id).Last(user)
	if result.Error != nil {
		err = result.Error
	}
	return *user, err
}
