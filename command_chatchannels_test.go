package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func setupChatChannelTest(t *testing.T) (*gotgbot.Bot, *fakeBotClient) {
	t.Helper()
	bot, client := setupTestEnv(t)
	if err := DB.AutoMigrate(&ChatChannelOwner{}, &ChatChannel{}, &ChatChannelImport{}); err != nil {
		t.Fatal(err)
	}
	return bot, client
}

func TestChatChannelsCommands(t *testing.T) {
	bot, client := setupChatChannelTest(t)
	dispatcher := ext.NewDispatcher(nil)
	registerBotHandlers(dispatcher)

	alice := &gotgbot.User{Id: 42, FirstName: "Алиса", Username: "alice"}
	bob := &gotgbot.User{Id: 54, FirstName: "Bob", Username: "bob"}
	changedAlice := *alice
	changedAlice.Username = "newalice"
	reusedAlice := *alice
	reusedAlice.Id = 55
	admin := testUser(200)
	client.setMemberStatus(admin.Id, "administrator")
	for _, tc := range []struct {
		user    *gotgbot.User
		command string
		want    string
	}{
		{alice, "/addchatchannel @bob https://t.me/AliceNews", "свой текущий ник"},
		{alice, "/addchatchannel@testbot @ALICE t.me/AliceNews/", "Канал добавлен"},
		{bob, "/addchatchannel bob http://t.me/alicenews", "уже есть"},
		{alice, "/addchatchannel @alice https://t.me/joinchat/Case_123", "Канал добавлен"},
		{alice, "/chatchannels@testbot", "Алиса (alice):\nhttps://t.me/AliceNews\nhttps://t.me/+Case_123"},
		{alice, "/delmychatchannel", "У тебя несколько каналов"},
		{bob, "/delmychatchannel https://t.me/AliceNews", "Твой канал не найден"},
		{&reusedAlice, "/delmychatchannel https://t.me/AliceNews", "Твой канал не найден"},
		{bob, "/delchatchannel https://t.me/AliceNews", "только администраторы"},
		{testUser(300), "/delchatchannel https://t.me/AliceNews", "только администраторы"},
		{&changedAlice, "/delmychatchannel https://t.me/ALICENEWS", "Канал удалён"},
		{admin, "/delchatchannel https://t.me/+case_123", "Канал не найден"},
		{admin, "/delchatchannel https://t.me/+Case_123", "Канал удалён"},
		{bob, "/addchatchannel @bob https://t.me/BobsChannel", "Канал добавлен"},
		{bob, "/delmychatchannel", "Канал удалён"},
		{bob, "/chatchannels", "В каталоге пока нет каналов"},
	} {
		client.clearCalls()
		msg := testMessage(tc.command, tc.user)
		msg.MessageId, msg.EphemeralMessageId = 0, 777
		if err := dispatcher.ProcessUpdate(bot, &gotgbot.Update{Message: msg}, nil); err != nil {
			t.Fatal(err)
		}
		calls := client.callsFor("sendMessage")
		if len(calls) != 1 || calls[0].Params["chat_id"] != Config.Chat {
			t.Fatalf("%s: got %d responses or wrong destination", tc.command, len(calls))
		}
		params := calls[0].Params
		eph, _ := params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
		if eph == nil || eph.ReceiverUserId != tc.user.Id {
			t.Fatalf("%s: response must only reach the caller", tc.command)
		}
		preview, _ := params["link_preview_options"].(*gotgbot.LinkPreviewOptions)
		if preview == nil || !preview.IsDisabled || !strings.Contains(params["text"].(string), tc.want) {
			t.Fatalf("%s: unexpected response %q", tc.command, params["text"])
		}
	}
}

func TestChatChannelsSeed(t *testing.T) {
	bot, client := setupChatChannelTest(t)
	user, unbound := testUser(42), testUser(43)
	user.Username, unbound.Username = "charlie", "delta"
	if err := DB.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	channel, _ := parseChatChannel("https://t.me/CharlieExtra")
	if _, err := addOwnChatChannel(&DB, user, channel); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "seed.json")
	write := func(seed string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(seed), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(`[{"name":"main","pinned":true,"links":["https://t.me/ExampleMain"]},{"name":"invalid","links":["https://example.com"]}]`)
	if err := importChatChannels(&DB, path); err == nil {
		t.Fatal("invalid seed accepted")
	}

	write(`[
		{"name":"Основной канал","pinned":true,"links":["https://t.me/ExampleMain"]},
		{"name":"Charlie","username":"Charlie","pinned":true,"links":["https://t.me/CharlieNews"]},
		{"name":"Delta","username":"Delta","pinned":true,"links":["https://t.me/DeltaNews","https://t.me/+Test_Invite"]},
		{"name":"Zulu","username":"zulu","links":["https://t.me/ZuluNews"]},
		{"name":"Bravo","username":"bravo","links":["https://t.me/BravoNews"]},
		{"name":"Яна","username":"yana","links":["https://t.me/YanaNews"]},
		{"name":"Борис","username":"boris","links":["https://t.me/BorisNews"]},
		{"name":"alice","username":"alice","links":["https://t.me/AliceNews"]}
	]`)
	if err := importChatChannels(&DB, path); err != nil {
		t.Fatal(err)
	}

	list, err := chatChannelList(&DB)
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []string{"Основной канал:", "Charlie (Charlie):", "Delta (Delta):", "alice (alice):",
		"Bravo (bravo):", "Zulu (zulu):", "Борис (boris):", "Яна (yana):"}
	for _, title := range wantOrder {
		i := strings.Index(list, title)
		if i < 0 {
			t.Fatalf("%q missing or out of order in %q", title, list)
		}
		list = list[i+len(title):]
	}
	if err := DelChatChannel(bot, testContext(bot, "/delchatchannel https://t.me/ExampleMain", testUser(100))); err != nil {
		t.Fatal(err)
	}
	if err := DelMyChatChannel(bot, testContext(bot, "/delmychatchannel https://t.me/DeltaNews", unbound)); err != nil {
		t.Fatal(err)
	}
	unbound.Username = "deltanew"
	if err := DelMyChatChannel(bot, testContext(bot, "/delmychatchannel", unbound)); err != nil {
		t.Fatal(err)
	}
	if err := DelMyChatChannel(bot, testContext(bot, "/delmychatchannel https://t.me/CharlieNews", user)); err != nil {
		t.Fatal(err)
	}

	// Restart with the same file must neither restore deleted channels nor remove a pin.
	if err := importChatChannels(&DB, path); err != nil {
		t.Fatal(err)
	}
	client.clearCalls()
	if err := ChatChannels(bot, testContext(bot, "/chatchannels", user)); err != nil {
		t.Fatal(err)
	}
	list = strings.Join(client.sentTexts(), "\n")
	if !strings.HasPrefix(list, "Charlie (Charlie):\nhttps://t.me/CharlieExtra") {
		t.Fatalf("retained pin or already registered link missing: %s", list)
	}
	for _, deleted := range []string{"ExampleMain", "CharlieNews", "DeltaNews", "+Test_Invite"} {
		if strings.Contains(list, deleted) {
			t.Fatalf("%s was restored", deleted)
		}
	}
}

func TestChatChannelsOutsideMainChatAndLongList(t *testing.T) {
	bot, client := setupChatChannelTest(t)
	user := testUser(42)
	for _, chat := range []gotgbot.Chat{{Id: user.Id, Type: "private"}, {Id: -200, Type: "supergroup"}} {
		msg := testMessage("/chatchannels", user)
		msg.Chat = chat
		if err := ChatChannels(bot, ext.NewContext(bot, &gotgbot.Update{Message: msg}, nil)); err != nil {
			t.Fatal(err)
		}
	}
	if len(client.callsFor("sendMessage")) != 0 {
		t.Fatal("catalog responded outside the main chat")
	}

	owner := ChatChannelOwner{Name: "Alice", Username: "alice"}
	if err := DB.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	var links []string
	for i := range 170 {
		link := fmt.Sprintf("https://t.me/LongExampleChannel%d", i)
		links = append(links, link)
		channel, _ := parseChatChannel(link)
		channel.OwnerID = owner.ID
		if err := DB.Create(&channel).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := ChatChannels(bot, testContext(bot, "/chatchannels", user)); err != nil {
		t.Fatal(err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) < 2 {
		t.Fatal("long catalog was not split")
	}
	var texts []string
	for _, call := range calls {
		params := call.Params
		text := params["text"].(string)
		eph, _ := params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
		if eph == nil || eph.ReceiverUserId != user.Id || len(utf16.Encode([]rune(text))) > 4096 {
			t.Fatal("long catalog part is public or too long")
		}
		texts = append(texts, text)
	}
	if got := strings.Split(strings.Join(texts, "\n"), "\n")[1:]; !slices.Equal(got, links) {
		t.Fatal("long catalog contains missing or split links")
	}
}

func TestChatChannelsMenu(t *testing.T) {
	group, private, admin := commandMenus(commandListConfig())
	has := func(commands []gotgbot.BotCommand, name string) bool {
		return slices.ContainsFunc(commands, func(command gotgbot.BotCommand) bool {
			return command.Command == name && command.IsEphemeral
		})
	}
	for _, name := range []string{"chatchannels", "addchatchannel", "delmychatchannel", "delchatchannel"} {
		if !has(admin, name) || has(private, name) || has(group, name) != (name != "delchatchannel") {
			t.Errorf("incorrect menu visibility for /%s", name)
		}
	}
}
