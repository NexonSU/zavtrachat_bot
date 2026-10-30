package main

import (
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestQuoteForbiddenTexts(t *testing.T) {
	for _, text := range []string{"нет", "Ого!", "Да", "тест", "ага 😎", "хуй", "пшел нахуй", "ПОШЁЛ НА ХУЙ!", "пшёл нахуй", "пошол-на-хуй", "иди нахуй", "иди на хуй", "идите нахуй", "пошла на хуй", "пошли нахуй", "ну ты иди на хуй конечно", "пооошееел нааа хуууй", "п о ш е л н а х у й", "пошёл ты нахуй", "иди-ка ты на хуй"} {
		if !quoteForbidden(text) {
			t.Errorf("allowed forbidden quote: %q", text)
		}
	}
	for _, text := range []string{"Привет", "Смысл жизни", "Да, конечно", "Нет времени объяснять", "Никто никуда не пошёл", "Пошёл на прогулку", "Иди на кухню", "непошелнахуйный", "Идиллия на хуторе", "2026", "RTX4090"} {
		if quoteForbidden(text) {
			t.Errorf("rejected unrelated quote: %q", text)
		}
	}
}

func TestQuoteNativeSelection(t *testing.T) {
	source := &gotgbot.Message{Text: "😎 Сначала длинное вступление. Смысл жизни — движение. И заключение.", Date: 123, From: &gotgbot.User{Id: 77, FirstName: "Иван"}}
	selected := "Смысл жизни — движение."
	position := len(utf16.Encode([]rune(strings.Split(source.Text, selected)[0])))
	msg := &gotgbot.Message{ReplyToMessage: source, Quote: &gotgbot.TextQuote{Text: selected, Position: int64(position), IsManual: true}}
	q, ok := quoteSelectedContent(msg)
	if !ok || q.Text != selected || q.Author != "Иван" || q.UserID != 77 || q.Date != 123 {
		t.Fatalf("selection changed attribution: %+v %v", q, ok)
	}
	msg.Quote.Position++ // Approximate UTF-16 positions do not corrupt the selected text.
	if q, ok := quoteSelectedContent(msg); !ok || q.Text != selected {
		t.Fatal("approximate offset rejected valid selection")
	}
	msg.Quote.IsManual = false
	if q, ok := quoteSelectedContent(msg); !ok || q.Text != selected {
		t.Fatal("quote text ignored when is_manual is false")
	}
	msg.Quote.IsManual = true
	msg.Quote.Text = "Подменённая цитата"
	if _, ok := quoteSelectedContent(msg); ok {
		t.Fatal("forged text attributed to source author")
	}
	source.Text = ""
	source.Caption = "Цитата из подписи к фотографии"
	msg.Quote.Text = "подписи к фотографии"
	if q, ok := quoteSelectedContent(msg); !ok || q.Text != msg.Quote.Text {
		t.Fatal("caption selection failed")
	}
}

func TestQuoteRefusalIsPrivateAndBeforeAvatar(t *testing.T) {
	bot, client := setupTestEnv(t)
	source := testMessage("Не сдавайся. Иди на хуй.", testUser(77))
	for _, selected := range []string{"", "Иди на хуй.", "нет"} {
		client.clearCalls()
		reply := *source
		if selected == "нет" {
			reply.Text = "Сначала слово нет, а потом продолжение."
		}
		ctx := testReplyContext(bot, "/quote", testUser(100), &reply)
		if selected != "" {
			ctx.Message.Quote = &gotgbot.TextQuote{Text: selected, IsManual: true}
		}
		if err := Quote(bot, ctx); err != nil {
			t.Fatal(err)
		}
		calls := client.callsFor("sendMessage")
		if len(calls) != 1 || calls[0].Params["text"] != quoteRejectedText {
			t.Fatalf("wrong refusal: %+v", calls)
		}
		eph, _ := calls[0].Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
		if eph == nil || eph.ReceiverUserId != 100 {
			t.Fatal("public refusal")
		}
		if len(client.callsFor("getUserProfilePhotos")) != 0 || len(client.callsFor("sendPhoto")) != 0 {
			t.Fatal("rejected quote triggered avatar/rendering")
		}
	}
	// Only the selected fragment is checked; unrelated source text does not block it.
	ctx := testReplyContext(bot, "/quote", testUser(100), source)
	ctx.Message.Quote = &gotgbot.TextQuote{Text: "Не сдавайся.", IsManual: true}
	q, ok := quoteSelectedContent(ctx.Message)
	if !ok || quoteForbidden(q.Text) {
		t.Fatal("clean selection rejected because of other source text")
	}
}

func TestQuoteShortTextGetsLargerType(t *testing.T) {
	if quoteTextSize("Смысл жизни") <= quoteTextSize(strings.Repeat("Смысл жизни продолжается. ", 12)) {
		t.Fatal("short quotation type not enlarged")
	}
}

func TestQuoteEllipses(t *testing.T) {
	source := "Боже как я люблю величие поп в никее!"
	for _, tc := range []struct{ selection, want string }{
		{source, source},
		{"я люблю величие поп", "... я люблю величие поп ..."},
		{"Боже как", "Боже как ..."},
		{"в никее!", "... в никее!"},
	} {
		if got := quoteDisplayText(source, tc.selection, nil); got != tc.want {
			t.Fatalf("%q: %q != %q", tc.selection, got, tc.want)
		}
	}
	if got := quoteDisplayText("слово слово", "слово", &gotgbot.TextQuote{Position: 6}); got != "... слово" {
		t.Fatal(got)
	}
}
