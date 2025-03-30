package main

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Send text in chat on /say
func Anekdot(bot *gotgbot.Bot, context *ext.Context) error {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	httpResponse, err := httpClient.Get("https://www.anekdot.ru/rss/randomu.html")
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		Body.Close()
	}(httpResponse.Body)
	html, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return err
	}

	text := string(html)
	text = strings.Split(text, "JSON.parse('[\\\"")[1]
	text = strings.Split(text, "\\\",\\\"")[0]
	text = strings.ReplaceAll(text, "\\\\\\\"", "\"")
	br := regexp.MustCompile(`([а-я])<br>([а-я])`)
	text = br.ReplaceAllString(text, `$1 $2`)
	text = strings.ReplaceAll(text, "<br>", "\n")

	return ReplyAndRemove(text, *context)
}
