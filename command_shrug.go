package main

import tele "gopkg.in/telebot.v3"

// Send shrug in chat on /shrug
func Shrug(context tele.Context) error {
	return context.Send("¯\\_(ツ)_/¯")
}
