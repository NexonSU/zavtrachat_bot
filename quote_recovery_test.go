package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/gotd/td/tg"
)

type quoteAPIStub struct {
	result    tg.MessagesMessagesClass
	err       error
	channelID int64
	ids       []tg.InputMessageClass
	plain     bool
}

func (a *quoteAPIStub) ChannelsGetMessages(_ context.Context, req *tg.ChannelsGetMessagesRequest) (tg.MessagesMessagesClass, error) {
	a.channelID = req.Channel.(*tg.InputChannel).ChannelID
	a.ids = req.ID
	return a.result, a.err
}
func (a *quoteAPIStub) MessagesGetMessages(_ context.Context, ids []tg.InputMessageClass) (tg.MessagesMessagesClass, error) {
	a.plain = true
	a.ids = ids
	return a.result, a.err
}
func TestQuoteRecoversCommandReplyHeader(t *testing.T) {
	chatID := int64(-1000000000042)
	source := &gotgbot.Message{MessageId: 50, Text: "Длинное вступление. Смысл жизни — движение. И заключение."}
	command := &gotgbot.Message{MessageId: 51, Text: "/quote", Chat: gotgbot.Chat{Id: chatID}, ReplyToMessage: source}
	header := &tg.MessageReplyHeader{ReplyToMsgID: 50, QuoteText: "Смысл жизни — движение."}
	remote := &tg.Message{ID: 51, Message: "/quote", PeerID: &tg.PeerChannel{ChannelID: 42}, ReplyTo: header}
	api := &quoteAPIStub{result: &tg.MessagesChannelMessages{Messages: []tg.MessageClass{remote}}}
	selected, err := quoteSelectionFromTelegram(context.Background(), api, command)
	if err != nil || selected != header.QuoteText || api.channelID != 42 || len(api.ids) != 1 || api.ids[0].(*tg.InputMessageID).ID != 51 {
		t.Fatalf("wrong command queried or selection: %q %v %+v", selected, err, api)
	}
	for _, tc := range []string{"source instead of command", "different chat", "different reply", "different command", "external reply"} {
		t.Run(tc, func(t *testing.T) {
			copyMessage := *remote
			copyHeader := *header
			copyMessage.ReplyTo = &copyHeader
			switch tc {
			case "source instead of command":
				copyMessage.ID = 50
			case "different chat":
				copyMessage.PeerID = &tg.PeerChannel{ChannelID: 99}
			case "different reply":
				copyHeader.ReplyToMsgID = 99
			case "different command":
				copyMessage.Message = "/other"
			case "external reply":
				copyHeader.ReplyToPeerID = &tg.PeerChannel{ChannelID: 99}
			}
			stub := &quoteAPIStub{result: &tg.MessagesChannelMessages{Messages: []tg.MessageClass{&copyMessage}}}
			if _, err := quoteSelectionFromTelegram(context.Background(), stub, command); err == nil {
				t.Fatal("accepted unrelated reply header")
			}
		})
	}
	command.Chat.Id = -42
	remote.PeerID = &tg.PeerChat{ChatID: 42}
	api.result = &tg.MessagesMessages{Messages: []tg.MessageClass{remote}}
	if text, err := quoteSelectionFromTelegram(context.Background(), api, command); err != nil || text != header.QuoteText || !api.plain {
		t.Fatalf("basic group recovery: %q %v", text, err)
	}
	api.err = errors.New("offline")
	if _, err := quoteSelectionFromTelegram(context.Background(), api, command); err == nil {
		t.Fatal("ignored lookup failure")
	}
}
func TestQuoteRecoveryValidatesSelection(t *testing.T) {
	source := &gotgbot.Message{Text: "Начало. Не сдавайся. Иди на хуй.", Date: 123, From: &gotgbot.User{Id: 77, FirstName: "Автор"}}
	message := &gotgbot.Message{ReplyToMessage: source}
	q, ok, status := quoteResolvedContent(message, func(*gotgbot.Message) (string, error) { return "Не сдавайся.", nil })
	if !ok || status != "native" || q.Text != "Не сдавайся." || q.UserID != 77 || q.Date != 123 || quoteForbidden(q.Text) {
		t.Fatalf("lost recovered selection/attribution: %+v %v %s", q, ok, status)
	}
	_, ok, _ = quoteResolvedContent(message, func(*gotgbot.Message) (string, error) { return "Подменённый текст", nil })
	if ok {
		t.Fatal("accepted forged fragment")
	}
	q, ok, _ = quoteResolvedContent(message, func(*gotgbot.Message) (string, error) { return "Иди на хуй.", nil })
	if !ok || !quoteForbidden(q.Text) {
		t.Fatal("recovered fragment bypassed filter")
	}
	for _, lookup := range []quoteLookup{
		func(*gotgbot.Message) (string, error) { return "", nil },
		func(*gotgbot.Message) (string, error) { return "", errors.New("private error") },
	} {
		q, ok, status = quoteResolvedContent(message, lookup)
		if !ok || q.Text != source.Text || quoteSelectionNotice(status) == "" {
			t.Fatal("silent whole-message fallback")
		}
	}
	message.Quote = &gotgbot.TextQuote{Text: "Не сдавайся."}
	_, _, status = quoteResolvedContent(message, func(*gotgbot.Message) (string, error) {
		t.Fatal("native lookup despite Bot API selection")
		return "", nil
	})
	if status != "bot_api" {
		t.Fatal(status)
	}
}
func TestQuoteDiagnosticIsPrivate(t *testing.T) {
	bot, client := setupTestEnv(t)
	source := testMessage("Секретное вступление. Не сдавайся.", testUser(77))
	ctx := testReplyContext(bot, "/quote debug", testUser(100), source)
	err := quoteWithLookup(bot, ctx, func(*gotgbot.Message) (string, error) { return "Не сдавайся.", nil })
	if err != nil {
		t.Fatal(err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatal(calls)
	}
	text, _ := calls[0].Params["text"].(string)
	if !strings.Contains(text, "native-recovery-v1") || !strings.Contains(text, "восстановлен") || strings.Contains(text, "Не сдавайся") || strings.Contains(text, "Секретное") {
		t.Fatal("unsafe or incomplete diagnostic", text)
	}
	eph, _ := calls[0].Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
	if eph == nil || eph.ReceiverUserId != 100 || len(client.callsFor("sendPhoto")) != 0 || len(client.callsFor("getUserProfilePhotos")) != 0 {
		t.Fatal("diagnostic not private or triggered media")
	}
}
