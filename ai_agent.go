package main

import (
	cntx "context"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	tgmd "github.com/eekstunt/telegramify-markdown-go"

	"github.com/cloudwego/eino/schema"
)

// Send ai response in chat on /ai
func AI(bot *gotgbot.Bot, context *ext.Context) error {
	var messages []*schema.Message

	messages = append(messages, &schema.Message{Role: schema.System, Content: AISystem})

	aiCntx := cntx.WithValue(cntx.Background(), "tgUser", context.EffectiveSender.User.Id)

	if AIBusy {
		return ReplyAndRemoveWithTarget("Команда занята", *context)
	}
	if AIAgent == nil {
		return ReplyAndRemoveWithTarget("Агент не инициализирован", *context)
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
		AIBusy = false
		done <- true
	}()
	AIBusy = true

	msgText := strings.Join(slices.Delete(context.Args(), 0, 1), " ")

	if context.Message.ReplyToMessage != nil || len(context.Message.Photo) > 0 {
		inputParts := []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: msgText}}

		if len(context.Message.Photo) > 0 {
			b64, mimeType, err := getImageForLLM(context.Message)
			if err != nil {
				return err
			}
			aiCntx = cntx.WithValue(aiCntx, "b64", b64)
			aiCntx = cntx.WithValue(aiCntx, "mimeType", mimeType)

			inputParts = append(inputParts, schema.MessageInputPart{
				Type: schema.ChatMessagePartTypeImageURL,
				Image: &schema.MessageInputImage{
					MessagePartCommon: schema.MessagePartCommon{
						Base64Data: toPtr(b64),
						MIMEType:   mimeType,
					},
				},
			})
		}

		if context.Message.ReplyToMessage != nil {
			if len(context.Message.ReplyToMessage.Photo) > 0 {
				b64, mimeType, err := getImageForLLM(context.Message.ReplyToMessage)
				if err != nil {
					return err
				}
				aiCntx = cntx.WithValue(aiCntx, "b64", b64)
				aiCntx = cntx.WithValue(aiCntx, "mimeType", mimeType)

				inputParts = append(inputParts, schema.MessageInputPart{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageInputImage{
						MessagePartCommon: schema.MessagePartCommon{
							Base64Data: toPtr(b64),
							MIMEType:   mimeType,
						},
					},
				})
			}

			if len(context.Message.ReplyToMessage.Caption) > 0 {
				inputParts = append(inputParts, schema.MessageInputPart{Type: schema.ChatMessagePartTypeText, Text: context.Message.ReplyToMessage.Caption})
			}

			if len(context.Message.ReplyToMessage.Text) > 0 {
				inputParts = append(inputParts, schema.MessageInputPart{Type: schema.ChatMessagePartTypeText, Text: context.Message.ReplyToMessage.Text})
			}
		}

		messages = append(messages, &schema.Message{
			Role:                  schema.User,
			UserInputMultiContent: inputParts,
		})
	} else {
		messages = append(messages, &schema.Message{Role: schema.User, Content: msgText})
	}

	firstLineMsg := "Выполняю запрос...\n\n"
	completeMsg := firstLineMsg
	lastUpdate := time.Now().Unix()
	sentMsg, err := context.Message.Reply(bot, completeMsg, nil)

	msgStream, err := AIAgent.Stream(aiCntx, messages)

	if err != nil {
		return err
	}

	defer msgStream.Close()

	for {
		msg, err := msgStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		completeMsg = completeMsg + msg.Content
		if lastUpdate != time.Now().Unix() {
			lastUpdate = time.Now().Unix()

			tgmdMsg := tgmd.Convert(completeMsg)

			_, _, err = bot.EditMessageText(tgmdMsg.Text, &gotgbot.EditMessageTextOpts{ChatId: sentMsg.Chat.Id, MessageId: sentMsg.MessageId, Entities: toEntities(tgmdMsg.Entities)})
			if err != nil {
				bot.EditMessageText(tgmdMsg.Text, &gotgbot.EditMessageTextOpts{ChatId: sentMsg.Chat.Id, MessageId: sentMsg.MessageId})
			}
		}
	}

	completeMsg, _ = strings.CutPrefix(completeMsg, firstLineMsg)

	tgmdMsg := tgmd.Convert(completeMsg)

	_, _, err = bot.EditMessageText(tgmdMsg.Text, &gotgbot.EditMessageTextOpts{ChatId: sentMsg.Chat.Id, MessageId: sentMsg.MessageId, Entities: toEntities(tgmdMsg.Entities)})
	if err != nil {
		bot.EditMessageText(tgmdMsg.Text, &gotgbot.EditMessageTextOpts{ChatId: sentMsg.Chat.Id, MessageId: sentMsg.MessageId})
	}

	return nil
}
