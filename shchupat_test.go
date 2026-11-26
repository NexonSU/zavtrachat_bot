package main

import (
	"reflect"
	"slices"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestShchupatGIFQuotesMatchedWord(t *testing.T) {
	bot, client := setupTestEnv(t)
	oldBudget := shchupatReplies.last
	t.Cleanup(func() { shchupatReplies.last = oldBudget })

	for _, tc := range []struct {
		name, text, caption string
		captionEntities     []gotgbot.MessageEntity
		quote               string
		position            int64
		quoteEntities       []gotgbot.MessageEntity
	}{
		{name: "first word", text: "щупал, но не нащупал", quote: "щупал"},
		{name: "emoji and punctuation", text: "🤔 «ПОЩУПАЛА!»", quote: "ПОЩУПАЛА", position: 4},
		{
			name: "formatted media caption", caption: "Я щупала вчера.",
			captionEntities: []gotgbot.MessageEntity{{Type: "bold", Offset: 0, Length: 8}},
			quote:           "щупала", position: 2,
			quoteEntities: []gotgbot.MessageEntity{{Type: "bold", Offset: 0, Length: 6}},
		},
		{name: "no whole word", text: "щупал_123"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client.clearCalls()
			msg := testMessage(tc.text, testUser(42))
			msg.Chat.Type = "supergroup"
			msg.MessageThreadId = 123
			msg.Caption = tc.caption
			msg.CaptionEntities = tc.captionEntities

			replyWithShchupatGIF(bot, msg)

			calls := client.callsFor("sendAnimation")
			if tc.quote == "" {
				if len(calls) != 0 {
					t.Fatalf("sendAnimation calls = %d, want 0", len(calls))
				}
				return
			}
			if len(calls) != 1 {
				t.Fatalf("sendAnimation calls = %d, want 1", len(calls))
			}
			params := calls[0].Params
			if !reflect.DeepEqual(params["animation"], gotgbot.InputFileByID(shchupatGIF)) || params["message_thread_id"] != msg.MessageThreadId {
				t.Fatalf("unexpected animation or topic in sendAnimation: %v", params)
			}
			reply, ok := params["reply_parameters"].(*gotgbot.ReplyParameters)
			if !ok || reply.MessageId != msg.MessageId || reply.Quote != tc.quote || reply.QuotePosition != tc.position || !slices.Equal(reply.QuoteEntities, tc.quoteEntities) {
				t.Fatalf("reply = %+v, want message %d, quote %q at %d with entities %v", reply, msg.MessageId, tc.quote, tc.position, tc.quoteEntities)
			}
		})
	}
}
