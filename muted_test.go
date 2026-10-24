package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/gotd/td/tg"
)

func TestMutedAccessAndPrivateDelivery(t *testing.T) {
	for _, tc := range []struct {
		name   string
		userID int64
		status string
		allow  bool
	}{
		{"configured admin", 100, "member", true},
		{"configured moderator", 300, "member", true},
		{"Telegram admin", 400, "administrator", true},
		{"owner", 401, "creator", true},
		{"member", 42, "member", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bot, client := setupTestEnv(t)
			client.setMemberStatus(tc.userID, tc.status)
			if err := Muted(bot, testContext(bot, "/muted", testUser(tc.userID))); err != nil {
				t.Fatal(err)
			}
			calls := client.callsFor("sendMessage")
			if len(calls) != 1 {
				t.Fatalf("responses = %d", len(calls))
			}
			eph, ok := calls[0].Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
			if !ok || eph.ReceiverUserId != tc.userID || calls[0].Params["chat_id"] != Config.Chat {
				t.Fatal("response was not private to the requester in the main chat")
			}
			text := calls[0].Params["text"].(string)
			if strings.Contains(text, "Сейчас замьючены") != tc.allow {
				t.Fatalf("unexpected access result: %q", text)
			}
			if tc.allow && (!strings.Contains(text, "Полный список Telegram недоступен") || strings.Contains(text, "Никто не замьючен")) {
				t.Fatal("an unavailable full list must not look like an empty complete list")
			}
		})
	}
	bot, client := setupTestEnv(t)
	client.setError("sendMessage", errors.New("delivery unavailable"))
	if err := Muted(bot, testContext(bot, "/muted", testUser(100))); err == nil || len(client.callsFor("sendMessage")) != 1 {
		t.Fatal("failed ephemeral delivery must not retry publicly")
	}
}

func TestMutedWrongChatAndRevokedRights(t *testing.T) {
	bot, client := setupTestEnv(t)
	ctx := testContext(bot, "/muted", testUser(100))
	ctx.EffectiveChat.Id = -999
	if err := Muted(bot, ctx); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(client.sentTexts(), ""), "Сейчас замьючены") {
		t.Fatal("report leaked outside the main chat")
	}
	client.clearCalls()
	client.setError("getChatMember", errors.New("cannot verify admin"))
	if err := Muted(bot, testContext(bot, "/muted", testUser(42))); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(client.sentTexts(), ""), "Сейчас замьючены") {
		t.Fatal("admin check must fail closed")
	}
}

func TestTrackedRestrictionsAndLifts(t *testing.T) {
	bot, client := setupTestEnv(t)
	seedUser(42, "victim", "Жертва")
	if err := Mute(bot, testContext(bot, "/mute @victim 3600", testUser(100))); err != nil {
		t.Fatal(err)
	}
	events, err := latestMuteEvents(Config.Chat)
	if err != nil || len(events) != 1 || events[0].Source != "mute" || events[0].ActorID != 100 || !events[0].Muted || events[0].UserID != 42 {
		t.Fatalf("mute event: %+v, %v", events, err)
	}
	until := events[0].UntilDate
	client.setResult("getChatMember", json.RawMessage(fmt.Sprintf(`{"status":"restricted","is_member":true,"can_send_messages":false,"until_date":%d,"user":{"id":42,"first_name":"Жертва"}}`, until)))
	if got := knownMutedMembers(bot, Config.Chat, events); len(got.members) != 1 || got.members[0].until != until {
		t.Fatalf("current mute not verified: %+v", got)
	}
	if err := Unmute(bot, testContext(bot, "/unmute @victim", testUser(100))); err != nil {
		t.Fatal(err)
	}
	events, err = latestMuteEvents(Config.Chat)
	if err != nil || len(events) != 1 || events[0].Muted {
		t.Fatalf("lift: %+v, %v", events, err)
	}
	// A later mute with a failed Telegram request must not replace the lift.
	client.setError("restrictChatMember", errors.New("forbidden"))
	if err := Mute(bot, testContext(bot, "/mute @victim", testUser(100))); err == nil {
		t.Fatal("expected API failure")
	}
	events, _ = latestMuteEvents(Config.Chat)
	if events[0].Muted {
		t.Fatal("failed restriction recorded as successful")
	}
	if snapshot := knownMutedMembers(bot, Config.Chat, events); len(snapshot.members) != 0 {
		t.Fatal("a successful lift remained a fallback candidate")
	}
}

func TestTrackedBlessAndSalvation(t *testing.T) {
	bot, _ := setupTestEnv(t)
	if err := blessingWithSalvationRoll(bot, testContext(bot, "/blessing", testUser(42)), func() int { return 99 }); err != nil {
		t.Fatal(err)
	}
	events, _ := latestMuteEvents(Config.Chat)
	if len(events) != 1 || events[0].Source != "blessing" || events[0].ActorID != 42 || events[0].UserID != 42 {
		t.Fatalf("self blessing: %+v", events)
	}
	DB.Create(&Salvation{Text: "Повезло"})
	if err := blessingWithSalvationRoll(bot, testContext(bot, "/blessing", testUser(43)), func() int { return 0 }); err != nil {
		t.Fatal(err)
	}
	events, _ = latestMuteEvents(Config.Chat)
	if len(events) != 1 {
		t.Fatal("salvation was recorded as a mute")
	}
}

func TestManualMuteUpdatesAndDeduplication(t *testing.T) {
	bot, _ := setupTestEnv(t)
	change := &gotgbot.ChatMemberUpdated{Chat: gotgbot.Chat{Id: Config.Chat}, From: *testUser(777), Date: time.Now().Unix(),
		OldChatMember: gotgbot.ChatMemberMember{User: *testUser(42)},
		NewChatMember: gotgbot.ChatMemberRestricted{User: *testUser(42), IsMember: true, UntilDate: time.Now().Unix() + 600}}
	ctx := ext.NewContext(bot, &gotgbot.Update{UpdateId: 9001, ChatMember: change}, nil)
	for range 2 {
		if err := OnMuteChatMember(bot, ctx); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	DB.Model(&MuteEvent{}).Count(&count)
	events, _ := latestMuteEvents(Config.Chat)
	if count != 1 || len(events) != 1 || events[0].ActorID != 777 || events[0].Source != "manual" {
		t.Fatalf("manual event/deduplication: %+v, count %d", events, count)
	}
	change.OldChatMember = change.NewChatMember
	change.NewChatMember = gotgbot.ChatMemberMember{User: *testUser(42)}
	ctx.UpdateId++
	OnMuteChatMember(bot, ctx)
	events, _ = latestMuteEvents(Config.Chat)
	if events[0].Muted {
		t.Fatal("manual unmute not recorded")
	}
	// Bot-authored updates must preserve command/game attribution.
	change.From = bot.User
	change.NewChatMember = gotgbot.ChatMemberRestricted{User: *testUser(42), IsMember: true}
	ctx.UpdateId++
	OnMuteChatMember(bot, ctx)
	events, _ = latestMuteEvents(Config.Chat)
	if events[0].Muted {
		t.Fatal("generic own-bot event overwrote command attribution")
	}
}

func TestKnownMutedExcludesExpiredLeftAndMediaOnly(t *testing.T) {
	for _, result := range []string{
		`{"status":"member","user":{"id":42,"first_name":"A"}}`,
		`{"status":"restricted","is_member":false,"can_send_messages":false,"user":{"id":42,"first_name":"A"}}`,
		`{"status":"restricted","is_member":true,"can_send_messages":true,"user":{"id":42,"first_name":"A"}}`,
		`{"status":"restricted","is_member":true,"can_send_messages":false,"until_date":1,"user":{"id":42,"first_name":"A"}}`,
	} {
		bot, client := setupTestEnv(t)
		client.setResult("getChatMember", json.RawMessage(result))
		if snapshot := knownMutedMembers(bot, Config.Chat, []MuteEvent{{UserID: 42, Muted: true}}); len(snapshot.members) != 0 {
			t.Fatalf("included an inactive mute: %s", result)
		}
	}
	bot, client := setupTestEnv(t)
	client.setError("getChatMember", errors.New("timeout"))
	if snapshot := knownMutedMembers(bot, Config.Chat, []MuteEvent{{UserID: 42, Muted: true}}); snapshot.unverified != 1 || !snapshot.partial {
		t.Fatal("failed verification hidden")
	}
}

func TestMutedOriginsAndSafeRendering(t *testing.T) {
	now := time.Unix(1800000000, 0)
	event := MuteEvent{UserID: 42, ActorID: 42, ActorName: "<Админ>&", ProviderID: 1, Muted: true, UntilDate: now.Unix() + 600, Source: "blessing", StartedAt: now.Unix(), CompletedAt: now.Unix()}
	member := mutedMember{user: gotgbot.User{Id: 42, FirstName: "<Кот>&"}, until: event.UntilDate, providerID: 1, date: now.Unix()}
	if got := mutedOrigin(member, event, 1); got != "Сам блессанулся." {
		t.Fatal(got)
	}
	member.providerID, member.provider = 777, "<Маша>&"
	if got := mutedOrigin(member, event, 1); !strings.Contains(got, "&lt;Маша&gt;&amp;") || strings.Contains(got, "блесс") {
		t.Fatalf("live admin did not override old game: %s", got)
	}
	member.providerID = 1
	member.date += 100
	if got := mutedOrigin(member, event, 1); got != "Причина неизвестна." {
		t.Fatal("stale event reused for a new restriction")
	}
	member.date = now.Unix()
	event.Source, event.ActorID = "mute", 100
	text, buttons := renderMuted(mutedSnapshot{members: []mutedMember{member}}, []MuteEvent{event}, 1, 100, 1000, now)
	for _, expected := range []string{"&lt;Кот&gt;&amp;", "&lt;Админ&gt;&amp;", "10 минут", "МСК", "Страница 1/1"} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing %q in %q", expected, text)
		}
	}
	if buttons.InlineKeyboard[0][0].CallbackData != "muted:100:0" {
		t.Fatal("refresh not bound to the requester")
	}
	var members []mutedMember
	for i := range 12 {
		members = append(members, mutedMember{user: gotgbot.User{Id: int64(i + 1), FirstName: strings.Repeat("😼<&", 100)}, until: now.Unix() + int64(i+1)*60})
	}
	text, buttons = renderMuted(mutedSnapshot{members: members}, nil, 1, 100, 1, now)
	if len(utf16.Encode([]rune(text))) > 4096 || !strings.Contains(text, "Страница 2/3") || len(buttons.InlineKeyboard[0]) != 3 {
		t.Fatal("oversized or broken paginated view")
	}
}

func TestMutedCallbackOwnershipAndRefresh(t *testing.T) {
	bot, client := setupTestEnv(t)
	client.setResult("editEphemeralMessageText", json.RawMessage(`true`))
	msg := testMessage("report", &bot.User)
	msg.MessageId, msg.EphemeralMessageId, msg.ReceiverUser = 0, 888, testUser(100)
	ctx := testCallbackContext(bot, msg, testUser(100))
	ctx.CallbackQuery.Data = "muted:100:0"
	if err := MutedCallback(bot, ctx); err != nil {
		t.Fatal(err)
	}
	edits := client.callsFor("editEphemeralMessageText")
	if len(edits) != 1 || edits[0].Params["receiver_user_id"] != int64(100) || edits[0].Params["ephemeral_message_id"] != int64(888) || len(client.callsFor("editMessageText")) != 0 {
		t.Fatal("refresh did not edit the owner's ephemeral message")
	}
	for _, data := range []string{"muted:42:0", "muted:100:-1", "muted:100:999999999999999999999999", "muted:garbage"} {
		client.clearCalls()
		ctx.CallbackQuery.Data = data
		if err := MutedCallback(bot, ctx); err != nil {
			t.Fatal(err)
		}
		if len(client.callsFor("editEphemeralMessageText")) != 0 || len(client.callsFor("sendMessage")) != 0 {
			t.Fatalf("invalid callback leaked a view: %s", data)
		}
	}
}

type fakeMutedAPI struct {
	pages    []*tg.ChannelsChannelParticipants
	requests []*tg.ChannelsGetParticipantsRequest
	err      error
}

func (f *fakeMutedAPI) ChannelsGetParticipants(_ context.Context, req *tg.ChannelsGetParticipantsRequest) (tg.ChannelsChannelParticipantsClass, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return nil, f.err
	}
	if len(f.requests) > len(f.pages) {
		return nil, errors.New("unexpected extra page")
	}
	return f.pages[len(f.requests)-1], nil
}

func TestTelegramMutedPaginationAndFilters(t *testing.T) {
	now := int64(1800000000)
	participant := func(id int64, rights tg.ChatBannedRights, left bool) tg.ChannelParticipantClass {
		return &tg.ChannelParticipantBanned{Peer: &tg.PeerUser{UserID: id}, KickedBy: 777, Date: int(now), BannedRights: rights, Left: left}
	}
	api := &fakeMutedAPI{pages: []*tg.ChannelsChannelParticipants{
		{Count: 6, Participants: []tg.ChannelParticipantClass{
			participant(42, tg.ChatBannedRights{SendMessages: true, UntilDate: int(now + 600)}, false),
			participant(43, tg.ChatBannedRights{SendMedia: true}, false),
			participant(44, tg.ChatBannedRights{SendMessages: true, ViewMessages: true}, false),
		}},
		{Count: 6, Participants: []tg.ChannelParticipantClass{
			participant(45, tg.ChatBannedRights{SendMessages: true}, true),
			participant(46, tg.ChatBannedRights{SendMessages: true, UntilDate: int(now - 1)}, false),
			participant(47, tg.ChatBannedRights{SendPlain: true, UntilDate: 2147483647}, false),
		}, Users: []tg.UserClass{&tg.User{ID: 42, FirstName: "Кот"}, &tg.User{ID: 777, FirstName: "Админ"}}},
	}}
	members, err := telegramMutedMembers(api, context.Background(), -1001234567890, now)
	if err != nil || len(members) != 2 || members[0].user.FirstName != "Кот" || members[0].provider != "Админ" || members[1].until != 0 {
		t.Fatalf("members: %+v, %v", members, err)
	}
	if api.requests[0].Channel.(*tg.InputChannel).ChannelID != 1234567890 || api.requests[1].Offset != 3 {
		t.Fatal("incorrect channel conversion/pagination")
	}
	broken := &fakeMutedAPI{pages: []*tg.ChannelsChannelParticipants{{Count: 3}}}
	if _, err := telegramMutedMembers(broken, context.Background(), -1001234567890, now); err == nil {
		t.Fatal("an incomplete empty page was reported as complete")
	}
	broken.err = errors.New("MTProto offline")
	if _, err := telegramMutedMembers(broken, context.Background(), -1001234567890, now); err == nil {
		t.Fatal("connection failure hidden")
	}
}

func TestMuteUntilNormalization(t *testing.T) {
	now := int64(1800000000)
	for _, delta := range []int64{0, 10, 29, 366*86400 + 1} {
		if got := normalizedMuteUntil(now+delta, now); got != 0 {
			t.Errorf("%d: %d", delta, got)
		}
	}
	for _, delta := range []int64{30, 3600, 366 * 86400} {
		if got := normalizedMuteUntil(now+delta, now); got != now+delta {
			t.Errorf("%d: %d", delta, got)
		}
	}
}

func TestMutedMenuScope(t *testing.T) {
	group, private, admin := commandMenus(commandListConfig())
	for _, menu := range [][]gotgbot.BotCommand{group, private} {
		for _, command := range menu {
			if command.Command == "muted" {
				t.Fatal("/muted advertised outside the administrator menu")
			}
		}
	}
	found := false
	for _, command := range admin {
		if command.Command == "muted" {
			found = command.IsEphemeral && command.Description == "Кто сейчас замьючен"
		}
	}
	if !found || len(admin) != len(group)+1 || len(admin) > 100 {
		t.Fatal("incomplete or invalid administrator menu")
	}
}

func TestMuteEventsOutOfOrderAndChatIsolation(t *testing.T) {
	setupTestEnv(t)
	saveMuteEvent(&MuteEvent{ChatID: Config.Chat, UserID: 42, Muted: false, CompletedAt: 200})
	// An old event is delivered after the lift and therefore has a larger ID.
	saveMuteEvent(&MuteEvent{ChatID: Config.Chat, UserID: 42, Muted: true, CompletedAt: 100})
	saveMuteEvent(&MuteEvent{ChatID: -999, UserID: 42, Muted: true, CompletedAt: 300})
	events, err := latestMuteEvents(Config.Chat)
	if err != nil || len(events) != 1 || events[0].Muted || events[0].CompletedAt != 200 {
		t.Fatalf("out-of-order/isolation: %+v %v", events, err)
	}
}

func TestOtherMuteSourcesAndLiftsAreTracked(t *testing.T) {
	for _, tc := range []struct {
		command string
		handler func(*gotgbot.Bot, *ext.Context) error
		source  string
		muted   bool
	}{
		{"/kill @victim", Kill, "kill", true},
		{"/revive @victim", Revive, "revive", false},
		{"/warn @victim", WarnUser, "warn", true},
		{"/megamlem", Shotgun, "megamlem", true},
		{"/redemption", Redemption, "redemption", false},
	} {
		t.Run(tc.source, func(t *testing.T) {
			bot, client := setupTestEnv(t)
			seedUser(42, "victim", "Жертва")
			DB.Create(&Stats{StatType: 3, ContextID: 42, DayTimestamp: GetStartOfDay(), Count: 1})
			if tc.source == "warn" {
				DB.Create(&Warn{UserID: 42, Amount: 2, LastWarn: time.Now()})
			}
			if tc.source == "redemption" {
				client.setMemberStatus(42, "restricted")
			}
			if err := tc.handler(bot, testContext(bot, tc.command, testUser(100))); err != nil {
				t.Fatal(err)
			}
			events, err := latestMuteEvents(Config.Chat)
			if err != nil || len(events) != 1 || events[0].UserID != 42 || events[0].ActorID != 100 || events[0].Source != tc.source || events[0].Muted != tc.muted {
				t.Fatalf("tracking missing: %+v, %v", events, err)
			}
		})
	}
}

func TestMuteTrackingFailurePreservesTelegramSuccess(t *testing.T) {
	bot, client := setupTestEnv(t)
	if err := DB.Migrator().DropTable(&MuteEvent{}); err != nil {
		t.Fatal(err)
	}
	ok, err := restrictChatMemberTracked(bot, Config.Chat, 42, gotgbot.ChatPermissions{}, nil, testUser(100), "mute")
	if err != nil || !ok || len(client.callsFor("restrictChatMember")) != 1 || len(client.callsFor("sendMessage")) != 0 {
		t.Fatal("journal failure changed the action or emitted a message")
	}
}

func TestMutedMissingNamesResolvedByIdentity(t *testing.T) {
	setupTestEnv(t)
	seedUser(42, "victim", "Участник")
	seedUser(777, "admin", "Маша")
	snapshot := mutedSnapshot{members: []mutedMember{{user: gotgbot.User{Id: 42}, providerID: 777}}}
	fillMutedNames(&snapshot, []MuteEvent{{ActorID: 100, ActorName: "Другой админ"}})
	if snapshot.members[0].user.FirstName != "Участник" || snapshot.members[0].provider != "Маша" {
		t.Fatal("missing names were not resolved using the live actor ID")
	}
}
