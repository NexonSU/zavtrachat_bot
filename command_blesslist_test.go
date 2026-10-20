package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func setupBlessListTest(t *testing.T, texts ...string) (*gotgbot.Bot, *fakeBotClient) {
	t.Helper()
	bot, client := setupTestEnv(t)
	blessListViews.Lock()
	old := blessListViews.views
	blessListViews.views = make(map[string]blessListView)
	blessListViews.Unlock()
	t.Cleanup(func() {
		blessListViews.Lock()
		blessListViews.views = old
		blessListViews.Unlock()
	})
	if err := DB.Where("1 = 1").Delete(&Bless{}).Error; err != nil {
		t.Fatal(err)
	}
	for _, text := range texts {
		if err := DB.Create(&Bless{Text: text}).Error; err != nil {
			t.Fatal(err)
		}
	}
	client.setResult("editMessageText", json.RawMessage(sentMessageResult))
	return bot, client
}

func blessPrivateContext(bot *gotgbot.Bot, text string, id int64) *ext.Context {
	msg := testMessage(text, testUser(id))
	msg.Chat = gotgbot.Chat{Id: id, Type: "private"}
	return ext.NewContext(bot, &gotgbot.Update{Message: msg}, nil)
}

func blessLastView(t *testing.T, client *fakeBotClient) fakeCall {
	t.Helper()
	client.mu.Lock()
	defer client.mu.Unlock()
	for i := len(client.Calls) - 1; i >= 0; i-- {
		if client.Calls[i].Method == "sendMessage" || client.Calls[i].Method == "editMessageText" {
			return client.Calls[i]
		}
	}
	t.Fatal("no list view was sent")
	return fakeCall{}
}

func blessButton(t *testing.T, client *fakeBotClient, label string) string {
	t.Helper()
	markup, ok := blessLastView(t, client).Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if !ok {
		t.Fatal("no inline keyboard")
	}
	for _, row := range markup.InlineKeyboard {
		for _, button := range row {
			if len(button.CallbackData) > 64 {
				t.Fatalf("callback exceeds Telegram's limit: %q", button.CallbackData)
			}
			if button.Text == label {
				return button.CallbackData
			}
		}
	}
	t.Fatalf("button %q not found", label)
	return ""
}

func blessClick(bot *gotgbot.Bot, data string, userID, chatID, messageID int64, chatType string) error {
	msg := testMessage("list", &bot.User)
	msg.MessageId = messageID
	msg.Chat = gotgbot.Chat{Id: chatID, Type: chatType}
	ctx := testCallbackContext(bot, msg, testUser(userID))
	ctx.CallbackQuery.Data = data
	return BlessListCallback(bot, ctx)
}

func blessCount(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := DB.Model(&Bless{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestBlessListAccess(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   int64
		pm   bool
		want string
	}{
		{"admin", 100, true, "Фразы блесса"},
		{"moderator", 300, true, "Фразы блесса"},
		{"member", 42, true, "только администраторам и модераторам"},
		{"group admin", 100, false, "в личке с ботом"},
		{"group member", 42, false, "в личке с ботом"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bot, client := setupBlessListTest(t, "секретная фраза")
			ctx := blessPrivateContext(bot, "/blesslist@testbot", tc.id)
			if !tc.pm {
				ctx = testContext(bot, "/blesslist@testbot", testUser(tc.id))
			}
			found := false
			for _, entry := range commandListConfig() {
				if entry.command.Command == "blesslist" {
					found = true
					if entry.command.IsEphemeral {
						t.Fatal("private command must not be ephemeral")
					}
					if err := entry.response(bot, ctx); err != nil {
						t.Fatal(err)
					}
				}
			}
			if !found {
				t.Fatal("/blesslist is not registered")
			}
			text := blessLastView(t, client).Params["text"].(string)
			if !strings.Contains(text, tc.want) {
				t.Fatalf("response = %q", text)
			}
			if (!tc.pm || tc.id == 42) && strings.Contains(text, "секретная фраза") {
				t.Fatal("phrase leaked outside authorized PM")
			}
			if len(client.callsFor("getChatMember")) != 0 || blessCount(t) != 1 {
				t.Fatal("listing must use existing configured roles and not change the pool")
			}
		})
	}
}

func TestBlessListPaginationAndCancel(t *testing.T) {
	var texts []string
	for i := 0; i < 12; i++ {
		texts = append(texts, fmt.Sprintf("фраза %02d", i))
	}
	bot, client := setupBlessListTest(t, texts...)
	if err := BlessList(bot, blessPrivateContext(bot, "/blesslist", 100)); err != nil {
		t.Fatal(err)
	}
	for _, page := range []struct{ button, want string }{
		{"Вперёд →", "Страница 2 из 3"},
		{"Вперёд →", "Страница 3 из 3"},
		{"← Назад", "Страница 2 из 3"},
		{"Удалить №6", "Удалить эту фразу"},
		{"Отмена", "Страница 2 из 3"},
	} {
		if err := blessClick(bot, blessButton(t, client, page.button), 100, 100, 555, "private"); err != nil {
			t.Fatal(err)
		}
		if text := blessLastView(t, client).Params["text"].(string); !strings.Contains(text, page.want) {
			t.Fatalf("after %q: %q", page.button, text)
		}
	}
	if blessCount(t) != 12 {
		t.Fatal("browsing and cancelling deleted phrases")
	}
}

func TestBlessListDeleteExactText(t *testing.T) {
	text := "<b>кот & ' OR 1=1; -- 🐈</b>"
	bot, client := setupBlessListTest(t, text, "другая", "третья")
	DB.Create(&Stats{ContextID: 42, StatType: 8, DayTimestamp: 1, Count: 7})
	if err := BlessList(bot, blessPrivateContext(bot, "/blesslist", 100)); err != nil {
		t.Fatal(err)
	}
	if shown := blessLastView(t, client).Params["text"].(string); !strings.Contains(shown, html.EscapeString(text)) {
		t.Fatalf("phrase HTML was not escaped: %q", shown)
	}
	if err := blessClick(bot, blessButton(t, client, "Удалить №1"), 100, 100, 555, "private"); err != nil {
		t.Fatal(err)
	}
	confirm := blessButton(t, client, "Да, удалить")
	if blessCount(t) != 3 {
		t.Fatal("selection deleted before confirmation")
	}
	// Another change shifts every page number. Confirmation must still identify
	// exactly the text the user saw, rather than an offset in the current pool.
	DB.Create(&Bless{Text: "!новая первая фраза"})
	if err := blessClick(bot, confirm, 100, 100, 555, "private"); err != nil {
		t.Fatal(err)
	}
	if blessCount(t) != 3 {
		t.Fatal("wrong number of phrases deleted")
	}
	var n int64
	DB.Model(&Bless{}).Where("text = ?", text).Count(&n)
	if n != 0 {
		t.Fatal("selected exact text is still in the pool")
	}
	receipt := blessLastView(t, client).Params["text"].(string)
	if !strings.Contains(receipt, "Фраза удалена.") || !strings.Contains(receipt, html.EscapeString(text)) || !strings.Contains(receipt, "/addbless") {
		t.Fatalf("missing recovery copy: %q", receipt)
	}
	var stat Stats
	DB.First(&stat)
	if stat.Count != 7 || GetBless() == text {
		t.Fatal("statistics changed or shared GetBless still returns the deleted text")
	}
	// Restore using the original /addbless handler, then replay the old confirm.
	if err := AddBless(bot, blessPrivateContext(bot, "/addbless "+text, 100)); err != nil {
		t.Fatal(err)
	}
	if err := blessClick(bot, confirm, 100, 100, 555, "private"); err != nil {
		t.Fatal(err)
	}
	DB.Model(&Bless{}).Where("text = ?", text).Count(&n)
	if n != 1 || blessCount(t) != 4 {
		t.Fatal("replaying old confirmation deleted a restored phrase")
	}
}

func TestBlessListCallbackAccess(t *testing.T) {
	for _, tc := range []struct {
		name                string
		user, chat, message int64
		chatType            string
		revoke              bool
	}{
		{"member", 42, 42, 555, "private", false},
		{"another moderator", 300, 300, 555, "private", false},
		{"wrong message", 100, 100, 556, "private", false},
		{"group", 100, -100, 555, "group", false},
		{"wrong private recipient", 100, 300, 555, "private", false},
		{"revoked admin", 100, 100, 555, "private", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bot, client := setupBlessListTest(t, "а", "б")
			BlessList(bot, blessPrivateContext(bot, "/blesslist", 100))
			blessClick(bot, blessButton(t, client, "Удалить №1"), 100, 100, 555, "private")
			confirm := blessButton(t, client, "Да, удалить")
			if tc.revoke {
				Config.Admins = nil
			}
			client.clearCalls()
			if err := blessClick(bot, confirm, tc.user, tc.chat, tc.message, tc.chatType); err != nil {
				t.Fatal(err)
			}
			if blessCount(t) != 2 || len(client.callsFor("editMessageText")) != 0 || len(client.callsFor("sendMessage")) != 0 {
				t.Fatal("unauthorized callback changed or disclosed the pool")
			}
			if len(client.callsFor("answerCallbackQuery")) != 1 {
				t.Fatal("unauthorized callback was not answered")
			}
		})
	}
}

func TestBlessListLastMissingAndExpired(t *testing.T) {
	for _, mode := range []string{"last", "missing", "expired", "forged confirm"} {
		t.Run(mode, func(t *testing.T) {
			bot, client := setupBlessListTest(t, "а", "б")
			BlessList(bot, blessPrivateContext(bot, "/blesslist", 100))
			button := blessButton(t, client, "Удалить №1")
			if mode == "forged confirm" {
				button = strings.Split(button, ":ask:")[0] + ":confirm"
			} else {
				blessClick(bot, button, 100, 100, 555, "private")
				button = blessButton(t, client, "Да, удалить")
			}
			switch mode {
			case "last":
				DB.Where("text = ?", "б").Delete(&Bless{})
			case "missing":
				DB.Where("text = ?", "а").Delete(&Bless{})
			case "expired":
				blessListViews.Lock()
				for key, view := range blessListViews.views {
					view.expires = time.Now().Add(-time.Second)
					blessListViews.views[key] = view
				}
				blessListViews.Unlock()
			}
			before := blessCount(t)
			blessClick(bot, button, 100, 100, 555, "private")
			if blessCount(t) != before {
				t.Fatal("invalid confirmation changed the pool")
			}
			if mode == "last" && !strings.Contains(blessLastView(t, client).Params["text"].(string), "Последнюю фразу") {
				t.Fatal("last phrase protection was not explained")
			}
		})
	}
}

func TestBlessListLongAndEmptyPools(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		bot, client := setupBlessListTest(t)
		if err := BlessList(bot, blessPrivateContext(bot, "/blesslist", 100)); err != nil {
			t.Fatal(err)
		}
		if text := blessLastView(t, client).Params["text"].(string); !strings.Contains(text, "Фраз пока нет") {
			t.Fatalf("empty pool = %q", text)
		}
		markup := blessLastView(t, client).Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
		if markup.InlineKeyboard == nil {
			t.Fatal("empty keyboard must encode as an array, not null")
		}
	})
	t.Run("long legacy text", func(t *testing.T) {
		text := strings.Repeat("🐈<&>", 500)
		bot, client := setupBlessListTest(t, text, "я")
		var copies []string
		client.Observe = func(method string, params map[string]any) {
			if method == "sendDocument" {
				file := params["document"].(*gotgbot.FileReader)
				data, _ := io.ReadAll(file.Data)
				copies = append(copies, string(data))
			}
		}
		BlessList(bot, blessPrivateContext(bot, "/blesslist", 100))
		// Cyrillic sorts before emoji in the test DB.
		blessClick(bot, blessButton(t, client, "Удалить №2"), 100, 100, 555, "private")
		if len(copies) != 1 || copies[0] != text {
			t.Fatal("confirmation did not deliver the full legacy text")
		}
		blessClick(bot, blessButton(t, client, "Да, удалить"), 100, 100, 555, "private")
		if len(copies) != 2 || copies[1] != text || blessCount(t) != 1 {
			t.Fatal("deletion did not preserve an exact recovery copy")
		}
	})
}

func TestBlessListFailures(t *testing.T) {
	t.Run("send failure", func(t *testing.T) {
		bot, client := setupBlessListTest(t, "а", "б")
		client.setError("sendMessage", errors.New("blocked"))
		if err := BlessList(bot, blessPrivateContext(bot, "/blesslist", 100)); err == nil {
			t.Fatal("send failure was lost")
		}
		if len(blessListViews.views) != 0 || blessCount(t) != 2 {
			t.Fatal("failed send retained active buttons or changed data")
		}
	})
	t.Run("confirmation edit failure", func(t *testing.T) {
		bot, client := setupBlessListTest(t, "а", "б")
		BlessList(bot, blessPrivateContext(bot, "/blesslist", 100))
		button := blessButton(t, client, "Удалить №1")
		client.setError("editMessageText", errors.New("message deleted"))
		if err := blessClick(bot, button, 100, 100, 555, "private"); err != nil {
			t.Fatal(err)
		}
		if blessCount(t) != 2 || !strings.Contains(blessLastView(t, client).Params["text"].(string), "Не удалось") {
			t.Fatal("failed confirmation must leave data untouched and explain recovery")
		}
	})
	t.Run("database failure", func(t *testing.T) {
		bot, client := setupBlessListTest(t, "а", "б")
		BlessList(bot, blessPrivateContext(bot, "/blesslist", 100))
		blessClick(bot, blessButton(t, client, "Удалить №1"), 100, 100, 555, "private")
		button := blessButton(t, client, "Да, удалить")
		DB.Migrator().DropTable(&Bless{})
		if err := blessClick(bot, button, 100, 100, 555, "private"); err != nil {
			t.Fatal("callback errors must not reach the message-only error handler")
		}
		if !strings.Contains(blessLastView(t, client).Params["text"].(string), "Не удалось") {
			t.Fatal("DB failure was not reported")
		}
	})
}

func TestBlessListConcurrentConfirmation(t *testing.T) {
	bot, client := setupBlessListTest(t, "а", "б", "в")
	BlessList(bot, blessPrivateContext(bot, "/blesslist", 100))
	blessClick(bot, blessButton(t, client, "Удалить №1"), 100, 100, 555, "private")
	button := blessButton(t, client, "Да, удалить")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			blessClick(bot, button, 100, 100, 555, "private")
		}()
	}
	wg.Wait()
	if blessCount(t) != 2 {
		t.Fatal("concurrent confirmation did not delete exactly once")
	}
}

func TestBlessListConcurrentLastPhrases(t *testing.T) {
	setupBlessListTest(t, "а", "б")
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan string, 2)
	for _, text := range []string{"а", "б"} {
		wg.Add(1)
		go func(text string) {
			defer wg.Done()
			<-start
			status, err := deleteBlessListText(text)
			if err == nil {
				results <- status
			}
		}(text)
	}
	close(start)
	wg.Wait()
	close(results)
	deleted := 0
	for status := range results {
		if status == "Фраза удалена." {
			deleted++
		}
	}
	// SQLite can reject one or both overlapping writers with SQLITE_BUSY;
	// either way it must roll back rather than leave the shared pool empty.
	if blessCount(t) < 1 || deleted > 1 {
		t.Fatal("overlapping deletions emptied the pool")
	}
}

func TestBlessListViewBounded(t *testing.T) {
	setupBlessListTest(t, "а")
	for i := 0; i < blessListSessionLimit+20; i++ {
		rememberBlessListView(fmt.Sprint(i), blessListView{ownerID: 100, messageID: int64(i + 1)})
	}
	if len(blessListViews.views) != blessListSessionLimit {
		t.Fatal("keyboard cache is not bounded")
	}
}
