package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChatChannelOwner struct {
	ID       uint
	UserID   *int64 `gorm:"uniqueIndex"`
	Username string
	Name     string
	PinOrder int
	Channels []ChatChannel `gorm:"foreignKey:OwnerID"`
}

type ChatChannel struct {
	ID      uint
	OwnerID uint   `gorm:"not null;index"`
	Key     string `gorm:"not null;uniqueIndex"`
	URL     string `gorm:"not null"`
}

type ChatChannelImport struct {
	Version string `gorm:"primaryKey"`
}

type chatChannelSeedOwner struct {
	Name     string   `json:"name"`
	Username string   `json:"username"`
	Pinned   bool     `json:"pinned"`
	Links    []string `json:"links"`
}

// The deploy workflow installs the private catalog alongside the bot binary.
func chatChannelSeedPath(configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	path := filepath.Join(filepath.Dir(executable), "chat-channels-seed.json")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else {
		return path, err
	}
}

var (
	channelPublicName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,31}$`)
	channelInviteCode = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

// Public usernames are case-insensitive. Invite codes must keep their case.
func parseChatChannel(link string) (ChatChannel, bool) {
	if !strings.Contains(link, "://") {
		link = "https://" + link
	}
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") ||
		!strings.EqualFold(u.Host, "t.me") || u.User != nil || u.RawQuery != "" ||
		u.ForceQuery || u.Fragment != "" || u.RawPath != "" || strings.Contains(link, "#") {
		return ChatChannel{}, false
	}
	path := strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/")
	if code, invite := strings.CutPrefix(path, "+"); invite || strings.HasPrefix(path, "joinchat/") {
		if !invite {
			code = strings.TrimPrefix(path, "joinchat/")
		}
		if channelInviteCode.MatchString(code) {
			return ChatChannel{Key: "+" + code, URL: "https://t.me/+" + code}, true
		}
		return ChatChannel{}, false
	}
	key := strings.ToLower(path)
	switch key {
	case "s", "c", "share", "proxy", "socks", "login", "joinchat", "addstickers", "addemoji":
		return ChatChannel{}, false
	}
	if !channelPublicName.MatchString(path) {
		return ChatChannel{}, false
	}
	return ChatChannel{Key: key, URL: "https://t.me/" + path}, true
}

func importChatChannels(db *gorm.DB, filename string) error {
	if filename == "" {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		marker := ChatChannelImport{Version: "initial"}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&marker)
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		data, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		var seeds []chatChannelSeedOwner
		if json.Unmarshal(data, &seeds) != nil || len(seeds) == 0 {
			return errors.New("invalid chat channels seed file")
		}
		seen := make(map[string]bool, len(seeds))
		for i, seed := range seeds {
			username := strings.ToLower(seed.Username)
			if strings.TrimSpace(seed.Name) == "" || len(seed.Links) == 0 || seen[username] ||
				(username != "" && !channelPublicName.MatchString(username)) {
				return fmt.Errorf("invalid chat channel owner in seed entry %d", i+1)
			}
			seen[username] = true
			owner := ChatChannelOwner{Username: seed.Username, Name: seed.Name}
			if seed.Pinned {
				owner.PinOrder = i + 1
			}
			if username != "" {
				var users []gotgbot.User
				if err := tx.Select("id").Where("lower(username) = ?", username).Limit(2).Find(&users).Error; err != nil {
					return err
				}
				if len(users) == 1 {
					owner.UserID = &users[0].Id
				}
			}
			// The registry may already have been used before adding the seed file.
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "user_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"username", "name", "pin_order"}),
			}).Create(&owner).Error; err != nil {
				return err
			}
			var channels []ChatChannel
			for _, link := range seed.Links {
				channel, ok := parseChatChannel(link)
				if !ok {
					return fmt.Errorf("invalid chat channel link in seed entry %d", i+1)
				}
				channel.OwnerID = owner.ID
				channels = append(channels, channel)
			}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoNothing: true}).
				Create(&channels).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Imported owners with no known ID can be matched by their current Telegram
// username. Once bound, a username change or reassignment cannot grant access.
func lockChatChannelOwner(tx *gorm.DB, user *gotgbot.User) (ChatChannelOwner, error) {
	var owner ChatChannelOwner
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? OR (user_id IS NULL AND username <> '' AND lower(username) = ?)",
			user.Id, strings.ToLower(user.Username)).
		Order("user_id NULLS LAST").Take(&owner).Error
	return owner, err
}

func addOwnChatChannel(db *gorm.DB, user *gotgbot.User, channel ChatChannel) (bool, error) {
	added := false
	err := db.Transaction(func(tx *gorm.DB) error {
		owner, err := lockChatChannelOwner(tx, user)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			owner = ChatChannelOwner{UserID: &user.Id, Username: user.Username, Name: UserFullName(user)}
			result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoNothing: true}).Create(&owner)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				owner, err = lockChatChannelOwner(tx, user)
			} else {
				err = nil
			}
		}
		if err != nil {
			return err
		}
		channel.OwnerID = owner.ID
		result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoNothing: true}).Create(&channel)
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		added = true
		return bindChatChannelOwner(tx, owner, user.Id)
	})
	return added, err
}

func bindChatChannelOwner(tx *gorm.DB, owner ChatChannelOwner, userID int64) error {
	if owner.UserID != nil {
		return nil
	}
	return tx.Model(&owner).Update("user_id", userID).Error
}

// With no link, remove the sole channel or return the choices for that owner.
func deleteOwnChatChannel(db *gorm.DB, user *gotgbot.User, key string) (bool, []string, error) {
	var deleted bool
	var choices []string
	err := db.Transaction(func(tx *gorm.DB) error {
		owner, err := lockChatChannelOwner(tx, user)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if key == "" {
			var channels []ChatChannel
			if err := tx.Where("owner_id = ?", owner.ID).Order("id").Find(&channels).Error; err != nil {
				return err
			}
			switch len(channels) {
			case 0:
				return nil
			case 1:
				key = channels[0].Key
			default:
				for _, channel := range channels {
					choices = append(choices, channel.URL)
				}
				return nil
			}
		}
		result := tx.Where("owner_id = ? AND key = ?", owner.ID, key).Delete(&ChatChannel{})
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		deleted = true
		return bindChatChannelOwner(tx, owner, user.Id)
	})
	return deleted, choices, err
}

func chatChannelList(db *gorm.DB) (string, error) {
	var owners []ChatChannelOwner
	if err := db.Preload("Channels", func(q *gorm.DB) *gorm.DB { return q.Order("id") }).Find(&owners).Error; err != nil {
		return "", err
	}
	names := collate.New(language.Russian, collate.IgnoreCase)
	sort.Slice(owners, func(i, j int) bool {
		a, b := owners[i], owners[j]
		if a.PinOrder != b.PinOrder {
			if a.PinOrder == 0 || b.PinOrder == 0 {
				return a.PinOrder != 0
			}
			return a.PinOrder < b.PinOrder
		}
		if cmp := names.CompareString(a.Name, b.Name); cmp != 0 {
			return cmp < 0
		}
		if aName, bName := strings.ToLower(a.Username), strings.ToLower(b.Username); aName != bName {
			return aName < bName
		}
		return a.ID < b.ID
	})
	blocks := make([]string, 0, len(owners))
	for _, owner := range owners {
		if len(owner.Channels) == 0 {
			continue
		}
		title := owner.Name
		if owner.Username != "" {
			title += " (" + owner.Username + ")"
		}
		lines := []string{title + ":"}
		for _, channel := range owner.Channels {
			lines = append(lines, channel.URL)
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	if len(blocks) == 0 {
		return "В каталоге пока нет каналов.", nil
	}
	return strings.Join(blocks, "\n\n"), nil
}
