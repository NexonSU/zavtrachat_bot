package main

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Kill user on /blessing, /suicide
func TestRandom(bot *gotgbot.Bot, context *ext.Context) error {
	if !IsAdminOrModer(context.Message.From.Id) {
		return KillSender(bot, context)
	}
	text := "1000xRandInt(0, 9):\n"
	numbers := [10]int{}
	for i := 0; i < 1000; i++ {
		numbers[RandInt(0, 9)] += 1
	}
	for number, count := range numbers {
		text = fmt.Sprintf("%v%v - %v\n", text, number, count)
	}
	return ReplyAndRemoveWithTarget(text, *context)
}
