package main

import (
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// List add pidors from DB on /pidorlist
func Pidorlist(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		return Denied(bot, context)
	}
	var pidorlist string
	var pidor PidorList
	var i = 0
	var err error
	result, err := DB.Model(&PidorList{}).Rows()
	if err != nil {
		return err
	}
	defer result.Close()
	for result.Next() {
		err := DB.ScanRows(result, &pidor)
		if err != nil {
			return err
		}
		i++
		pidorlist += strconv.Itoa(i) + ". @" + pidor.Username + " (" + strconv.FormatInt(pidor.Id, 10) + ")\n"
		if len(pidorlist) > 3900 {
			err = Reply(pidorlist, *context)
			if err != nil {
				return err
			}
			pidorlist = ""
		}
	}
	return Reply(pidorlist, *context)
}
