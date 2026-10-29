package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/fogleman/gg"
)

func TestQuoteAttribution(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  gotgbot.Message
		want quoteContent
	}{
		{"user", gotgbot.Message{Text: "Текст", Date: 100, From: &gotgbot.User{Id: 7, FirstName: "Иван", LastName: "Петров"}}, quoteContent{Text: "Текст", Date: 100, Author: "Иван Петров", UserID: 7}},
		{"caption", gotgbot.Message{Caption: "Подпись", Date: 101, From: &gotgbot.User{Id: 7, FirstName: "Иван"}}, quoteContent{Text: "Подпись", Date: 101, Author: "Иван", UserID: 7}},
		{"anonymous", gotgbot.Message{Text: "Текст", Date: 100, From: &gotgbot.User{Id: 7, FirstName: "Fake"}, SenderChat: &gotgbot.Chat{Id: -2, Title: "Канал"}, AuthorSignature: "Редактор"}, quoteContent{Text: "Текст", Date: 100, Author: "Редактор · Канал", ChatID: -2}},
		{"forward user", gotgbot.Message{Text: "Текст", Date: 200, From: &gotgbot.User{Id: 9, FirstName: "Forwarder"}, ForwardOrigin: gotgbot.MessageOriginUser{Date: 100, SenderUser: gotgbot.User{Id: 7, FirstName: "Иван"}}}, quoteContent{Text: "Текст", Date: 100, Author: "Иван", UserID: 7}},
		{"hidden", gotgbot.Message{Text: "Текст", Date: 200, From: &gotgbot.User{Id: 9, FirstName: "Forwarder"}, ForwardOrigin: gotgbot.MessageOriginHiddenUser{Date: 100, SenderUserName: "Скрытый автор"}}, quoteContent{Text: "Текст", Date: 100, Author: "Скрытый автор"}},
		{"forward channel", gotgbot.Message{Text: "Текст", Date: 200, ForwardOrigin: gotgbot.MessageOriginChannel{Date: 100, Chat: gotgbot.Chat{Id: -2, Title: "Канал"}, AuthorSignature: "Редактор"}}, quoteContent{Text: "Текст", Date: 100, Author: "Редактор · Канал", ChatID: -2}},
		{"unknown", gotgbot.Message{Text: "Текст"}, quoteContent{Text: "Текст", Author: "Неизвестный автор"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := quoteFromMessage(&tc.msg); got != tc.want {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
		})
	}
	if got := quoteDate(time.Date(2026, 10, 29, 11, 37, 0, 0, time.UTC).Unix()); got != "29.10.2026 · 14:37 МСК" {
		t.Fatal(got)
	}
	if quoteDate(0) != "Дата неизвестна" {
		t.Fatal("fabricated date")
	}
}

func TestQuoteUsageAndValidation(t *testing.T) {
	bot, client := setupTestEnv(t)
	if err := Quote(bot, &ext.Context{}); err != nil {
		t.Fatal(err)
	}
	for _, reply := range []*gotgbot.Message{nil, {}, {Text: strings.Repeat("а", 4097)}} {
		client.clearCalls()
		ctx := testContext(bot, "/quote", testUser(100))
		ctx.Message.ReplyToMessage = reply
		if err := Quote(bot, ctx); err != nil {
			t.Fatal(err)
		}
		if len(client.callsFor("sendPhoto")) != 0 || len(client.callsFor("getUserProfilePhotos")) != 0 {
			t.Fatal("invalid input triggered work")
		}
		calls := client.callsFor("sendMessage")
		if len(calls) != 1 {
			t.Fatal("missing usage")
		}
		eph, _ := calls[0].Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
		if eph == nil || eph.ReceiverUserId != 100 {
			t.Fatal("usage is public")
		}
	}
}

func quoteTestAvatar() image.Image {
	dc := gg.NewContext(160, 160)
	dc.SetHexColor("#FFBF84")
	dc.Clear()
	dc.SetHexColor("#442C54")
	dc.DrawCircle(80, 67, 34)
	dc.Fill()
	dc.DrawEllipse(80, 158, 64, 58)
	dc.Fill()
	dc.SetHexColor("#FFDDAD")
	dc.DrawCircle(69, 66, 3)
	dc.DrawCircle(91, 66, 3)
	dc.Fill()
	return dc.Image()
}

func TestQuoteSendPhotoAndAvatarFallback(t *testing.T) {
	bot, client := setupTestEnv(t)
	dir := t.TempDir()
	avatarPath := filepath.Join(dir, "avatar.png")
	file, err := os.Create(avatarPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, quoteTestAvatar()); err != nil {
		t.Fatal(err)
	}
	file.Close()
	client.setResult("getUserProfilePhotos", json.RawMessage(`{"total_count":1,"photos":[[{"file_id":"avatar1","width":160,"height":160}]]}`))
	fakeMediaFile(t, client, avatarPath)
	var sent []byte
	client.Observe = func(method string, p map[string]any) {
		if method == "sendPhoto" {
			if f, ok := p["photo"].(*gotgbot.FileReader); ok {
				sent, _ = io.ReadAll(f.Data)
			}
		}
	}
	reply := &gotgbot.Message{MessageId: 321, Text: "Сначала сделаем красиво.\nА потом ещё красивее. 😎", Date: time.Date(2026, 10, 29, 11, 37, 0, 0, time.UTC).Unix(), From: &gotgbot.User{Id: 77, FirstName: "Автор", LastName: "сообщения"}}
	ctx := testReplyContext(bot, "/quote ПОДМЕНА", testUser(100), reply)
	if err := Quote(bot, ctx); err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(sent))
	if err != nil || cfg.Width != 1200 || cfg.Height < 660 {
		t.Fatalf("invalid card: %+v %v", cfg, err)
	}
	imageCard, err := png.Decode(bytes.NewReader(sent))
	if err != nil {
		t.Fatal(err)
	}
	// The author/date must remain visible outside the circular avatar clip.
	for _, band := range []struct {
		name                   string
		top, bottom, threshold int
	}{{"author", 462, 490, 210}, {"date", 496, 524, 145}} {
		pixels := 0
		for y := band.top; y < band.bottom; y++ {
			for x := 232; x < 850; x++ {
				r, g, b, _ := imageCard.At(x, y).RGBA()
				if int(r>>8) > band.threshold && int(g>>8) > band.threshold && int(b>>8) > band.threshold {
					pixels++
				}
			}
		}
		if pixels < 30 {
			t.Fatalf("%s not visible outside avatar clip", band.name)
		}
	}
	calls := client.callsFor("sendPhoto")
	if len(calls) != 1 {
		t.Fatal("missing photo")
	}
	eph, _ := calls[0].Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
	if eph == nil || eph.ReceiverUserId != 100 {
		t.Fatalf("quote must reach only invoking admin: %+v", calls[0].Params)
	}
	params, _ := calls[0].Params["reply_parameters"].(*gotgbot.ReplyParameters)
	if params == nil || params.MessageId != 321 {
		t.Fatal("not replying to source")
	}
	if len(client.callsFor("getUserProfilePhotos")) != 1 {
		t.Fatal("avatar not fetched")
	}
	if output := os.Getenv("ZAVTRA_QUOTE_PREVIEW_DIR"); output != "" {
		if err := os.MkdirAll(output, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(output, "quote-preview.png"), sent, 0644); err != nil {
			t.Fatal(err)
		}
	}
	client.setError("getUserProfilePhotos", errors.New("avatar private"))
	client.clearCalls()
	if err := Quote(bot, ctx); err != nil {
		t.Fatal(err)
	}
	if len(client.callsFor("sendPhoto")) != 1 {
		t.Fatal("unavailable avatar prevented quote")
	}
	client.setError("sendPhoto", errors.New("cannot send"))
	if err := Quote(bot, ctx); err == nil {
		t.Fatal("upload error hidden")
	}
}

func TestQuoteAvatarHTTPAndBounds(t *testing.T) {
	bot, client := setupTestEnv(t)
	client.setResult("getUserProfilePhotos", json.RawMessage(`{"total_count":1,"photos":[[{"file_id":"avatar1","width":160,"height":160}]]}`))
	client.setResult("getFile", json.RawMessage(`{"file_path":"quote-test/avatar.png"}`))
	stubProxy(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.RequestURI, "/quote-test/avatar.png") {
			png.Encode(w, quoteTestAvatar())
			return
		}
		http.NotFound(w, r)
	})
	if img := quoteAvatar(context.Background(), bot, quoteContent{UserID: 77}); img == nil || img.Bounds().Dx() != 160 {
		t.Fatal("HTTP avatar missing")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if img := quoteAvatar(ctx, bot, quoteContent{UserID: 77}); img != nil {
		t.Fatal("cancelled avatar succeeded")
	}
	for _, data := range [][]byte{[]byte("not image"), bytes.Repeat([]byte{1}, quoteAvatarMaxBytes+1)} {
		if decodeQuoteAvatar(bytes.NewReader(data)) != nil {
			t.Fatal("invalid avatar accepted")
		}
	}
	var oversized bytes.Buffer
	png.Encode(&oversized, image.NewRGBA(image.Rect(0, 0, 2049, 1)))
	if decodeQuoteAvatar(&oversized) != nil {
		t.Fatal("oversized dimensions accepted")
	}
	client.setResult("getChat", json.RawMessage(`{"id":-2,"type":"channel","title":"Канал","photo":{"small_file_id":"avatar1","big_file_id":"avatar2"}}`))
	if img := quoteAvatar(context.Background(), bot, quoteContent{ChatID: -2}); img == nil {
		t.Fatal("channel avatar missing")
	}
	client.setResult("getUserProfilePhotos", json.RawMessage(`{"total_count":0,"photos":[]}`))
	if quoteAvatar(context.Background(), bot, quoteContent{UserID: 77}) != nil {
		t.Fatal("missing profile photo")
	}
}

func TestQuoteTextLayoutAndFonts(t *testing.T) {
	fonts, err := loadQuoteFonts()
	if err != nil {
		t.Fatal(err)
	}
	face, err := fonts.newFace(42, false)
	if err != nil {
		t.Fatal(err)
	}
	defer face.Close()
	for _, r := range "Русский Ёё English 😎🔥😂❤" {
		if _, ok := face.GlyphAdvance(r); !ok {
			t.Fatalf("missing glyph %U", r)
		}
	}
	dc := gg.NewContext(1, 1)
	dc.SetFontFace(face)
	text := "Точная цитата <теги> & символы\n\n" + strings.Repeat("оченьдлинноеслово", 20)
	lines := quoteWrap(dc, text, 1008)
	for _, line := range lines {
		w, _ := dc.MeasureString(line)
		if w > 1008 {
			t.Fatalf("line overflows: %f", w)
		}
	}
	if strings.Join(strings.Fields(strings.Join(lines, " ")), "") != strings.Join(strings.Fields(text), "") {
		t.Fatal("text lost in wrapping")
	}
	if len(lines) < 3 || lines[1] != "" {
		t.Fatal("paragraph break lost")
	}
	if _, err := renderQuote(quoteContent{Text: strings.Repeat("\n", 150), Author: "Иван"}, nil); err == nil {
		t.Fatal("unbounded canvas")
	}
	for _, n := range []int{0, 200, 500, 1100, 4096} {
		data, err := renderQuote(quoteContent{Text: strings.Repeat("я", n), Author: "Длинное имя автора сообщения", Date: 10}, nil)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Height > 6000 || cfg.Width != 1200 {
			t.Fatalf("layout %d: %+v %v", n, cfg, err)
		}
	}
	if quoteInitials("Иван Петров") != "ИП" || quoteInitials("😎") != "?" {
		t.Fatal("initials")
	}
}

func TestQuoteConcurrentRender(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Go(func() {
			data, err := renderQuote(quoteContent{Text: "Параллельные цитаты 😎", Author: "Тест"}, quoteTestAvatar())
			if err != nil || len(data) == 0 {
				t.Error(fmt.Sprint(err))
			}
		})
	}
	wg.Wait()
}

func TestQuoteQueueAndRegistration(t *testing.T) {
	bot, client := setupTestEnv(t)
	found := false
	for _, entry := range commandListConfig() {
		if entry.command.Command == "quote" {
			found = true
		}
	}
	if !found || !strings.Contains(zbAdminHelpText, "/quote") {
		t.Fatal("quote not registered or documented")
	}
	old := mediaTasks
	mediaTasks = newMediaTaskQueue(1, 0, time.Second, time.Minute)
	t.Cleanup(func() { mediaTasks = old })
	_, release, err := mediaTasks.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	reply := &gotgbot.Message{Text: "Текст", From: &gotgbot.User{Id: 77, FirstName: "Иван"}}
	if err := Quote(bot, testReplyContext(bot, "/quote", testUser(100), reply)); err != nil {
		t.Fatal(err)
	}
	if len(client.callsFor("sendPhoto")) != 0 || len(client.callsFor("getUserProfilePhotos")) != 0 {
		t.Fatal("full queue started work")
	}
	if text := strings.Join(client.sentTexts(), " "); !strings.Contains(text, "очередь") {
		t.Fatal(text)
	}
}
