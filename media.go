package main

// The media of an incoming message: what it is, which file behind it, and where that file is
// on disk.

import (
	"os"
	"path/filepath"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type Media struct {
	Type                  string
	FileID                string
	FileName              string
	FilePath              string
	FileURL               string
	FileSize              int64
	Height                int64
	Width                 int64
	Duration              int64
	Caption               string
	CaptionEntities       []gotgbot.MessageEntity
	HasSpoiler            bool
	ShowCaptionAboveMedia bool
	MediaGroupId          string
}

func IsContainsMedia(message *gotgbot.Message) bool {
	switch {
	case message.Photo != nil:
		return true
	case message.Voice != nil:
		return true
	case message.Audio != nil:
		return true
	case message.Animation != nil:
		return true
	case message.Sticker != nil:
		return true
	case message.Document != nil:
		return true
	case message.Video != nil:
		return true
	case message.VideoNote != nil:
		return true
	default:
		return false
	}
}

func GetMedia(message *gotgbot.Message) (Media, error) {
	var result Media

	switch {
	case message.Photo != nil:
		photoIndex := 0
		for ndx, photo := range message.Photo {
			if photo.FileSize > message.Photo[photoIndex].FileSize {
				photoIndex = ndx
			}
		}
		result.Type = "photo"
		result.FileID = message.Photo[photoIndex].FileId
		result.Height = message.Photo[photoIndex].Height
		result.Width = message.Photo[photoIndex].Width
		result.FileSize = message.Photo[photoIndex].FileSize
	case message.Voice != nil:
		result.Type = "voice"
		result.FileID = message.Voice.FileId
		result.Duration = message.Voice.Duration
		result.FileSize = message.Voice.FileSize
	case message.Audio != nil:
		result.Type = "audio"
		result.FileID = message.Audio.FileId
		result.Duration = message.Audio.Duration
		result.FileSize = message.Audio.FileSize
	case message.Animation != nil:
		result.Type = "animation"
		result.FileID = message.Animation.FileId
		result.Height = message.Animation.Height
		result.Width = message.Animation.Width
		result.Duration = message.Animation.Duration
		result.FileSize = message.Animation.FileSize
	case message.Sticker != nil:
		result.Type = "sticker"
		result.FileID = message.Sticker.FileId
		result.Height = message.Sticker.Height
		result.Width = message.Sticker.Width
		result.FileSize = message.Sticker.FileSize
	case message.Video != nil:
		result.Type = "video"
		result.FileID = message.Video.FileId
		result.Height = message.Video.Height
		result.Width = message.Video.Width
		result.Duration = message.Video.Duration
		result.FileSize = message.Video.FileSize
	case message.VideoNote != nil:
		result.Type = "video_note"
		result.FileID = message.VideoNote.FileId
		result.Duration = message.VideoNote.Duration
		result.FileSize = message.VideoNote.FileSize
	default:
		result.Type = "document"
		result.FileID = message.Document.FileId
		result.FileSize = message.Document.FileSize
	}
	result.Caption = message.Caption
	result.CaptionEntities = message.CaptionEntities
	result.HasSpoiler = message.HasMediaSpoiler
	result.ShowCaptionAboveMedia = message.ShowCaptionAboveMedia
	result.MediaGroupId = message.MediaGroupId

	file, err := Bot.GetFile(result.FileID, nil)
	if err != nil {
		return Media{}, err
	}

	result.FilePath = file.FilePath
	result.FileName = filepath.Base(file.FilePath)
	result.FileURL = file.URL(Bot, nil)

	for _, localPath := range mediaLocalPaths(result.FilePath, Bot.Id) {
		if info, err := os.Stat(localPath); err == nil && info.Mode().IsRegular() {
			result.FilePath = localPath
			break
		}
	}

	return result, nil
}
