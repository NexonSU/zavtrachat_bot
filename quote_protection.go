package main

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"gorm.io/gorm/clause"
)

// Supplied by the private deployment secret, never committed to source.
var quoteProtectedDefaults string

type QuoteProtectedIdentity struct {
	UserID     int64 `gorm:"primaryKey;autoIncrement:false"`
	HandleHash string
}

func quoteHandleHash(handle string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.ToLower(strings.TrimPrefix(strings.TrimSpace(handle), "@")))))
}
func quoteAuthorRestricted(message *gotgbot.Message) (bool, error) {
	if Config == nil || len(Config.QuoteProtectedUsernames) == 0 {
		return false, nil
	}
	hashes := make([]string, 0, len(Config.QuoteProtectedUsernames))
	for _, name := range Config.QuoteProtectedUsernames {
		hashes = append(hashes, quoteHandleHash(name))
	}
	author := message.From
	if message.SenderChat != nil {
		author = nil
	}
	if message.ForwardOrigin != nil {
		origin := message.ForwardOrigin.MergeMessageOrigin()
		author = origin.SenderUser
		if author == nil && origin.Chat == nil && origin.SenderChat == nil {
			return true, nil
		}
	}
	if author == nil {
		return false, nil
	}
	var count int64
	if err := DB.Model(&QuoteProtectedIdentity{}).Where("user_id = ? AND handle_hash IN ?", author.Id, hashes).Count(&count).Error; err != nil {
		return true, err
	}
	if count > 0 {
		return true, nil
	}
	hash := quoteHandleHash(author.Username)
	for _, protected := range hashes {
		if hash == protected {
			row := QuoteProtectedIdentity{UserID: author.Id, HandleHash: hash}
			err := DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"handle_hash"})}).Create(&row).Error
			return true, err
		}
	}
	return false, nil
}
