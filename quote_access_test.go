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
		})
	}
	bot, client := setupTestEnv(t)
	client.setError("getChatMember", errors.New("unavailable"))
	if canUseQuote(bot, 42) {
		t.Fatal("permission lookup failure allowed access")
	}
}

func TestQuotePublicMenu(t *testing.T) {
	group, private, admin := commandMenus(commandListConfig())
	_ = private
	foundPublic := false
	for _, c := range group {
		if c.Command == "quote" && !c.IsEphemeral {
			foundPublic = true
		}
	}
	if !foundPublic {
		t.Fatal("missing public quote menu")
	}
	found := false
	for _, c := range admin {
		if c.Command == "quote" {
			found = true
			if c.IsEphemeral {
				t.Fatal("quote command must preserve native Quote Reply as an ordinary message")
			}
		}
	}
	if !found || !strings.Contains(zbHelpText, "/quote") {
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
			msg := testMessage("/quote", testUser(42))
			msg.ReplyToMessage = testMessage("Quote", testUser(77))
			processTestUpdate(t, dispatcher, bot, &gotgbot.Update{UpdateId: 1000, Message: msg})
			var used int64
			if err := DB.Model(&QuoteBudget{}).Select("COALESCE(SUM(used),0)").Scan(&used).Error; err != nil || used != 0 {
				t.Fatalf("failed send consumed quota: %d %v", used, err)
			}
			if len(client.callsFor("deleteMessage")) != 0 {
				t.Fatal("failed quote deleted command")
			}
			photos := client.callsFor("sendPhoto")
			if len(photos) != 1 {
				t.Fatal("missing photo attempt")
			}
			for _, call := range client.callsFor("sendMessage") {
				eph, _ := call.Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
				if eph == nil || eph.ReceiverUserId != 42 {
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
