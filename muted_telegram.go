package main

import (
	"context"
	"fmt"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/gotd/td/tg"
)

type mutedMember struct {
	user       gotgbot.User
	until      int64
	providerID int64
	provider   string
	date       int64
}

type mutedSnapshot struct {
	members    []mutedMember
	partial    bool
	unverified int
}

type mutedParticipantsAPI interface {
	ChannelsGetParticipants(context.Context, *tg.ChannelsGetParticipantsRequest) (tg.ChannelsChannelParticipantsClass, error)
}

func telegramMutedMembers(api mutedParticipantsAPI, ctx context.Context, chatID int64, now int64) ([]mutedMember, error) {
	if chatID >= -1000000000000 {
		return nil, fmt.Errorf("mute listing requires a supergroup")
	}
	// Zero access hashes are explicitly supported for bots which know the ID:
	// https://core.telegram.org/api/peers#access-hash
	channel := &tg.InputChannel{ChannelID: -chatID - 1000000000000}
	users := map[int64]gotgbot.User{}
	seen := map[int64]bool{}
	var members []mutedMember
	for offset := 0; ; {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result, err := api.ChannelsGetParticipants(ctx, &tg.ChannelsGetParticipantsRequest{
			Channel: channel, Filter: &tg.ChannelParticipantsBanned{}, Offset: offset, Limit: 100,
		})
		if err != nil {
			return nil, err
		}
		page, ok := result.(*tg.ChannelsChannelParticipants)
		if !ok {
			return nil, fmt.Errorf("unexpected muted participants response")
		}
		for _, value := range page.Users {
			if u, ok := value.(*tg.User); ok {
				users[u.ID] = gotgbot.User{Id: u.ID, IsBot: u.Bot, FirstName: u.FirstName, LastName: u.LastName, Username: u.Username}
			}
		}
		added := 0
		for _, value := range page.Participants {
			p, ok := value.(*tg.ChannelParticipantBanned)
			if !ok {
				continue
			}
			peer, ok := p.Peer.(*tg.PeerUser)
			if !ok || seen[peer.UserID] {
				continue
			}
			seen[peer.UserID], added = true, added+1
			rights := p.BannedRights
			if p.Left || rights.ViewMessages || (!rights.SendMessages && !rights.SendPlain) {
				continue
			}
			until := int64(rights.UntilDate)
			if until >= 2147483647 {
				until = 0
			}
			if until != 0 && until <= now {
				continue
			}
			members = append(members, mutedMember{user: gotgbot.User{Id: peer.UserID}, until: until, providerID: p.KickedBy, date: int64(p.Date)})
		}
		offset += len(page.Participants)
		if offset >= page.Count {
			break
		}
		if len(page.Participants) == 0 || added == 0 {
			return nil, fmt.Errorf("muted participants pagination made no progress")
		}
	}
	for i := range members {
		if user, ok := users[members[i].user.Id]; ok {
			members[i].user = user
		}
		if actor, ok := users[members[i].providerID]; ok {
			members[i].provider = UserFullName(&actor)
		}
	}
	return members, nil
}

func loadMutedSnapshot(bot *gotgbot.Bot, chatID int64, events []MuteEvent) mutedSnapshot {
	if api, base, ready := mtprotoAPI(); ready {
		ctx, cancel := context.WithTimeout(base, 12*time.Second)
		members, err := telegramMutedMembers(api, ctx, chatID, time.Now().Unix())
		cancel()
		if err == nil {
			return mutedSnapshot{members: members}
		}
	}
	// Bot API cannot enumerate all restricted members. During an MTProto
	// outage, verify known candidates and explicitly label the result partial.
	return knownMutedMembers(bot, chatID, events)
}

func knownMutedMembers(bot *gotgbot.Bot, chatID int64, events []MuteEvent) mutedSnapshot {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	result := mutedSnapshot{partial: true}
	now := time.Now().Unix()
	checked := 0
	for _, event := range events {
		if !event.Muted || (event.UntilDate != 0 && event.UntilDate <= now) {
			continue
		}
		// Bound a fallback scan as well as its wall time. Never imply that an
		// unqueried member is unmuted.
		if checked >= 500 || ctx.Err() != nil {
			result.unverified++
			continue
		}
		checked++
		member, err := bot.GetChatMemberWithContext(ctx, chatID, event.UserID, nil)
		if err != nil {
			result.unverified++
			continue
		}
		if !chatMemberIsMuted(member) {
			continue
		}
		m := member.MergeChatMember()
		if m.UntilDate != 0 && m.UntilDate <= now {
			continue
		}
		result.members = append(result.members, mutedMember{user: member.GetUser(), until: m.UntilDate})
	}
	return result
}
