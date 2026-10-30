package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/gotd/td/tg"
)

var errQuoteNativeUnavailable = errors.New("native client unavailable")

type quoteLookup func(*gotgbot.Message) (string, error)

type quoteMessagesAPI interface {
	ChannelsGetMessages(context.Context, *tg.ChannelsGetMessagesRequest) (tg.MessagesMessagesClass, error)
	MessagesGetMessages(context.Context, []tg.InputMessageClass) (tg.MessagesMessagesClass, error)
}

// Read the command's reply header, never the source message's own reply header.
// Bot API may omit quote metadata; MTProto preserves the native reply header.
func recoverQuoteSelection(message *gotgbot.Message) (string, error) {
	api, base, ready := mtprotoAPI()
	if !ready {
		return "", errQuoteNativeUnavailable
	}
	ctx, cancel := context.WithTimeout(base, 4*time.Second)
	defer cancel()
	return quoteSelectionFromTelegram(ctx, api, message)
}

func quotePeerMatches(peer tg.PeerClass, chatID int64) bool {
	switch p := peer.(type) {
	case *tg.PeerChannel:
		return chatID == -1000000000000-p.ChannelID
	case *tg.PeerChat:
		return chatID == -p.ChatID
	case *tg.PeerUser:
		return chatID == p.UserID
	}
	return false
}

func quoteSelectionFromTelegram(ctx context.Context, api quoteMessagesAPI, message *gotgbot.Message) (string, error) {
	if message == nil || message.ReplyToMessage == nil {
		return "", fmt.Errorf("missing reply")
	}
	ids := []tg.InputMessageClass{&tg.InputMessageID{ID: int(message.MessageId)}}
	var result tg.MessagesMessagesClass
	var err error
	if message.Chat.Id < -1000000000000 {
		result, err = api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{Channel: &tg.InputChannel{ChannelID: -message.Chat.Id - 1000000000000}, ID: ids})
	} else {
		result, err = api.MessagesGetMessages(ctx, ids)
	}
	if err != nil {
		return "", err
	}
	var messages []tg.MessageClass
	switch r := result.(type) {
	case *tg.MessagesChannelMessages:
		messages = r.Messages
	case *tg.MessagesMessages:
		messages = r.Messages
	case *tg.MessagesMessagesSlice:
		messages = r.Messages
	default:
		return "", fmt.Errorf("unexpected message response")
	}
	for _, value := range messages {
		m, ok := value.(*tg.Message)
		if !ok || int64(m.ID) != message.MessageId || !quotePeerMatches(m.PeerID, message.Chat.Id) {
			continue
		}
		if m.Message != message.Text {
			return "", fmt.Errorf("command changed")
		}
		header, ok := m.ReplyTo.(*tg.MessageReplyHeader)
		if !ok || int64(header.ReplyToMsgID) != message.ReplyToMessage.MessageId {
			return "", fmt.Errorf("reply mismatch")
		}
		if header.ReplyToPeerID != nil && !quotePeerMatches(header.ReplyToPeerID, message.Chat.Id) {
			return "", fmt.Errorf("external reply")
		}
		return header.QuoteText, nil
	}
	return "", fmt.Errorf("command not found")
}

func quoteResolvedContent(message *gotgbot.Message, lookup quoteLookup) (quoteContent, bool, string) {
	q, valid := quoteSelectedContent(message)
	if message.Quote != nil {
		return q, valid, "bot_api"
	}
	selected, err := lookup(message)
	if err != nil {
		if errors.Is(err, errQuoteNativeUnavailable) {
			return q, valid, "unavailable_offline"
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return q, valid, "unavailable_timeout"
		}
		return q, valid, "unavailable"
	}
	if selected == "" {
		return q, valid, "full"
	}
	if !strings.Contains(q.Text, selected) {
		return q, false, "mismatch"
	}
	q.Text = selected
	return q, true, "native"
}

func quoteSelectionNotice(status string) string {
	switch status {
	case "full":
		return "Telegram не передал выделенный фрагмент — использован весь текст."
	case "unavailable", "unavailable_offline", "unavailable_timeout":
		return "В Bot API нет выделения; проверить его напрямую сейчас не удалось — использован весь текст."
	}
	return ""
}

func quoteSelectionDiagnostic(message *gotgbot.Message, q quoteContent, valid bool, status string) string {
	apiState := "нет"
	if message.Quote != nil {
		apiState = fmt.Sprintf("есть, %d символов", utf8.RuneCountInString(message.Quote.Text))
	}
	states := map[string]string{"bot_api": "фрагмент из Bot API", "native": "фрагмент восстановлен напрямую из Telegram", "full": "в Telegram нет фрагмента", "unavailable": "прямое чтение недоступно", "mismatch": "фрагмент не совпадает с оригиналом", "unavailable_offline": "MTProto-клиент не подключён", "unavailable_timeout": "таймаут прямого чтения"}
	return fmt.Sprintf("Проверка /quote\nВерсия: native-recovery-v1\nBot API: %s\nРезультат: %s\nТекст для карточки: %d символов\nСовпадение с оригиналом: %t", apiState, states[status], utf8.RuneCountInString(q.Text), valid)
}
