package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"gorm.io/gorm"
)

func TestAllGetMediaRoundTrips(t *testing.T) {
	for _, tc := range []struct{ kind, method, parameter string }{
		{"Animation", "sendAnimation", "animation"}, {"Audio", "sendAudio", "audio"}, {"Photo", "sendPhoto", "photo"},
		{"Video", "sendVideo", "video"}, {"Voice", "sendVoice", "voice"}, {"Document", "sendDocument", "document"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			bot, client := setupTestEnv(t)
			msg := testMessage("", testUser(42))
			msg.Caption = "Первое слово & хвост"
			msg.CaptionEntities = []gotgbot.MessageEntity{{Type: "bold", Offset: 0, Length: 6}}
			switch tc.kind {
			case "Animation":
				msg.Animation = &gotgbot.Animation{FileId: "file-123"}
			case "Audio":
				msg.Audio = &gotgbot.Audio{FileId: "file-123"}
			case "Photo":
				msg.Photo = []gotgbot.PhotoSize{{FileId: "file-123"}}
			case "Video":
				msg.Video = &gotgbot.Video{FileId: "file-123"}
			case "Voice":
				msg.Voice = &gotgbot.Voice{FileId: "file-123"}
			case "Document":
				msg.Document = &gotgbot.Document{FileId: "file-123"}
			}
			if err := Set(bot, testReplyContext(bot, "/set media", testUser(42), msg)); err != nil {
				t.Fatal(err)
			}
			var row Get
			DB.First(&row, "name = ?", "media")
			caption := "<b>Первое</b> слово &amp; хвост"
			if row.Caption != caption || row.Type != tc.kind || row.Data != "file-123" {
				t.Fatalf("stored media changed: %+v", row)
			}
			if err := GetGet(bot, testContext(bot, "/get MEDIA", testUser(99))); err != nil {
				t.Fatal(err)
			}
			calls := client.callsFor(tc.method)
			if len(calls) != 1 {
				t.Fatalf("%s calls=%d", tc.method, len(calls))
			}
			params := calls[0].Params
			reply, _ := params["reply_parameters"].(*gotgbot.ReplyParameters)
			if paramFileID(t, params[tc.parameter]) != "file-123" || params["caption"] != caption || params["parse_mode"] != "HTML" || reply == nil || reply.MessageId != 100 {
				t.Fatalf("wrong media delivery: %+v", params)
			}
			failure := errors.New("media send failed")
			client.setError(tc.method, failure)
			if err := GetGet(bot, testContext(bot, "/get media", testUser(99))); !errors.Is(err, failure) {
				t.Fatal(err)
			}
			if len(client.callsFor(tc.method)) != 2 {
				t.Fatal("send retried unexpectedly")
			}
			var after Get
			DB.First(&after, "name = ?", "media")
			if after.Caption != caption || after.Data != row.Data {
				t.Fatal("failed delivery changed stored get")
			}
		})
	}
}

func TestGetOwnershipThroughRegisteredCommands(t *testing.T) {
	bot, client := setupTestEnv(t)
	seedUser(42, "owner", "Владелец")
	DB.Create(&Get{Name: "locked", Creator: 42, Type: "Text", Data: "<b>Original</b>"})
	dispatcher := newBotDispatcher()
	registerBotHandlers(dispatcher)
	for _, command := range []string{"/set locked changed", "/del locked"} {
		processTestUpdate(t, dispatcher, bot, &gotgbot.Update{Message: testMessage(command, testUser(43))})
		var row Get
		if err := DB.First(&row, "name = ?", "locked").Error; err != nil || row.Data != "<b>Original</b>" || row.Creator != 42 {
			t.Fatalf("other user changed get: %+v %v", row, err)
		}
	}
	if !strings.Contains(strings.Join(client.sentTexts(), ""), "могут изменять") {
		t.Fatal("missing ownership denial")
	}
	processTestUpdate(t, dispatcher, bot, &gotgbot.Update{Message: testMessage("/set locked <changed> & text", testUser(42))})
	var row Get
	DB.First(&row, "name = ?", "locked")
	if row.Data != "&lt;changed&gt; &amp; text" {
		t.Fatalf("text escaping lost: %q", row.Data)
	}
	processTestUpdate(t, dispatcher, bot, &gotgbot.Update{Message: testMessage("/del locked", testUser(100))})
	var count int64
	DB.Model(&Get{}).Count(&count)
	if count != 0 {
		t.Fatal("admin cannot delete get")
	}
}

func TestGetMalformedDataAndReadFailure(t *testing.T) {
	bot, client := setupTestEnv(t)
	emptyPhoto := testMessage("", testUser(42))
	emptyPhoto.Photo = []gotgbot.PhotoSize{}
	if err := Set(bot, testReplyContext(bot, "/set empty", testUser(42), emptyPhoto)); err != nil {
		t.Fatal(err)
	}
	var count int64
	DB.Model(&Get{}).Count(&count)
	if count != 0 {
		t.Fatal("empty photo saved")
	}
	client.clearCalls()
	if err := GetGet(bot, testContext(bot, "/get <bad>&", testUser(42))); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(client.sentTexts(), ""); !strings.Contains(got, "&lt;bad&gt;&amp;") || strings.Contains(got, "<bad>") {
		t.Fatalf("unsafe missing-get name: %q", got)
	}
	failure := errors.New("get read failed")
	if err := DB.Callback().Query().Before("gorm:query").Register("test:fail_get_read", func(tx *gorm.DB) {
		if tx.Statement.Table == "gets" {
			tx.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { DB.Callback().Query().Remove("test:fail_get_read") })
	client.clearCalls()
	if err := GetGet(bot, testContext(bot, "/get known", testUser(42))); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if len(client.sentTexts()) != 0 {
		t.Fatal("database failure misreported as missing get")
	}
}
