package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gotd/td/tg"
)

type protoMediaChannelCache struct {
	mu      sync.Mutex
	channel *tg.Channel
}

// Resolve caches are shared by concurrent command handlers. Only resolution
// needs the lock; fetching messages and downloading media remain independent.
func resolveMediaChannel(api *tg.Client, base context.Context, username string, cache *protoMediaChannelCache) (*tg.Channel, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.channel != nil {
		return cache.channel, nil
	}
	var chats []tg.ChatClass
	err := protoRetry(base, 2, func(ctx context.Context) error {
		resolved, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: username})
		if err == nil {
			chats = resolved.GetChats()
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	if len(chats) == 0 {
		return nil, fmt.Errorf("resolve @%s: channel not found", username)
	}
	channel, ok := chats[0].(*tg.Channel)
	if !ok {
		return nil, fmt.Errorf("resolve @%s: not a channel", username)
	}
	cache.channel = channel
	return channel, nil
}

func protoDocumentFilename(doc *tg.Document) string {
	if attr, ok := doc.MapAttributes().AsDocumentAttributeFilename().First(); ok && attr.FileName != "" {
		return attr.FileName
	}
	return strings.ReplaceAll(doc.MimeType, "/", ".")
}
