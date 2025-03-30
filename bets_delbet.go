package main

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Remove bet
func DelBet(bot *gotgbot.Bot, context *ext.Context) error {
	var bet Bets
	if len(context.Args()) < 3 {
		return ReplyAndRemove("Пример использования: <code>/delbet 30.06.2023 ставлю жопу, что TESVI будет говном</code>", *context)
	}
	date, err := time.Parse("02.01.2006", context.Args()[1])
	if err != nil {
		return err
	}
	bet.UserID = context.Message.From.Id
	bet.Timestamp = date.Unix()
	bet.Text = strings.Join(context.Args()[2:], " ")
	result := DB.Delete(&bet)
	if result.RowsAffected != 0 {
		return ReplyAndRemove(fmt.Sprintf("Ставка удалена:\n%v, %v:<pre>%v</pre>\n", time.Unix(bet.Timestamp, 0).Format("02.01.2006"), UserFullName(context.Message.From), html.EscapeString(bet.Text)), *context)
	} else {
		return ReplyAndRemove("Твоя ставка не найдена по указанным параметрам.", *context)
	}
}
