package main

import (
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

func TestZBHelpPrivateDelivery(t *testing.T) {
	for _, chatType := range []string{"group", "supergroup", "private"} {
		t.Run(chatType, func(t *testing.T) {
			bot, client := setupTestEnv(t)
			msg := testMessage("/zbhelp", testUser(42))
			msg.Chat.Type = chatType
			if chatType == "private" {
				msg.Chat.Id = 42
			} else {
				// Commands chosen from Telegram's ephemeral menu have no regular ID.
				msg.MessageId, msg.EphemeralMessageId = 0, 777
			}
			if err := ZBHelp(bot, ext.NewContext(bot, &gotgbot.Update{Message: msg}, nil)); err != nil {
				t.Fatal(err)
			}
			calls := client.callsFor("sendMessage")
			if len(calls) != 1 || calls[0].Params["chat_id"] != msg.Chat.Id {
				t.Fatalf("want one response in the invoking chat: %+v", calls)
			}
			params := calls[0].Params
			eph, _ := params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
			if chatType == "private" {
				if eph != nil {
					t.Fatal("private chats do not support ephemeral parameters")
				}
			} else if eph == nil || eph.ReceiverUserId != 42 {
				t.Fatal("help must only be visible to the caller")
			}
			text := params["text"].(string)
			if !strings.Contains(text, "команды участников") || !strings.Contains(text, "@testbot") || params["parse_mode"] != gotgbot.ParseModeHTML {
				t.Fatal("missing help content, bot username or HTML formatting")
			}
			if len(client.callsFor("getChatMember")) != 0 {
				t.Fatal("participant help must not require an admin lookup")
			}
		})
	}
}

func TestZBAdminHelpAccess(t *testing.T) {
	for _, tc := range []struct {
		name   string
		userID int64
		status string
		allow  bool
	}{
		{"configured admin", 100, "member", true},
		{"configured moderator", 300, "member", true},
		{"sysadmin", 500, "member", true},
		{"Telegram admin", 400, "administrator", true},
		{"Telegram owner", 401, "creator", true},
		{"ordinary member", 42, "member", false},
		{"former member", 43, "left", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bot, client := setupTestEnv(t)
			Config.SysAdmin = 500
			client.setMemberStatus(tc.userID, tc.status)
			if err := ZBAdminHelp(bot, testContext(bot, "/zbadminhelp", testUser(tc.userID))); err != nil {
				t.Fatal(err)
			}
			calls := client.callsFor("sendMessage")
			if len(calls) != 1 {
				t.Fatalf("responses = %d", len(calls))
			}
			eph, _ := calls[0].Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
			if eph == nil || eph.ReceiverUserId != tc.userID {
				t.Fatal("both help and denials must be private to the caller")
			}
			text := calls[0].Params["text"].(string)
			if strings.Contains(text, "/restart") != tc.allow {
				t.Fatalf("incorrect access to admin help: %q", text)
			}
		})
	}
}

func TestZBAdminHelpPrivateChatAndRevokedRights(t *testing.T) {
	bot, client := setupTestEnv(t)
	msg := testMessage("/zbadminhelp", testUser(400))
	msg.Chat.Id, msg.Chat.Type = 400, "private"
	ctx := ext.NewContext(bot, &gotgbot.Update{Message: msg}, nil)
	client.setMemberStatus(400, "administrator")
	if err := ZBAdminHelp(bot, ctx); err != nil {
		t.Fatal(err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 || calls[0].Params["chat_id"] != int64(400) || !strings.Contains(calls[0].Params["text"].(string), "/restart") {
		t.Fatal("admin help did not reach the caller's private chat")
	}
	if eph, _ := calls[0].Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters); eph != nil {
		t.Fatal("private response has unsupported ephemeral parameters")
	}
	checks := client.callsFor("getChatMember")
	if len(checks) != 1 || checks[0].Params["chat_id"] != Config.Chat {
		t.Fatal("admin rights must be checked in the configured main chat")
	}
	for _, failedLookup := range []bool{false, true} {
		client.clearCalls()
		client.setMemberStatus(400, "member")
		if failedLookup {
			client.setError("getChatMember", errors.New("lookup unavailable"))
		}
		if err := ZBAdminHelp(bot, ctx); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.Join(client.sentTexts(), ""), "/restart") {
			t.Fatal("revoked or unverified permissions must not expose admin help")
		}
	}
}

func TestZBHelpNoPublicFallback(t *testing.T) {
	for _, command := range []string{"zbhelp", "zbadminhelp"} {
		for _, userID := range []int64{42, 100} {
			bot, client := setupTestEnv(t)
			failure := errors.New("ephemeral delivery unavailable")
			client.setError("sendMessage", failure)
			var handler handlers.Response = ZBHelp
			if command == "zbadminhelp" {
				handler = ZBAdminHelp
			}
			if err := handler(bot, testContext(bot, "/"+command, testUser(userID))); !errors.Is(err, failure) {
				t.Fatalf("lost delivery failure: %v", err)
			}
			if calls := client.callsFor("sendMessage"); len(calls) != 1 {
				t.Fatal("failed private response retried publicly")
			}
		}
	}
}

func TestZBHelpRegistrationAndRouting(t *testing.T) {
	bot, client := setupTestEnv(t)
	dispatcher := ext.NewDispatcher(nil)
	for _, entry := range commandListConfig() {
		dispatcher.AddHandler(handlers.NewCommand(entry.command.Command, entry.response))
	}
	for _, tc := range []struct {
		command string
		userID  int64
		want    string
	}{
		{"/zbhelp", 42, "команды участников"},
		{"/zbhelp@testbot", 42, "команды участников"},
		{"/zbadminhelp", 100, "команды администраторов"},
		{"/zbadminhelp@testbot", 42, "доступна только"},
		{"/zbhelp@otherbot", 42, ""},
	} {
		client.clearCalls()
		if err := dispatcher.ProcessUpdate(bot, &gotgbot.Update{Message: testMessage(tc.command, testUser(tc.userID))}, nil); err != nil {
			t.Fatal(err)
		}
		texts := client.sentTexts()
		if tc.want == "" {
			if len(texts) != 0 {
				t.Fatal("command intended for another bot was handled")
			}
		} else if len(texts) != 1 || !strings.Contains(texts[0], tc.want) {
			t.Fatalf("%s routed incorrectly: %q", tc.command, texts)
		}
	}
}

func TestZBHelpMenuScopes(t *testing.T) {
	group, private, admin := commandMenus(commandListConfig())
	for _, menu := range []struct {
		name     string
		commands []gotgbot.BotCommand
	}{
		{"group", group}, {"private", private}, {"admin", admin},
	} {
		if len(menu.commands) > 100 {
			t.Fatalf("%s menu exceeds Telegram's limit", menu.name)
		}
		foundHelp, foundAdmin := false, false
		for _, command := range menu.commands {
			switch command.Command {
			case "zbhelp":
				foundHelp = command.IsEphemeral
			case "zbadminhelp":
				foundAdmin = command.IsEphemeral
			}
		}
		if !foundHelp || foundAdmin != (menu.name == "admin") {
			t.Fatalf("wrong help visibility in %s menu", menu.name)
		}
	}
}

func TestZBHelpContentAndSize(t *testing.T) {
	commandRE := regexp.MustCompile(`(?:^|\s)/([a-z][a-z0-9_]*)`)
	public, admin := map[string]bool{}, map[string]bool{}
	for _, doc := range []struct {
		text     string
		commands map[string]bool
	}{
		{zbHelpText, public}, {zbAdminHelpText, admin},
	} {
		decoder := xml.NewDecoder(strings.NewReader("<help>" + doc.text + "</help>"))
		var visible strings.Builder
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("invalid help markup: %v", err)
			}
			if data, ok := token.(xml.CharData); ok {
				visible.Write(data)
			}
		}
		if units := len(utf16.Encode([]rune(visible.String()))); units > 4096 || units == 0 {
			t.Fatalf("help must fit one Telegram message, got %d UTF-16 units", units)
		}
		for _, match := range commandRE.FindAllStringSubmatch(visible.String(), -1) {
			doc.commands[match[1]] = true
		}
	}
	registered := map[string]bool{}
	for _, entry := range commandListConfig() {
		name := entry.command.Command
		registered[name] = true
		if public[name] == admin[name] {
			t.Errorf("/%s must be described in exactly one help category", name)
		}
	}
	for name := range public {
		if !registered[name] {
			t.Errorf("unregistered public command /%s", name)
		}
	}
	for name := range admin {
		if !registered[name] {
			t.Errorf("unregistered admin command /%s", name)
		}
	}
	for _, name := range []string{"getid", "say", "kill", "muted", "pidorlist", "addsalvation", "perf", "restart"} {
		if !admin[name] {
			t.Errorf("restricted command /%s listed for ordinary participants", name)
		}
	}
	for _, name := range []string{"set", "del", "slap", "mywarns", "blessing", "duel"} {
		if !public[name] {
			t.Errorf("participant command /%s missing from public help", name)
		}
	}
}

func TestZBHelpMissingContext(t *testing.T) {
	bot, client := setupTestEnv(t)
	for _, handler := range []handlers.Response{ZBHelp, ZBAdminHelp} {
		if err := handler(bot, ext.NewContext(bot, &gotgbot.Update{}, nil)); err != nil {
			t.Fatal(err)
		}
	}
	if len(client.callsFor("sendMessage")) != 0 {
		t.Fatal("response sent without a recipient")
	}
}
