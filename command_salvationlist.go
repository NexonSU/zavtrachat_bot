package main

import (
	"crypto/rand"
	"fmt"
	"html"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const salvationListPrefix = "salvationlist:"
const salvationListPageSize = 5
const salvationListSessionLimit = 128
const salvationListSessionTTL = 30 * time.Minute

// Each keyboard is a one-use snapshot, bound to its owner's private message.
// Text is the existing primary key; page numbers must never identify a deletion.
type salvationListView struct {
	ownerID   int64
	messageID int64
	page      int
	texts     []string
	confirm   bool
	expires   time.Time
}

var salvationListViews = struct {
	sync.Mutex
	views map[string]salvationListView
}{views: make(map[string]salvationListView)}

func rememberSalvationListView(token string, view salvationListView) {
	salvationListViews.Lock()
	defer salvationListViews.Unlock()
	for key, old := range salvationListViews.views {
		if time.Now().After(old.expires) || (old.ownerID == view.ownerID && old.messageID == view.messageID) {
			delete(salvationListViews.views, key)
		}
	}
	if len(salvationListViews.views) >= salvationListSessionLimit {
		var oldest string
		for key, old := range salvationListViews.views {
			if oldest == "" || old.expires.Before(salvationListViews.views[oldest].expires) {
				oldest = key
			}
		}
		delete(salvationListViews.views, oldest)
	}
	view.expires = time.Now().Add(salvationListSessionTTL)
	salvationListViews.views[token] = view
}

func takeSalvationListView(token string, ownerID, messageID int64) (salvationListView, bool) {
	salvationListViews.Lock()
	defer salvationListViews.Unlock()
	view, ok := salvationListViews.views[token]
	if !ok || view.ownerID != ownerID || view.messageID != messageID {
		return salvationListView{}, false
	}
	delete(salvationListViews.views, token)
	return view, time.Now().Before(view.expires)
}

func SalvationList(bot *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveChat == nil || ctx.EffectiveUser == nil {
		return nil
	}
	if ctx.EffectiveChat.Type != "private" {
		_, err := bot.SendMessage(ctx.EffectiveChat.Id, "Команда работает только для администраторов и модераторов чата в личке с ботом.", nil)
		return err
	}
	if !IsAdminOrModer(ctx.EffectiveUser.Id) {
		_, err := bot.SendMessage(ctx.EffectiveChat.Id, "Управление фразами доступно только администраторам и модераторам чата.", nil)
		return err
	}
	return showSalvationListPage(bot, salvationListView{ownerID: ctx.EffectiveUser.Id}, 0)
}

func salvationListButton(token, text, action string) gotgbot.InlineKeyboardButton {
	return gotgbot.InlineKeyboardButton{Text: text, CallbackData: salvationListPrefix + token + ":" + action}
}

func salvationListPreview(text string) string {
	runes := []rune(text)
	if len(runes) > 200 {
		return string(runes[:200]) + "…"
	}
	return text
}

func showSalvationListPage(bot *gotgbot.Bot, view salvationListView, page int) error {
	var total int64
	if err := DB.Model(&Salvation{}).Count(&total).Error; err != nil {
		return err
	}
	pages := max(1, (int(total)+salvationListPageSize-1)/salvationListPageSize)
	page = min(max(0, page), pages-1)
	var phrases []Salvation
	if err := DB.Order("text").Offset(page * salvationListPageSize).Limit(salvationListPageSize).Find(&phrases).Error; err != nil {
		return err
	}
	view.page, view.texts, view.confirm = page, nil, false
	token := rand.Text()
	text := fmt.Sprintf("<b>Фразы спасения — %d</b>\nСтраница %d из %d.\nСообщения для спасения при блессинге.\n", total, page+1, pages)
	var buttons [][]gotgbot.InlineKeyboardButton
	for i, phrase := range phrases {
		view.texts = append(view.texts, phrase.Text)
		text += fmt.Sprintf("\n<b>%d.</b> %s\n", page*salvationListPageSize+i+1, html.EscapeString(salvationListPreview(phrase.Text)))
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{salvationListButton(token, fmt.Sprintf("Удалить №%d", page*salvationListPageSize+i+1), fmt.Sprintf("ask:%d", i))})
	}
	if len(phrases) == 0 {
		text += "\nФраз пока нет. Добавить: /addsalvation текст"
	}
	var navigation []gotgbot.InlineKeyboardButton
	if page > 0 {
		navigation = append(navigation, salvationListButton(token, "← Назад", fmt.Sprintf("page:%d", page-1)))
	}
	if page+1 < pages {
		navigation = append(navigation, salvationListButton(token, "Вперёд →", fmt.Sprintf("page:%d", page+1)))
	}
	if len(navigation) > 0 {
		buttons = append(buttons, navigation)
	}
	return showSalvationListView(bot, token, view, text, buttons)
}

func showSalvationListView(bot *gotgbot.Bot, token string, view salvationListView, text string, buttons [][]gotgbot.InlineKeyboardButton) error {
	if buttons == nil {
		buttons = [][]gotgbot.InlineKeyboardButton{}
	}
	markup := gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons}
	if view.messageID == 0 {
		sent, err := bot.SendMessage(view.ownerID, text, &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, ReplyMarkup: markup})
		if err != nil {
			return err
		}
		view.messageID = sent.MessageId
	} else {
		if _, _, err := bot.EditMessageText(&gotgbot.EditMessageTextOpts{ChatId: view.ownerID, MessageId: view.messageID, Text: text, ParseMode: gotgbot.ParseModeHTML, ReplyMarkup: markup}); err != nil {
			return err
		}
	}
	rememberSalvationListView(token, view)
	return nil
}

// Long legacy phrases go to a file so confirmations and recovery copies stay
// exact without exceeding Telegram's message limit. /addsalvation remains unchanged.
func salvationListQuote(bot *gotgbot.Bot, ownerID int64, text string) (string, error) {
	if len([]rune(text)) <= 1500 {
		return "<pre>" + html.EscapeString(text) + "</pre>", nil
	}
	_, err := bot.SendDocument(ownerID, gotgbot.InputFileByReader("salvation.txt", strings.NewReader(text)), &gotgbot.SendDocumentOpts{Caption: "Полный текст выбранной фразы."})
	return "Полный текст фразы — в файле salvation.txt.\n<pre>" + html.EscapeString(salvationListPreview(text)) + "</pre>", err
}

func SalvationListCallback(bot *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.CallbackQuery
	if query == nil {
		return nil
	}
	answer := func(text string) {
		query.Answer(bot, &gotgbot.AnswerCallbackQueryOpts{Text: text, ShowAlert: text != ""})
	}
	// Check both configured permissions and the actual private recipient on every
	// click, including confirmations. A forwarded keyboard cannot grant access.
	if !IsAdminOrModer(query.From.Id) || ctx.EffectiveChat == nil || ctx.EffectiveChat.Type != "private" || ctx.EffectiveChat.Id != query.From.Id || ctx.EffectiveMessage == nil {
		answer("Управление фразами доступно только администраторам и модераторам в личке с ботом.")
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(query.Data, salvationListPrefix), ":")
	if !strings.HasPrefix(query.Data, salvationListPrefix) || len(parts) < 2 || len(parts) > 3 {
		answer("Кнопка устарела. Откройте /salvationlist заново.")
		return nil
	}
	view, ok := takeSalvationListView(parts[0], query.From.Id, ctx.EffectiveMessage.MessageId)
	if !ok {
		answer("Кнопка устарела. Откройте /salvationlist заново.")
		return nil
	}
	answer("")
	var err error
	switch {
	case parts[1] == "page" && len(parts) == 3:
		page, parseErr := strconv.Atoi(parts[2])
		if parseErr == nil && page >= 0 {
			err = showSalvationListPage(bot, view, page)
		} else {
			err = fmt.Errorf("invalid salvationlist page")
		}
	case parts[1] == "ask" && len(parts) == 3 && !view.confirm:
		index, parseErr := strconv.Atoi(parts[2])
		if parseErr != nil || index < 0 || index >= len(view.texts) {
			err = fmt.Errorf("invalid salvationlist selection")
			break
		}
		view.texts, view.confirm = []string{view.texts[index]}, true
		var quote string
		quote, err = salvationListQuote(bot, view.ownerID, view.texts[0])
		if err == nil {
			token := rand.Text()
			note := "\n\nОна будет удалена из набора сообщений для спасения."
			if len([]rune(view.texts[0])) > 200 {
				note += "\nЭта старая фраза длиннее 200 символов: /addsalvation не сможет вернуть её целиком. Сохраните копию перед удалением."
			}
			err = showSalvationListView(bot, token, view, "<b>Удалить эту фразу из общего набора?</b>\n\n"+quote+note, [][]gotgbot.InlineKeyboardButton{{salvationListButton(token, "Да, удалить", "confirm"), salvationListButton(token, "Отмена", fmt.Sprintf("page:%d", view.page))}})
		}
	case parts[1] == "confirm" && len(parts) == 2 && view.confirm && len(view.texts) == 1:
		var status string
		status, err = deleteSalvationListText(view.texts[0])
		if err == nil {
			text := status
			if status == "Фраза удалена." {
				var quote string
				quote, err = salvationListQuote(bot, view.ownerID, view.texts[0])
				text += "\n\n" + quote + "\n\nСохраните текст. /addsalvation принимает фразы до 200 символов."
			}
			if err == nil {
				view.texts, view.confirm = nil, false
				token := rand.Text()
				err = showSalvationListView(bot, token, view, text, [][]gotgbot.InlineKeyboardButton{{salvationListButton(token, "К списку", fmt.Sprintf("page:%d", view.page))}})
			}
		}
	default:
		err = fmt.Errorf("invalid salvationlist action")
	}
	if err != nil {
		// Callback contexts have no ctx.Message: handle failures here instead of
		// passing them to the generic command error reply, which expects one.
		Logger.Error("salvationlist action failed", "error", err)
		bot.SendMessage(query.From.Id, "Не удалось выполнить действие. Откройте /salvationlist заново.", nil)
	}
	return nil
}

func deleteSalvationListText(text string) (string, error) {
	status := "Фраза уже удалена. Обновите список."
	err := DB.Transaction(func(tx *gorm.DB) error {
		// Lock the pool in a consistent order. In PostgreSQL a second deletion
		// waits, then sees the surviving rows; concurrent moderators cannot both
		// delete one of the last two phrases. No Telegram calls hold these locks.
		var phrases []Salvation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Order("text").Find(&phrases).Error; err != nil {
			return err
		}
		for _, phrase := range phrases {
			if phrase.Text != text {
				continue
			}
			if len(phrases) <= 1 {
				status = "Последнюю фразу удалить нельзя. Сначала добавьте другую через /addsalvation."
				return nil
			}
			result := tx.Where("text = ?", text).Delete(&Salvation{})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 {
				status = "Фраза удалена."
			}
			return nil
		}
		return nil
	})
	return status, err
}
