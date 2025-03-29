package main

import (
	"bytes"
	"reflect"
	"strings"

	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"
	tele "gopkg.in/telebot.v3"
)

var channel *tg.Channel

// Reply with GIF from Pan Kotek's channel
func Meow(context tele.Context) error {
	api := GotdClient.API()

	//get channel object
	if channel == nil {
		channelResolve, err := api.ContactsResolveUsername(GotdContext, "imacat")
		if err != nil {
			return err
		}
		channel = channelResolve.GetChats()[0].(*tg.Channel)
	}
	//prepare message query
	messagesQuery := []tg.InputMessageClass{}
	firstMessageId := RandInt(15, 17500)
	for message_id := firstMessageId; message_id < firstMessageId+10; message_id++ {
		messageObject := tg.Message{ID: message_id}
		messagesQuery = append(messagesQuery, messageObject.AsInputMessageID())
	}
	//query messages
	messagesResult, err := api.ChannelsGetMessages(GotdContext, &tg.ChannelsGetMessagesRequest{
		Channel: channel.AsInput(),
		ID:      messagesQuery,
	})
	if err != nil {
		return err
	}
	//search and download gif
	buf := bytes.Buffer{}
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
			downloader.NewDownloader().Download(api, docFile.AsInputDocumentFileLocation()).Stream(GotdContext, &buf)
			return context.Reply(&tele.Document{FileName: filename, File: tele.File{FileReader: &buf}})
		} else {
			continue
		}
	}

	return nil
}
