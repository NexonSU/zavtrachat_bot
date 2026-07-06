package main

import (
	"log"
	"slices"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

var cache = &AlbumCache{
	timers: make(map[string]*time.Timer),
	groups: make(map[string][]int64),
}

// Forward channel post to chat
func ForwardChannelPost(bot *gotgbot.Bot, context *ext.Context) error {
	if context.EffectiveMessage == nil || context.EffectiveChat.Id != Config.Channel {
		return nil
	}

	if context.EffectiveMessage.MediaGroupId == "" {
		return forwardMessages(context.EffectiveChat.Id, Config.Chat, []int64{context.EffectiveMessage.MessageId}, context)
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	groupID := context.EffectiveMessage.MediaGroupId
	cache.groups[groupID] = append(cache.groups[groupID], context.EffectiveMessage.MessageId)

	if timer, exists := cache.timers[groupID]; exists {
		timer.Stop()
	}

	cache.timers[groupID] = time.AfterFunc(500*time.Millisecond, func() {
		cache.mu.Lock()
		msgIds := cache.groups[groupID]
		delete(cache.groups, groupID)
		delete(cache.timers, groupID)
		cache.mu.Unlock()
		err := forwardMessages(context.EffectiveChat.Id, Config.Chat, msgIds, context)
		if err != nil {
			log.Printf("Failed to forward album %s: %v", groupID, err)
		}
	})

	return nil
}

func forwardMessages(from int64, to int64, messageIds []int64, context *ext.Context) error {
	slices.Sort(messageIds)
	_, err := Bot.ForwardMessages(to, from, messageIds, nil)
	if Config.StreamChannel != 0 {
		if strings.Contains(context.EffectiveMessage.GetText(), "zavtracast/live") {
			_, err := Bot.ForwardMessages(Config.StreamChannel, from, messageIds, nil)
			return err
		}
		for _, entity := range append(context.EffectiveMessage.CaptionEntities, context.EffectiveMessage.Entities...) {
			if entity.Type == "url" || entity.Type == "text_link" {
				if strings.Contains(entity.Url, "zavtracast/live") {
					_, err := Bot.ForwardMessages(Config.StreamChannel, from, messageIds, nil)
					return err
				}
			}
		}
	}
	return err
}
