package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestQuoteAdministratorAccess(t *testing.T) {
	for _, tc := range []struct {
		name    string
		id      int64
		status  string
		allowed bool
	}{
		{"configured admin", 100, "member", true}, {"sysadmin", 500, "member", true},
		{"Telegram admin", 400, "administrator", true}, {"owner", 401, "creator", true},
		{"moderator only", 300, "member", false}, {"member", 42, "member", false}, {"left", 43, "left", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bot, client := setupTestEnv(t)
			Config.SysAdmin = 500
			client.setMemberStatus(tc.id, tc.status)
			if got := canUseQuote(bot, tc.id); got != tc.allowed {
				t.Fatalf("access=%v want=%v", got, tc.allowed)
			}
			if !tc.allowed {
				ctx := testReplyContext(bot, "/quote", testUser(tc.id), &gotgbot.Message{Text: "Secret quote", From: &gotgbot.User{Id: 77}})
				if err := Quote(bot, ctx); err != nil {
					t.Fatal(err)
				}
				if len(client.callsFor("sendPhoto")) != 0 || len(client.callsFor("getUserProfilePhotos")) != 0 {
					t.Fatal("denied user triggered quote")
				}
				calls := client.callsFor("sendMessage")
				if len(calls) != 1 {
					t.Fatal("missing denial")
				}
				eph, _ := calls[0].Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
				if eph == nil || eph.ReceiverUserId != tc.id {
					t.Fatal("denial not requester-only")
				}
			}
		})
	}
	bot, client := setupTestEnv(t)
	client.setError("getChatMember", errors.New("unavailable"))
	if canUseQuote(bot, 42) {
		t.Fatal("permission lookup failure allowed access")
	}
}

func TestQuoteHiddenFromPublicMenus(t *testing.T) {
	group, private, admin := commandMenus(commandListConfig())
	for _, menu := range [][]gotgbot.BotCommand{group, private} {
		for _, c := range menu {
			if c.Command == "quote" {
				t.Fatal("quote in public/private general menu")
			}
		}
	}
	found := false
	for _, c := range admin {
		if c.Command == "quote" {
			found = true
		}
	}
	if !found || strings.Contains(zbHelpText, "/quote") || !strings.Contains(zbAdminHelpText, "/quote") {
		t.Fatal("quote menu/help visibility wrong")
	}
}

func TestQuotePrivateDeliveryErrorsNeverPublic(t *testing.T) {
	for _, failNotice := range []bool{false, true} {
		t.Run(map[bool]string{false: "notice succeeds", true: "notice fails"}[failNotice], func(t *testing.T) {
			bot, client := setupTestEnv(t)
			Config.AntiSpamCount = 100
			client.setError("sendPhoto", errors.New("photo delivery failed"))
			if failNotice {
				client.setError("sendMessage", errors.New("private notice failed"))
			}
			dispatcher := newBotDispatcher()
			registerBotHandlers(dispatcher)
			msg := testMessage("/quote", testUser(100))
			msg.ReplyToMessage = testMessage("Quote", testUser(77))
			processTestUpdate(t, dispatcher, bot, &gotgbot.Update{UpdateId: 1000, Message: msg})
			photos := client.callsFor("sendPhoto")
			if len(photos) != 1 {
				t.Fatal("missing photo attempt")
			}
			for _, call := range append(photos, client.callsFor("sendMessage")...) {
				eph, _ := call.Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
				if eph == nil || eph.ReceiverUserId != 100 {
					t.Fatalf("public or wrong recipient: %+v", call)
				}
			}
			if len(client.callsFor("sendMessage")) != 1 {
				t.Fatal("delivery failure retried or not reported")
			}
		})
	}
}

func TestQuoteInPrivateChat(t *testing.T) {
	bot, client := setupTestEnv(t)
	msg := testMessage("/quote", testUser(100))
	msg.Chat.Id = 100
	msg.Chat.Type = "private"
	msg.ReplyToMessage = testMessage("Quote", testUser(77))
	if err := Quote(bot, testReplyContext(bot, "/quote", testUser(100), msg.ReplyToMessage)); err != nil {
		t.Fatal(err)
	}
	client.clearCalls()
	ctx := blessPrivateContext(bot, "/quote", 100)
	ctx.Message.ReplyToMessage = msg.ReplyToMessage
	if err := Quote(bot, ctx); err != nil {
		t.Fatal(err)
	}
	calls := client.callsFor("sendPhoto")
	if len(calls) != 1 || calls[0].Params["chat_id"] != int64(100) {
		t.Fatal("private card wrong destination")
	}
	if p, _ := calls[0].Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters); p != nil {
		t.Fatal("PM must not use ephemeral parameters")
	}
}
