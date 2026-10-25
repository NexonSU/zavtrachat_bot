package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/gotd/td/bin"
	"github.com/gotd/td/tg"
)

// Exercise the generated Telegram client and the real downloader, replacing
// only the MTProto wire transport. No live channels or credentials are used.
type fakeMediaInvoker struct {
	mu           sync.Mutex
	chats        []tg.ChatClass
	messages     tg.MessagesMessagesClass
	data         []byte
	fail         string
	resolveCalls int
	getCalls     int
	fileCalls    int
	username     string
	requestedIDs int
}

func (f *fakeMediaInvoker) Invoke(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var result bin.Encoder
	var stage string
	switch req := input.(type) {
	case *tg.ContactsResolveUsernameRequest:
		f.resolveCalls++
		f.username = req.Username
		stage = "resolve"
		result = &tg.ContactsResolvedPeer{Peer: &tg.PeerChannel{ChannelID: 10}, Chats: f.chats}
	case *tg.ChannelsGetMessagesRequest:
		f.getCalls++
		f.requestedIDs = len(req.ID)
		stage = "messages"
		result = f.messages
	case *tg.UploadGetFileRequest:
		f.fileCalls++
		stage = "download"
		data := f.data
		if req.Offset >= int64(len(data)) {
			data = nil
		} else {
			data = data[req.Offset:]
		}
		result = &tg.UploadFile{Type: &tg.StorageFileUnknown{}, Bytes: data}
	default:
		return fmt.Errorf("unexpected MTProto request %T", input)
	}
	if f.fail == stage {
		return fmt.Errorf("fixture %s failed", stage)
	}
	var wire bin.Buffer
	if err := result.Encode(&wire); err != nil {
		return err
	}
	return output.Decode(&wire)
}

func resetProtoMediaCaches(t *testing.T) {
	t.Helper()
	oldMeow, oldWoof, oldMeme := meowChannel, woofChannel, memeChannel
	meowChannel, woofChannel, memeChannel = &protoMediaChannelCache{}, &protoMediaChannelCache{}, &protoMediaChannelCache{}
	t.Cleanup(func() {
		meowChannel, woofChannel, memeChannel = oldMeow, oldWoof, oldMeme
	})
}

type protoMediaHandler func(*gotgbot.Bot, *ext.Context, *tg.Client, context.Context) error

func protoMediaHandlers() map[string]protoMediaHandler {
	return map[string]protoMediaHandler{"meow": meowWithAPI, "woof": woofWithAPI, "meme": memeWithAPI}
}

func protoDocumentMessage(mime string, named bool) *tg.Message {
	doc := &tg.Document{ID: 1, AccessHash: 2, FileReference: []byte("reference"), MimeType: mime, Size: 10, DCID: 1}
	if named {
		doc.Attributes = []tg.DocumentAttributeClass{&tg.DocumentAttributeFilename{FileName: "source.file"}}
	}
	return &tg.Message{ID: 123, PeerID: &tg.PeerChannel{ChannelID: 10}, Media: &tg.MessageMediaDocument{Document: doc}}
}

func protoMediaFixture(messages ...tg.MessageClass) *fakeMediaInvoker {
	return &fakeMediaInvoker{
		chats:    []tg.ChatClass{&tg.Channel{ID: 10, AccessHash: 20, Title: "fixture", Photo: &tg.ChatPhotoEmpty{}}},
		messages: &tg.MessagesChannelMessages{Messages: messages},
		data:     []byte("media fixture bytes"),
	}
}

func TestProtoMediaDocumentDelivery(t *testing.T) {
	for command, handler := range protoMediaHandlers() {
		for _, mime := range []string{"video/mp4", "image/jpeg", "video/quicktime"} {
			for _, named := range []bool{true, false} {
				t.Run(fmt.Sprintf("%s/%s/named=%v", command, mime, named), func(t *testing.T) {
					bot, client := setupTestEnv(t)
					resetProtoMediaCaches(t)
					transport := protoMediaFixture(protoDocumentMessage(mime, named))
					var sent []byte
					var filename string
					client.Observe = func(method string, params map[string]any) {
						key := strings.TrimPrefix(method, "send")
						if file, ok := params[strings.ToLower(key)].(*gotgbot.FileReader); ok {
							filename = file.Name
							sent, _ = io.ReadAll(file.Data)
						}
					}
					if err := handler(bot, testContext(bot, "/"+command, testUser(42)), tg.NewClient(transport), context.Background()); err != nil {
						t.Fatal(err)
					}
					if command == "meme" && mime == "video/quicktime" {
						if len(sent) != 0 || transport.fileCalls != 0 {
							t.Fatal("meme's existing quicktime exclusion changed")
						}
						return
					}
					method := "sendVideo"
					if mime == "image/jpeg" {
						method = "sendPhoto"
					}
					if mime == "video/quicktime" {
						method = "sendAnimation"
					}
					calls := client.callsFor(method)
					if len(calls) != 1 || calls[0].Params["chat_id"] != Config.Chat || !bytes.Equal(sent, transport.data) {
						t.Fatalf("media did not reach the invoking chat: %+v", calls)
					}
					wantName := strings.ReplaceAll(mime, "/", ".")
					if named {
						wantName = "source.file"
					}
					if filename != wantName || transport.requestedIDs != 10 || transport.resolveCalls != 1 {
						t.Fatalf("filename/query mismatch: %q, %+v", filename, transport)
					}
					wantChannel := map[string]string{"meow": "imacat", "woof": "imnotacat", "meme": "thedankestmemes"}[command]
					if transport.username != wantChannel {
						t.Fatal("resolved a different source channel")
					}
				})
			}
		}
	}
}

func TestProtoMediaErrorsAndUnexpectedAnswers(t *testing.T) {
	for command, handler := range protoMediaHandlers() {
		for _, kind := range []string{"resolve", "messages", "download", "no channel", "wrong channel", "wrong messages", "upload"} {
			t.Run(command+"/"+kind, func(t *testing.T) {
				bot, client := setupTestEnv(t)
				resetProtoMediaCaches(t)
				transport := protoMediaFixture(protoDocumentMessage("video/mp4", true))
				switch kind {
				case "no channel":
					transport.chats = nil
				case "wrong channel":
					transport.chats = []tg.ChatClass{&tg.ChatEmpty{ID: 1}}
				case "wrong messages":
					transport.messages = &tg.MessagesMessages{}
				case "upload":
					client.setError("sendVideo", errors.New("upload failed"))
				default:
					transport.fail = kind
				}
				err := handler(bot, testContext(bot, "/"+command, testUser(42)), tg.NewClient(transport), context.Background())
				if err == nil {
					t.Fatal("failure was silently discarded")
				}
				if (kind == "resolve" || kind == "messages" || kind == "download") && !strings.Contains(err.Error(), "fixture "+kind+" failed") {
					t.Fatalf("test did not reach the intended failure: %v", err)
				}
				if kind != "upload" && len(client.callsFor("sendVideo")) != 0 {
					t.Fatal("sent media after failed fetch")
				}
				if kind == "resolve" && transport.resolveCalls != 2 {
					t.Fatal("read-only resolve retry missing")
				}
				if kind == "messages" && transport.getCalls != 2 {
					t.Fatal("read-only message retry missing")
				}
			})
		}
	}
}

func TestProtoMediaSkipsEmptyAndUnsupportedMessages(t *testing.T) {
	for command, handler := range protoMediaHandlers() {
		t.Run(command, func(t *testing.T) {
			bot, client := setupTestEnv(t)
			resetProtoMediaCaches(t)
			transport := protoMediaFixture(
				&tg.MessageEmpty{ID: 1},
				&tg.Message{ID: 2, PeerID: &tg.PeerChannel{ChannelID: 10}},
				&tg.Message{ID: 3, PeerID: &tg.PeerChannel{ChannelID: 10}, Media: &tg.MessageMediaDocument{Document: &tg.DocumentEmpty{ID: 3}}},
				protoDocumentMessage("image/jpeg", false),
			)
			if err := handler(bot, testContext(bot, "/"+command, testUser(42)), tg.NewClient(transport), context.Background()); err != nil {
				t.Fatal(err)
			}
			if len(client.callsFor("sendPhoto")) != 1 {
				t.Fatal("valid media after empty messages was not found")
			}
			transport.messages = &tg.MessagesChannelMessages{}
			client.clearCalls()
			if err := handler(bot, testContext(bot, "/"+command, testUser(42)), tg.NewClient(transport), context.Background()); err != nil {
				t.Fatal(err)
			}
			if len(client.callsFor("sendPhoto")) != 0 || transport.resolveCalls != 1 {
				t.Fatal("empty result sent media or lost the channel cache")
			}
		})
	}
}

func TestMemePhotoDeliveryAndInvalidSizes(t *testing.T) {
	for _, kind := range []string{"valid", "no sizes", "empty size type", "empty photo", "download failure"} {
		t.Run(kind, func(t *testing.T) {
			bot, client := setupTestEnv(t)
			resetProtoMediaCaches(t)
			photo := &tg.Photo{ID: 1, AccessHash: 2, FileReference: []byte("reference"), Sizes: []tg.PhotoSizeClass{&tg.PhotoSize{Type: "x", W: 128, H: 96, Size: 10}}}
			media := &tg.MessageMediaPhoto{Photo: photo}
			switch kind {
			case "no sizes":
				photo.Sizes = nil
			case "empty size type":
				photo.Sizes[0].(*tg.PhotoSize).Type = ""
			case "empty photo":
				media.Photo = &tg.PhotoEmpty{ID: 1}
			}
			transport := protoMediaFixture(&tg.Message{ID: 123, PeerID: &tg.PeerChannel{ChannelID: 10}, Media: media})
			if kind == "download failure" {
				transport.fail = "download"
			}
			err := memeWithAPI(bot, testContext(bot, "/meme", testUser(42)), tg.NewClient(transport), context.Background())
			if (err != nil) != (kind == "download failure") {
				t.Fatalf("unexpected error: %v", err)
			}
			want := 0
			if kind == "valid" {
				want = 1
			}
			if len(client.callsFor("sendPhoto")) != want {
				t.Fatal("incorrect handling of photo/size data")
			}
		})
	}
}

func TestProtoMediaConcurrentCacheAndCancelledContext(t *testing.T) {
	bot, client := setupTestEnv(t)
	resetProtoMediaCaches(t)
	transport := protoMediaFixture(protoDocumentMessage("video/mp4", false))
	api := tg.NewClient(transport)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- meowWithAPI(bot, testContext(bot, "/meow", testUser(42)), api, context.Background())
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if transport.resolveCalls != 1 || len(client.callsFor("sendVideo")) != 8 {
		t.Fatal("concurrent calls raced or lost cached channel")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := meowWithAPI(bot, testContext(bot, "/meow", testUser(42)), api, ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
