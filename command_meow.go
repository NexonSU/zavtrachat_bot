package main

import (
	"bytes"
	"reflect"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"
)

var meowChannel *tg.Channel

// Reply with GIF from Pan Kotek's channel
func Meow(bot *gotgbot.Bot, context *ext.Context) error {
	api := GoTGProtoClient.API()

	//get channel object
	if meowChannel == nil {
		channelResolve, err := api.ContactsResolveUsername(GoTGProtoContext, &tg.ContactsResolveUsernameRequest{Username: "imacat"})
		if err != nil {
			return err
		}
		meowChannel = channelResolve.GetChats()[0].(*tg.Channel)
	}
	//prepare message query
	messagesQuery := []tg.InputMessageClass{}
	firstMessageId := RandInt(15, 24613)
	for message_id := firstMessageId; message_id < firstMessageId+10; message_id++ {
		messageObject := tg.Message{ID: message_id}
		messagesQuery = append(messagesQuery, messageObject.AsInputMessageID())
	}
	//query messages
	messagesResult, err := api.ChannelsGetMessages(GoTGProtoContext, &tg.ChannelsGetMessagesRequest{
		Channel: meowChannel.AsInput(),
		ID:      messagesQuery,
	})
	if err != nil {
		return err
	}
	//search and download gif
	for _, mc := range messagesResult.(*tg.MessagesChannelMessages).Messages {
		if reflect.TypeOf(mc) != reflect.TypeOf(&tg.Message{}) {
			continue
		}
		messageMediaClass, check := mc.(*tg.Message).GetMedia()
		if check && reflect.TypeOf(messageMediaClass) == reflect.TypeOf(&tg.MessageMediaDocument{}) {
			document, check := messageMediaClass.(*tg.MessageMediaDocument).GetDocument()
			if !check {
				continue
			}
			docFile, check := document.AsNotEmpty()
			if !check {
				continue
			}
			fileNameObj, check := docFile.MapAttributes().AsDocumentAttributeFilename().First()
			filename := fileNameObj.FileName
			if !check {
				filename = strings.ReplaceAll(docFile.MimeType, "/", ".")
			}
			if docFile.MimeType == "video/quicktime" {
				continue
			}
			buf := bytes.Buffer{}
			downloader.NewDownloader().Download(api, docFile.AsInputDocumentFileLocation()).Stream(GoTGProtoContext, &buf)
			if strings.Contains(docFile.MimeType, "video") {
				if docFile.MimeType == "video/quicktime" {
					_, err = bot.SendAnimation(context.Message.Chat.Id, gotgbot.InputFileByReader(filename, &buf), &gotgbot.SendAnimationOpts{ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.MessageId}})
				} else {
					_, err = bot.SendVideo(context.Message.Chat.Id, gotgbot.InputFileByReader(filename, &buf), &gotgbot.SendVideoOpts{SupportsStreaming: true, ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.MessageId}})
				}
			}
			if strings.Contains(docFile.MimeType, "image") {
				_, err = bot.SendPhoto(context.Message.Chat.Id, gotgbot.InputFileByReader(filename, &buf), &gotgbot.SendPhotoOpts{ReplyParameters: &gotgbot.ReplyParameters{MessageId: context.Message.MessageId}})
			}
			return err
		} else {
			continue
		}
	}

	return nil
}
