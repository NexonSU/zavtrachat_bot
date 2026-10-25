package main

import (
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func processTestUpdate(t *testing.T, d *ext.Dispatcher, bot *gotgbot.Bot, update *gotgbot.Update) {
	t.Helper()
	if err := d.ProcessUpdate(bot, update, nil); err != nil {
		t.Fatal(err)
	}
}

func TestRegisteredCommandsThroughMiddleware(t *testing.T) {
	for _, tc := range []struct {
		name, text         string
		user               int64
		ephemeral, allowed bool
	}{
		{"ordinary warning", "/warn", 100, false, true},
		{"menu warning", "/warn@testbot", 100, true, true},
		{"denied warning", "/warn@testbot", 42, true, false},
		{"different bot", "/warn@otherbot", 100, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bot, client := setupTestEnv(t)
			Config.AntiSpamCount = 100
			dispatcher := newBotDispatcher()
			registerBotHandlers(dispatcher)
			msg := testMessage(tc.text, testUser(tc.user))
			msg.ReplyToMessage = testMessage("target", testUser(777))
			if tc.ephemeral {
				msg.MessageId = 0
				msg.EphemeralMessageId = 456
			}
			processTestUpdate(t, dispatcher, bot, &gotgbot.Update{UpdateId: 1000, Message: msg})
			var count int64
			DB.Model(&Warn{}).Count(&count)
			if (count == 1) != tc.allowed {
				t.Fatalf("wrong routing/permission result: rows=%d", count)
			}
			calls := client.callsFor("sendMessage")
			if tc.allowed {
				if len(calls) != 2 {
					t.Fatalf("notifications=%d", len(calls))
				}
				for i, id := range []int64{777, tc.user} {
					params := calls[i].Params
					eph, _ := params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
					if params["chat_id"] != int64(-100) || eph == nil || eph.ReceiverUserId != id {
						t.Fatalf("wrong private receiver: %+v", params)
					}
				}
			}
			if tc.name == "denied warning" && (len(calls) != 1 || calls[0].Params["text"] != "нет") {
				t.Fatal("missing denial")
			}
			if tc.name == "different bot" {
				waitForCalls(t, client, "getChatMember", 1)
				if len(client.callsFor("restrictChatMember")) != 0 || len(calls) != 0 {
					t.Fatal("handled another bot's command")
				}
			}
		})
	}
}

func TestRegisteredAnonymousAndEmptyUpdates(t *testing.T) {
	bot, client := setupTestEnv(t)
	dispatcher := newBotDispatcher()
	registerBotHandlers(dispatcher)
	for _, update := range []*gotgbot.Update{{}, {Message: testMessage("/kill 42", nil)}, {Message: testMessage("anonymous text", nil)}} {
		processTestUpdate(t, dispatcher, bot, update)
	}
	if len(client.sentTexts()) != 0 || len(client.callsFor("restrictChatMember")) != 0 {
		t.Fatal("acted without sender")
	}
	var count int64
	DB.Model(&Duelist{}).Count(&count)
	if count != 0 {
		t.Fatal("anonymous update changed counters")
	}
}

func TestRegisteredDuelAndPhraseCallbacks(t *testing.T) {
	t.Run("duel denial", func(t *testing.T) {
		bot, client, ctx := duelScenario(t, 777)
		dispatcher := newBotDispatcher()
		registerBotHandlers(dispatcher)
		ctx.CallbackQuery.Data = "russianroulette_deny"
		processTestUpdate(t, dispatcher, bot, ctx.Update)
		assertDuelReleased(t)
		if len(client.callsFor("answerCallbackQuery")) != 1 || len(client.callsFor("editMessageText")) != 1 || len(client.callsFor("restrictChatMember")) != 0 {
			t.Fatal("duel callback not routed")
		}
	})
	t.Run("phrase confirmation and permission recheck", func(t *testing.T) {
		bot, client := setupBlessListTest(t, "<phrase> & safe", "another phrase")
		dispatcher := newBotDispatcher()
		registerBotHandlers(dispatcher)
		ctx := blessPrivateContext(bot, "/blesslist", 100)
		processTestUpdate(t, dispatcher, bot, ctx.Update)
		data := blessButton(t, client, "Удалить №1")
		msg := testMessage("keyboard", testUser(1))
		msg.MessageId = 555
		msg.Chat = gotgbot.Chat{Id: 100, Type: "private"}
		callback := testCallbackContext(bot, msg, testUser(100))
		callback.CallbackQuery.Data = data
		processTestUpdate(t, dispatcher, bot, callback.Update)
		confirm := blessButton(t, client, "Да, удалить")
		// An ordinary participant cannot use an administrator's keyboard.
		callback.CallbackQuery.From = *testUser(42)
		callback.CallbackQuery.Data = confirm
		processTestUpdate(t, dispatcher, bot, callback.Update)
		var count int64
		DB.Model(&Bless{}).Count(&count)
		if count != 2 {
			t.Fatal("unauthorized phrase deletion")
		}
		callback.CallbackQuery.From = *testUser(100)
		processTestUpdate(t, dispatcher, bot, callback.Update)
		DB.Model(&Bless{}).Count(&count)
		if count != 1 {
			t.Fatal("registered confirmation did not delete phrase")
		}
	})
}

func TestRegisteredChatMemberPersistsOneEvent(t *testing.T) {
	bot, client := setupTestEnv(t)
	dispatcher := newBotDispatcher()
	registerBotHandlers(dispatcher)
	member := &gotgbot.ChatMemberUpdated{Chat: gotgbot.Chat{Id: Config.Chat, Type: "supergroup"}, From: *testUser(100), Date: time.Now().Unix(),
		OldChatMember: gotgbot.ChatMemberMember{User: *testUser(42)},
		NewChatMember: gotgbot.ChatMemberRestricted{User: *testUser(42), IsMember: true, CanSendMessages: false, UntilDate: time.Now().Add(time.Hour).Unix()},
	}
	update := &gotgbot.Update{UpdateId: 8765, ChatMember: member}
	processTestUpdate(t, dispatcher, bot, update)
	processTestUpdate(t, dispatcher, bot, update)
	var events []MuteEvent
	if err := DB.Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ActorID != 100 || events[0].UserID != 42 || events[0].Source != "manual" || !events[0].Muted {
		t.Fatalf("wrong persisted event: %+v", events)
	}
	if strings.Contains(strings.Join(client.sentTexts(), ""), "mute") {
		t.Fatal("event leaked publicly")
	}
}
