package main

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"strings"
	"testing"
)

func TestQuoteProtectedIdentity(t *testing.T) {
	setupTestEnv(t)
	Config.QuoteProtectedUsernames = []string{"@ExampleProtected"}
	author := testUser(77)
	author.Username = "EXAMPLEPROTECTED"
	source := testMessage("Достаточно длинная цитата.", author)
	restricted, err := quoteAuthorRestricted(source)
	if err != nil || !restricted {
		t.Fatal(restricted, err)
	}
	author.Username = "renamed"
	if restricted, err = quoteAuthorRestricted(source); err != nil || !restricted {
		t.Fatal("renamed protected identity allowed", restricted, err)
	}
	other := testUser(88)
	other.Username = "ordinary"
	forwarded := testMessage(source.Text, other)
	forwarded.ForwardOrigin = gotgbot.MessageOriginUser{Date: 123, SenderUser: *author}
	if restricted, err = quoteAuthorRestricted(forwarded); err != nil || !restricted {
		t.Fatal("forwarded identity allowed", restricted, err)
	}
	forwarded.ForwardOrigin = gotgbot.MessageOriginHiddenUser{Date: 123, SenderUserName: "Unverifiable author"}
	if restricted, err = quoteAuthorRestricted(forwarded); err != nil || !restricted {
		t.Fatal("hidden forward bypass", restricted, err)
	}
	if restricted, err = quoteAuthorRestricted(testMessage(source.Text, other)); err != nil || restricted {
		t.Fatal("ordinary author blocked", restricted, err)
	}
	Config.QuoteProtectedUsernames = []string{"differentProtected"}
	if restricted, err = quoteAuthorRestricted(source); err != nil || restricted {
		t.Fatal("removed entry still blocks", restricted, err)
	}
}

func TestQuoteProtectedRefusalAndAdminBypass(t *testing.T) {
	bot, client := setupTestEnv(t)
	Config.QuoteProtectedUsernames = []string{"exampleProtected"}
	author := testUser(77)
	author.Username = "exampleProtected"
	source := testMessage("Достаточно длинная цитата.", author)
	ctx := testReplyContext(bot, "/quote", testUser(42), source)
	if err := quoteWithLookup(bot, ctx, func(*gotgbot.Message) (string, error) { t.Fatal("blocked quote fetched metadata"); return "", nil }); err != nil {
		t.Fatal(err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatal(calls)
	}
	text, _ := calls[0].Params["text"].(string)
	if strings.Contains(strings.ToLower(text), "exampleprotected") || !strings.Contains(text, "администратор") {
		t.Fatal("unsafe refusal", text)
	}
	eph, _ := calls[0].Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
	if eph == nil || eph.ReceiverUserId != 42 || len(client.callsFor("sendPhoto")) != 0 || len(client.callsFor("getUserProfilePhotos")) != 0 {
		t.Fatal("refusal not private or rendered media")
	}
	var count int64
	if err := DB.Model(&QuoteBudget{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("refusal spent quota", count, err)
	}
	client.clearCalls()
	ctx = testReplyContext(bot, "/quote debug", testUser(100), source)
	ctx.Message.Quote = &gotgbot.TextQuote{Text: source.Text}
	if err := Quote(bot, ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(client.sentTexts(), " "), "native-recovery-v1") {
		t.Fatal("admin was restricted")
	}
}
