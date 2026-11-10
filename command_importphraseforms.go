package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm"
)

const maxPhraseImportBytes = 256 * 1024

type phraseFormImport struct {
	Blessings  []PhraseVariants `json:"blessings"`
	Salvations []PhraseVariants `json:"salvations"`
}

type phraseImportResult struct {
	updated int64
	skipped int64
	missing int64
}

func ImportPhraseForms(bot *gotgbot.Bot, ctx *ext.Context) error {
	chat, user, msg := ctx.EffectiveChat, ctx.EffectiveUser, ctx.EffectiveMessage
	if chat == nil || user == nil || msg == nil || chat.Type != "private" || chat.Id != user.Id ||
		(user.Id != Config.SysAdmin && !IsAdmin(user.Id)) {
		return nil
	}
	reply := func(text string) error {
		_, err := bot.SendMessage(chat.Id, text, nil)
		return err
	}
	target := msg.ReplyToMessage
	if target == nil || target.Document == nil || target.Document.FileSize <= 0 ||
		target.Document.FileSize > maxPhraseImportBytes {
		return reply("Ответь /importphraseforms на JSON-файл миграции размером до 256 КБ.")
	}
	deadline, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	data, err := readPhraseImport(deadline, bot, target)
	if err != nil {
		return reply("Не удалось прочитать файл миграции.")
	}
	result, err := applyPhraseImport(DB.WithContext(deadline), data)
	if err != nil {
		return reply("Импорт не выполнен. Проверь формат файла и состояние базы.")
	}
	return reply(fmt.Sprintf("Миграция завершена. Обновлено: %d. Уже заполнено: %d. Не найдено: %d.",
		result.updated, result.skipped, result.missing))
}

func readPhraseImport(ctx context.Context, bot *gotgbot.Bot, msg *gotgbot.Message) ([]byte, error) {
	media, err := GetMedia(msg)
	if err != nil {
		return nil, err
	}
	path, cleanup, err := mediaInputFile(ctx, bot, media)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, maxPhraseImportBytes+1))
}

func applyPhraseImport(db *gorm.DB, data []byte) (phraseImportResult, error) {
	var payload phraseFormImport
	if len(data) > maxPhraseImportBytes || !utf8.Valid(data) || json.Unmarshal(data, &payload) != nil ||
		len(payload.Blessings)+len(payload.Salvations) == 0 {
		return phraseImportResult{}, errors.New("invalid phrase import")
	}
	pools := []struct {
		model any
		rows  []PhraseVariants
	}{{&Bless{}, payload.Blessings}, {&Salvation{}, payload.Salvations}}
	for _, pool := range pools {
		seen := make(map[string]bool, len(pool.rows))
		for _, row := range pool.rows {
			if strings.TrimSpace(row.Male) == "" || strings.TrimSpace(row.Female) == "" || seen[row.Male] ||
				strings.ContainsRune(row.Male, 0) || strings.ContainsRune(row.Female, 0) {
				return phraseImportResult{}, errors.New("empty, invalid or duplicate phrase")
			}
			seen[row.Male] = true
		}
	}

	var result phraseImportResult
	err := db.Transaction(func(tx *gorm.DB) error {
		for _, pool := range pools {
			for _, row := range pool.rows {
				update := tx.Model(pool.model).Where("text = ? AND male_text = '' AND female_text = ''", row.Male).
					Updates(map[string]any{"male_text": row.Male, "female_text": row.Female})
				if update.Error != nil {
					return update.Error
				}
				if update.RowsAffected == 1 {
					result.updated++
					continue
				}
				var count int64
				if err := tx.Model(pool.model).Where("text = ?", row.Male).Count(&count).Error; err != nil {
					return err
				}
				if count == 0 {
					result.missing++
				} else {
					result.skipped++
				}
			}
		}
		return nil
	})
	return result, err
}
