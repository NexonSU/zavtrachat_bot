package main

import (
	"os"

	tele "gopkg.in/telebot.v3"
)

// Restart bot on /restart
func Restart(context tele.Context) error {
	Bot.Delete(context.Message())
	os.Exit(0)
	return nil
}
