package main

import (
	"encoding/json"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestGenderCommandsAdminOnly(t *testing.T) {
	bot, client := setupTestEnv(t)
	target := gotgbot.User{Id: 77, Username: "cactus", FirstName: "Кактус"}
	if err := DB.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	client.Results = map[string]json.RawMessage{
		"getChatMember": json.RawMessage(`{"status":"member","user":{"id":77,"username":"cactus","first_name":"Кактус"}}`),
	}
	for _, tc := range []struct {
		command string
		handler func(*gotgbot.Bot, *ext.Context) error
		sender  int64
		private bool
		want    string
	}{
		{"/gendercheck", GenderCheck, 100, true, "@cactus мальчик"},
		{"/genderswap", GenderSwap, 42, true, ""},
		{"/genderswap", GenderSwap, 300, true, ""},
		{"/genderswap", GenderSwap, 42, false, ""},
		{"/genderswap", GenderSwap, 100, false, "@cactus теперь девочка"},
		{"/gendercheck", GenderCheck, 100, false, "@cactus девочка"},
		{"/gendercheck", GenderCheck, 100, true, "@cactus девочка"},
		{"/genderswap", GenderSwap, 100, true, "@cactus теперь мальчик"},
		{"/gendercheck", GenderCheck, 100, true, "@cactus мальчик"},
	} {
		client.clearCalls()
		msg := testMessage(tc.command+" @CACTUS", testUser(tc.sender))
		if tc.private {
			msg.Chat.Id, msg.Chat.Type = tc.sender, "private"
		}
		ctx := ext.NewContext(bot, &gotgbot.Update{Message: msg}, nil)
		if err := tc.handler(bot, ctx); err != nil {
			t.Fatal(err)
		}
		calls := client.callsFor("sendMessage")
		if tc.want == "" {
			if len(calls) != 0 {
				t.Fatalf("%s exposed a reply: %+v", tc.command, calls)
			}
			continue
		}
		if len(calls) != 1 || calls[0].Params["text"] != tc.want || calls[0].Params["chat_id"] != msg.Chat.Id {
			t.Fatalf("%s sent %+v, want %q in %d", tc.command, calls, tc.want, msg.Chat.Id)
		}
		eph, _ := calls[0].Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
		if tc.private && eph != nil || !tc.private && (eph == nil || eph.ReceiverUserId != tc.sender) {
			t.Fatalf("%s used incorrect reply visibility: %+v", tc.command, eph)
		}
		if !tc.private && len(client.callsFor("deleteMessage")) != 1 {
			t.Fatalf("%s did not remove the typed command", tc.command)
		}
	}
}
