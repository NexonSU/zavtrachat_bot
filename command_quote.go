package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type quoteContent struct {
	Text, Author   string
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

// Quote uses only the replied-to text/caption, author and original timestamp.
// Command arguments cannot replace somebody else's words or attribution.
func Quote(bot *gotgbot.Bot, ctx *ext.Context) error {
	if ctx == nil || ctx.Update == nil || ctx.Message == nil {
		return nil
	}
	if ctx.Message.ReplyToMessage == nil {
		return sendZBHelp(bot, ctx, "Отправь <code>/quote</code> в ответ на сообщение — сделаю карточку с цитатой, аватаркой, именем и датой.")
	}
	q := quoteFromMessage(ctx.Message.ReplyToMessage)
	if strings.TrimSpace(q.Text) == "" {
		return sendZBHelp(bot, ctx, "В сообщении нет текста или подписи, которые можно процитировать.")
	}
	if utf8.RuneCountInString(q.Text) > 4096 {
		return sendZBHelp(bot, ctx, "Цитата слишком длинная для одной карточки. Выбери сообщение покороче.")
	}
	jobCtx, release, err := mediaTasks.acquire(context.Background())
	if err != nil {
		return sendZBHelp(bot, ctx, err.Error())
	}
	defer release()
	avatarCtx, cancel := context.WithTimeout(jobCtx, 8*time.Second)
	avatar := quoteAvatar(avatarCtx, bot, q)
	cancel()
	card, err := renderQuote(q, avatar)
	if err != nil {
		return sendZBHelp(bot, ctx, fmt.Sprintf("Не удалось создать цитату: %s", err))
	}
	_, err = bot.SendPhoto(ctx.EffectiveChat.Id, gotgbot.InputFileByReader("quote.png", bytes.NewReader(card)), &gotgbot.SendPhotoOpts{
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.Message.ReplyToMessage.MessageId, AllowSendingWithoutReply: true},
	})
	return err
}
