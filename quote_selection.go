package main

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

const quoteRejectedText = "Хехе, попа, а не цитата, выбери другое."

// Token-window matching accepts spacing/punctuation, ё/е, repeated letters and
// common Russian spellings, without matching a substring inside another word.
var rejectedQuotePhrase = regexp.MustCompile(`^(?:п[оа]?ш[ео]?л(?:а|о|и(?:те)?|те)?|иди(?:те)?)(?:ка|ты|вы){0,2}нахуй$`)

func quoteWords(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
}

func quoteForbidden(text string) bool {
	words := quoteWords(text)
	if len(words) == 1 {
		letters := 0
		for _, r := range words[0] {
			if unicode.IsLetter(r) {
				letters++
			}
		}
		if letters > 0 && letters <= 4 && letters == utf8.RuneCountInString(words[0]) {
			return true
		}
	}
	for start := range words {
		compact := ""
		for end := start; end < len(words) && end < start+16; end++ {
			compact += strings.ReplaceAll(strings.ToLower(words[end]), "ё", "е")
			var normalized strings.Builder
			var previous rune
			for _, r := range compact {
				if r != previous {
					normalized.WriteRune(r)
				}
				previous = r
			}
			if rejectedQuotePhrase.MatchString(normalized.String()) {
				return true
			}
			if len(compact) > 160 {
				break
			}
		}
	}
	return false
}

// Telegram quote text replaces the source text regardless of the optional is_manual flag.
// Position is approximate UTF-16; substring validation avoids splitting emoji.
func quoteSelectedContent(message *gotgbot.Message) (quoteContent, bool) {
	q := quoteFromMessage(message.ReplyToMessage)
	if message.Quote != nil {
		selected := message.Quote.Text
		if selected == "" || !strings.Contains(q.Text, selected) {
			return q, false
		}
		q.Text = selected
	}
	return q, true
}
