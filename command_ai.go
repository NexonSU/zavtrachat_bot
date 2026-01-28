package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

var AIBusy bool
var MarkdownBold = regexp.MustCompile(`\*\*(.*)\*\*`)
var AISystem string
var AIModel string

// Send shrug in chat on /shrug
func AI(bot *gotgbot.Bot, context *ext.Context) error {
	if AISystem == "" {
		AISystem = Config.OpenWebUISystem
	}
	if AIModel == "" {
		AIModel = Config.OpenWebUIModel
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

	prompt := strings.Join(slices.Delete(context.Args(), 0, 1), " ")
	var imgData []byte
	var messages []OpenWebUIMessage

	if context.Message.ReplyToMessage != nil {
		if len(context.Message.ReplyToMessage.Photo) > 0 {
			file, err := Bot.GetFile(context.Message.ReplyToMessage.Photo[0].FileId, nil)
			if err != nil {
				return err
			}
			imgData, err = os.ReadFile(file.FilePath)
			if err != nil {
				return err
			}
		}

		if len(context.Message.ReplyToMessage.Caption) > 0 {
			prompt += "\nПодпись: " + context.Message.ReplyToMessage.Caption
		}

		if len(context.Message.ReplyToMessage.Text) > 0 {
			prompt += "\nТекст: " + context.Message.ReplyToMessage.Text
		}
	}

	messages = append(messages, OpenWebUIMessage{
		Role:    "user",
		Content: prompt,
	})

	result, err := OpenWebUIRequest("/api/chat/completions", OpenWebUIRequestFields{
		Model:    AIModel,
		Format:   "json",
		Stream:   false,
		System:   AISystem,
		Messages: messages,
	}, imgData)
	if err != nil {
		return err
	}
	_, err = context.Message.Reply(bot, result.Choices[0].Message.Content, nil)
	return err
}

type OpenWebUIRequestFields struct {
	Model    string             `json:"model"`
	Messages []OpenWebUIMessage `json:"messages"`
	Format   string             `json:"format"`
	Prompt   string             `json:"prompt"`
	System   string             `json:"system"`
	Stream   bool               `json:"stream"`
}

type OpenWebUIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenWebUIResponse struct {
	Choices []OpenWebUIResponseChoice `json:"choices"`
}

type OpenWebUIResponseChoice struct {
	Message OpenWebUIMessage `json:"message"`
}

func OpenWebUIRequest(endpoint string, fields OpenWebUIRequestFields, imgData []byte) (OpenWebUIResponse, error) {
	postBody, err := json.Marshal(fields)
	if err != nil {
		return OpenWebUIResponse{}, err
	}
	req, err := http.NewRequest("POST", Config.OpenWebUIURL+endpoint, bytes.NewBuffer(postBody))
	req.Header.Set("Authorization", "Bearer "+Config.OpenWebUIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return OpenWebUIResponse{}, err
	}
	defer resp.Body.Close()

	response := OpenWebUIResponse{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	for _, choise := range response.Choices {
		choise.Message.Content = strings.ReplaceAll(choise.Message.Content, "<", "&lt;")
		choise.Message.Content = strings.ReplaceAll(choise.Message.Content, ">", "&gt;")
		choise.Message.Content = strings.ReplaceAll(choise.Message.Content, "*", "")
	}
	return response, err
}
