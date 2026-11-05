package main

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm"
)

func Sweet(bot *gotgbot.Bot, ctx *ext.Context) (err error) {
	defer func() {
		if err != nil {
			err = &quotePrivateError{err}
		}
	}()
	if ctx == nil || ctx.Message == nil || ctx.EffectiveUser == nil || ctx.EffectiveChat == nil {
		return nil
	}
	switch ctx.EffectiveChat.Type {
	case "private", "group", "supergroup":
	default:
		return nil
	}
	reply := ctx.Message.ReplyToMessage
	if reply == nil {
		return sendZBHelp(bot, ctx, "Отправь <code>/sweet</code> в ответ на сообщение того, кого хочешь порадовать.")
	}
	var target quoteContent
	switch {
	case reply.SenderChat != nil:
		target.Author, target.ChatID = reply.SenderChat.Title, reply.SenderChat.Id
	case reply.From != nil:
		target.Author, target.UserID = UserFullName(reply.From), reply.From.Id
	}
	if strings.TrimSpace(target.Author) == "" {
		return sendZBHelp(bot, ctx, "Не удалось определить автора сообщения. Попробуй ответить на другое сообщение.")
	}

	sent := false
	if !canUseQuote(bot, ctx.EffectiveUser.Id) {
		remaining, hour, budgetErr := reserveCard(time.Now(), "sweet_budgets", 5)
		if budgetErr != nil {
			return budgetErr
		}
		if remaining < 0 {
			return sendZBHelp(bot, ctx, "Лимит открыток /sweet в час исчерпан. Новые открытки будут доступны в "+cardBudgetResetTime(hour)+".")
		}
		defer func() {
			if !sent {
				if refundErr := DB.Model(&SweetBudget{}).Where("hour = ? AND used > 0", hour).
					UpdateColumn("used", gorm.Expr("used - 1")).Error; refundErr != nil {
					Logger.Warn("sweet budget refund failed", "error", refundErr)
				}
			}
		}()
	}

	jobCtx, release, err := mediaTasks.acquire(context.Background())
	if err != nil {
		return sendZBHelp(bot, ctx, err.Error())
	}
	defer release()
	avatar, _ := quoteAvatarWithStatus(jobCtx, bot, target)
	card, err := renderSweet(target.Author, avatar)
	if err != nil {
		Logger.Warn("sweet render failed", "error", err)
		return sendZBHelp(bot, ctx, "Не удалось создать открытку. Попробуй позже.")
	}
	opts := &gotgbot.SendPhotoOpts{
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: reply.MessageId, AllowSendingWithoutReply: true},
	}
	if _, err = bot.SendPhoto(ctx.EffectiveChat.Id, gotgbot.InputFileByReader("sweet.png", bytes.NewReader(card)), opts); err != nil {
		return err
	}
	sent = true
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, cleanupErr := bot.DeleteMessageWithContext(cleanupCtx, ctx.EffectiveChat.Id, ctx.Message.MessageId, nil); cleanupErr != nil {
		Logger.Warn("sweet command cleanup failed")
	}
	return nil
}
