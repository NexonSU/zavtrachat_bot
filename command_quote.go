package main

import (
	"bytes"
	"context"
	"fmt"
	"gorm.io/gorm"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type quoteContent struct {
	Text, Author   string
	DisplayText    string
	Date           int64
	UserID, ChatID int64
}

func quoteFromMessage(msg *gotgbot.Message) quoteContent {
	q := quoteContent{Text: msg.Text, Date: msg.Date}
	if q.Text == "" {
		q.Text = msg.Caption
	}
	if msg.SenderChat != nil {
		q.Author, q.ChatID = msg.SenderChat.Title, msg.SenderChat.Id
		if msg.AuthorSignature != "" {
			q.Author = msg.AuthorSignature + " · " + q.Author
		}
	} else if msg.From != nil {
		q.Author, q.UserID = UserFullName(msg.From), msg.From.Id
	}
	if msg.ForwardOrigin != nil {
		origin := msg.ForwardOrigin.MergeMessageOrigin()
		q.Date, q.UserID, q.ChatID = origin.Date, 0, 0
		switch {
		case origin.SenderUser != nil:
			q.Author, q.UserID = UserFullName(origin.SenderUser), origin.SenderUser.Id
		case origin.SenderUserName != "":
			q.Author = origin.SenderUserName
		case origin.Chat != nil:
			q.Author, q.ChatID = origin.Chat.Title, origin.Chat.Id
		case origin.SenderChat != nil:
			q.Author, q.ChatID = origin.SenderChat.Title, origin.SenderChat.Id
		default:
			q.Author = "Неизвестный автор"
		}
		if origin.AuthorSignature != "" && q.ChatID != 0 {
			q.Author = origin.AuthorSignature + " · " + q.Author
		}
	}
	if strings.TrimSpace(q.Author) == "" {
		q.Author = "Неизвестный автор"
	}
	return q
}

// Mark quote errors so the dispatcher never publishes a failed private response.
type quotePrivateError struct{ err error }

func (e *quotePrivateError) Error() string { return e.err.Error() }
func (e *quotePrivateError) Unwrap() error { return e.err }

func canUseQuote(bot *gotgbot.Bot, userID int64) bool {
	if Config == nil || userID <= 0 {
		return false
	}
	if IsAdmin(userID) || userID == Config.SysAdmin {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	member, err := bot.GetChatMemberWithContext(ctx, Config.Chat, userID, nil)
	return err == nil && member != nil && (member.GetStatus() == "administrator" || member.GetStatus() == "creator")
}

// Quote uses only the replied-to text/caption, author and original timestamp.
// Command arguments cannot replace somebody else's words or attribution.
func Quote(bot *gotgbot.Bot, ctx *ext.Context) error {
	return quoteWithLookup(bot, ctx, recoverQuoteSelection)
}

func quoteWithLookup(bot *gotgbot.Bot, ctx *ext.Context, lookup quoteLookup) (err error) {
	defer func() {
		if err != nil {
			err = &quotePrivateError{err}
		}
	}()
	if ctx == nil || ctx.Update == nil || ctx.Message == nil || ctx.EffectiveUser == nil || ctx.EffectiveChat == nil {
		return nil
	}
	admin := canUseQuote(bot, ctx.EffectiveUser.Id)
	if ctx.Message.ReplyToMessage == nil {
		return sendZBHelp(bot, ctx, "Отправь <code>/quote</code> в ответ на сообщение. Можно выделить часть текста через «Цитировать» в Telegram — в карточку попадёт только этот фрагмент.")
	}
	if !admin {
		restricted, checkErr := quoteAuthorRestricted(ctx.Message.ReplyToMessage)
		if checkErr != nil {
			return sendZBHelp(bot, ctx, "Не удалось проверить возможность цитирования. Попробуй позже.")
		}
		if restricted {
			return sendZBHelp(bot, ctx, "Эту цитату может создать только администратор.")
		}
	}
	q, validSelection, selectionStatus := quoteResolvedContent(ctx.Message, lookup)
	if args := ctx.Args(); len(args) == 2 && args[1] == "debug" {
		if !admin {
			return sendZBHelp(bot, ctx, "Диагностика доступна только администраторам.")
		}
		return sendZBHelp(bot, ctx, quoteSelectionDiagnostic(ctx.Message, q, validSelection, selectionStatus))
	}
	if !validSelection {
		return sendZBHelp(bot, ctx, "Не удалось сопоставить выделенный фрагмент с исходным сообщением. Выбери цитату заново.")
	}
	if strings.TrimSpace(q.Text) == "" {
		return sendZBHelp(bot, ctx, "В сообщении нет текста или подписи, которые можно процитировать.")
	}
	if quoteForbidden(q.Text) {
		return sendZBHelp(bot, ctx, quoteRejectedText)
	}
	if utf8.RuneCountInString(q.Text) > 4096 {
		return sendZBHelp(bot, ctx, "Цитата слишком длинная для одной карточки. Выбери сообщение покороче.")
	}
	q.DisplayText = quoteDisplayText(quoteFromMessage(ctx.Message.ReplyToMessage).Text, q.Text, ctx.Message.Quote)
	remaining, hour := 0, int64(0)
	if !admin {
		var budgetErr error
		remaining, hour, budgetErr = reserveQuote(time.Now())
		if budgetErr != nil {
			return budgetErr
		}
		if remaining < 0 {
			return sendZBHelp(bot, ctx, quoteBudgetText(remaining, hour))
		}
		sent := false
		defer func() {
			if !sent {
				DB.Model(&QuoteBudget{}).Where("hour = ? AND used > 0", hour).UpdateColumn("used", gorm.Expr("used - 1"))
			}
		}()
		// Mark success only after SendPhoto returns successfully.

		return sendBudgetQuote(bot, ctx, q, selectionStatus, remaining, hour, &sent)
	}
	return sendBudgetQuote(bot, ctx, q, selectionStatus, -1, 0, nil)
}

func sendBudgetQuote(bot *gotgbot.Bot, ctx *ext.Context, q quoteContent, selectionStatus string, remaining int, hour int64, sent *bool) (err error) {
	jobCtx, release, err := mediaTasks.acquire(context.Background())
	if err != nil {
		return sendZBHelp(bot, ctx, err.Error())
	}
	defer release()
	avatar, _ := quoteAvatarWithStatus(jobCtx, bot, q)
	card, err := renderQuote(q, avatar)
	if err != nil {
		return sendZBHelp(bot, ctx, fmt.Sprintf("Не удалось создать цитату: %s", err))
	}
	opts := &gotgbot.SendPhotoOpts{
		Caption:         "",
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.Message.ReplyToMessage.MessageId, AllowSendingWithoutReply: true},
	}
	switch ctx.EffectiveChat.Type {
	case "private":
	case "group", "supergroup":
	default:
		return nil
	}
	if remaining >= 0 {
		opts.Caption = strings.TrimSpace(opts.Caption + "\n" + quoteBudgetText(remaining, hour))
	}
	_, err = bot.SendPhoto(ctx.EffectiveChat.Id, gotgbot.InputFileByReader("quote.png", bytes.NewReader(card)), opts)
	if sent != nil && err == nil {
		*sent = true
	}
	return err
}
