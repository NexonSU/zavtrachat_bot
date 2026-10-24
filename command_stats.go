package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm"
)

// statsTopKinds maps the /stats argument to the stat type it ranks and the
// title of the top. Aliases share the stat type but keep their own wording:
// likes/reactions, mutes/blessings, mutetime/blesstime.
var statsTopKinds = map[string]struct {
	statType int64
	title    string
}{
	"saves":     {salvationStatType, "по спасениям"},
	"likes":     {6, "по лайкам"},
	"reactions": {6, "по лайкам"},
	"mutes":     {8, "по смертям"},
	"blessings": {8, "по блесингам"},
	"mutetime":  {7, "по времени в муте"},
	"blesstime": {7, "по времени в блесинге"},
	"messages":  {3, "по числу сообщений"},
	"words":     {4, "по словам"},
}

// chatMessageLink builds a t.me link to a message of the chat: private
// chats use the numeric "c/" form with the -100 prefix stripped.
func chatMessageLink(chatID int64, messageID int64) string {
	return fmt.Sprintf("https://t.me/c/%v/%v", strings.TrimPrefix(strconv.FormatInt(chatID, 10), "-100"), messageID)
}

// StatsTop answers /stats <top> [day|week] with a top-10 from the stats
// table: likes (stat type 6, counted per message), deaths (stat type 8),
// mute time in seconds (stat type 7), messages (stat type 3) and words
// (stat type 4, context_id is the StatsWords id). Without arguments it
// replies with the list of tops.
func StatsTop(bot *gotgbot.Bot, context *ext.Context) error {
	// context.Args() starts with the command itself: args[1] is the top,
	// args[2] the optional period.
	args := context.Args()
	if len(args) < 2 || args[1] == "" {
		return Reply("Топы: <code>/stats likes</code>, <code>/stats mutes</code>, <code>/stats mutetime</code>, <code>/stats messages</code>, <code>/stats words</code>, <code>/stats saves</code>\nПериод: <code>day</code> (сутки) или <code>week</code> (неделя), без периода — за всё время.", *context)
	}
	kind, ok := statsTopKinds[strings.ToLower(args[1])]
	if !ok {
		return Reply(fmt.Sprintf("Непонятный топ: <code>%v</code>.\nДоступные топы: likes, reactions, mutes, blessings, mutetime, blesstime, messages, words, saves.", args[1]), *context)
	}
	where := "stat_type = ?"
	whereArgs := []any{kind.statType}
	period := "за всё время"
	if len(args) > 2 {
		switch strings.ToLower(args[2]) {
		case "day":
			where += " AND day_timestamp = ?"
			whereArgs = append(whereArgs, GetStartOfDay())
			period = "за сутки"
		case "week":
			where += " AND day_timestamp >= ?"
			whereArgs = append(whereArgs, GetStartOfDay()-6*86400)
			period = "за неделю"
		default:
			return Reply(fmt.Sprintf("Непонятный период: <code>%v</code>.\nПериод: <code>day</code> (сутки) или <code>week</code> (неделя).", args[2]), *context)
		}
	}

	type topRow struct {
		ContextID int64
		Total     int64
		Word      string
	}
	var rows []topRow
	var query *gorm.DB
	if kind.statType == 4 {
		// Words are keyed by the StatsWords id, so the top joins the
		// dictionary to show the word itself.
		query = DB.Table("stats").
			Select("stats.context_id", "SUM(stats.count) AS total", "stats_words.word").
			Joins("JOIN stats_words ON stats_words.id = stats.context_id").
			Where(where, whereArgs...).
			Group("stats.context_id, stats_words.word")
	} else {
		query = DB.Model(&Stats{}).
			Select("context_id", "SUM(count) AS total").
			Where(where, whereArgs...).
			Group("context_id")
	}
	if err := query.Order("total DESC").Limit(10).Scan(&rows).Error; err != nil {
		return err
	}

	text := fmt.Sprintf("Топ-10 %v %v:\n\n", kind.title, period)
	for i, row := range rows {
		var name, value string
		switch kind.statType {
		case 6:
			// Reactions are counted per message: rank the messages
			// themselves with a link into the chat.
			name = fmt.Sprintf("<a href=\"%v\">сообщение</a>", chatMessageLink(context.EffectiveChat.Id, row.ContextID))
			value = fmt.Sprintf("%d %v", row.Total, ruPlural(row.Total, "лайк", "лайка", "лайков"))
		case salvationStatType:
			name = statsTopUserName(row.ContextID)
			value = fmt.Sprintf("%d %v", row.Total, ruPlural(row.Total, "спасение", "спасения", "спасений"))
		case 8:
			name = statsTopUserName(row.ContextID)
			value = fmt.Sprintf("%d %v", row.Total, ruPlural(row.Total, "смерть", "смерти", "смертей"))
		case 7:
			name = statsTopUserName(row.ContextID)
			value = formatDuration(row.Total)
		case 3:
			name = statsTopUserName(row.ContextID)
			value = fmt.Sprintf("%d %v", row.Total, ruPlural(row.Total, "сообщение", "сообщения", "сообщений"))
		case 4:
			name = row.Word
			value = fmt.Sprintf("%d %v", row.Total, ruPlural(row.Total, "раз", "раза", "раз"))
		}
		text += fmt.Sprintf("%v. %v - %v\n", i+1, name, value)
	}
	return Reply(text, *context)
}

// statsTopUserName resolves a user id for the top list, falling back to the
// bare id when the users table does not know them.
func statsTopUserName(userID int64) string {
	name := strconv.FormatInt(userID, 10)
	if user, err := GetUserFromDB(name); err == nil {
		name = UserFullName(&user)
	}
	return name
}
