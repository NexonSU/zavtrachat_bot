package main

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestQuoteCaptionIdentifiesRequester(t *testing.T) {
	for _, tc := range []struct {
		name     string
		chatType string
		userID   int64
	}{
		{"member in supergroup", "supergroup", 42},
		{"admin in group", "group", 100},
		{"member in private", "private", 42},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bot, client := setupTestEnv(t)
			requester := testUser(tc.userID)
			requester.FirstName, requester.LastName = "Кот <&>", "Тест"
			source := testMessage("Слова исходного автора.", testUser(77))
			source.ForwardOrigin = gotgbot.MessageOriginUser{Date: 123, SenderUser: *testUser(88)}
			ctx := testReplyContext(bot, "/quote чужое имя", requester, source)
			ctx.Message.Chat.Type = tc.chatType
			if tc.chatType == "private" {
				ctx.Message.Chat.Id = requester.Id
			}
			if err := Quote(bot, ctx); err != nil {
				t.Fatal(err)
			}
			photos := client.callsFor("sendPhoto")
			if len(photos) != 1 {
				t.Fatal("missing quote card")
			}
			want := "Цитату запросил(а): " + MentionUser(requester)
			if photos[0].Params["caption"] != want || photos[0].Params["parse_mode"] != gotgbot.ParseModeHTML {
				t.Fatalf("wrong requester or unescaped name: %+v", photos[0].Params)
			}
			if photos[0].Params["chat_id"] != ctx.EffectiveChat.Id {
				t.Fatal("caption sent to the wrong chat")
			}
			if eph, _ := photos[0].Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters); eph != nil {
				t.Fatal("quote card is not public")
			}
			if len(client.callsFor("sendMessage")) != 0 {
				t.Fatal("caption must be attached to the photo")
			}
		})
	}
}
