package main

import (
	cntx "context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/ollama/ollama/api"
)

var AIBusy bool
var MarkdownBold = regexp.MustCompile(`\*\*(.*)\*\*`)
var AISystem string
var AIModel string

// Send shrug in chat on /shrug
func AI(bot *gotgbot.Bot, context *ext.Context) error {
	if AISystem == "" {
		AISystem = Config.OllamaSystem
	}
	if AIModel == "" {
		AIModel = Config.OllamaModel
	}
	if DistortBusy {
		return ReplyAndRemoveWithTarget("Команда занята", *context)
	}

	context.EffectiveMessage.SetReaction(Bot, &gotgbot.SetMessageReactionOpts{
		Reaction: []gotgbot.ReactionType{gotgbot.ReactionTypeEmoji{Emoji: "👀"}},
	})

	var done = make(chan bool, 1)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				context.EffectiveChat.SendAction(bot, gotgbot.ChatActionTyping, nil)
			}
			time.Sleep(time.Second * 5)
		}
	}()

	defer func() {
		DistortBusy = false
		done <- true
	}()
	DistortBusy = true

	var msg *gotgbot.Message

	if context.Message.ReplyToMessage != nil {
		msg = context.Message.ReplyToMessage
	} else {
		msg = context.Message
	}

	url, err := url.Parse(Config.OllamaURL)
	if err != nil {
		return err
	}

	client := api.NewClient(url, http.DefaultClient)

	ctx := cntx.Background()

	req := &api.GenerateRequest{
		Model:  AIModel,
		Stream: new(bool),
		System: AISystem,
	}

	if len(msg.Photo) > 0 {
		file, err := Bot.GetFile(msg.Photo[0].FileId, nil)
		if err != nil {
			return err
		}
		imgData, err := os.ReadFile(file.FilePath)
		if err != nil {
			return err
		}
		req.Images = []api.ImageData{imgData}
	}

	req.Prompt = strings.Join(slices.Delete(context.Args(), 0, 1), " ")

	if len(msg.Caption) > 0 {
		req.Prompt += "\nПодпись: " + msg.Caption
	}

	if len(msg.Text) > 0 {
		req.Prompt += "\nТекст: " + msg.Text
	}

	err = client.Generate(ctx, req, func(resp api.GenerateResponse) error {
		return fmt.Errorf("%s", resp.Response)
	})

	result := fmt.Sprint(err)
	result = strings.ReplaceAll(result, "<", "&lt;")
	result = strings.ReplaceAll(result, ">", "&gt;")
	result = MarkdownBold.ReplaceAllString(result, `<b>$1</b>`)
	_, err = context.Message.Reply(bot, result, nil)
	return err
}
