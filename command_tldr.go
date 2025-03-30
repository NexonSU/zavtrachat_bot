package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PuerkitoBio/goquery"
	"github.com/tidwall/gjson"
)

// Send Yandex 300 response on link
func TLDR(bot *gotgbot.Bot, context *ext.Context) error {
	var err error
	if Config.YandexSummarizerToken == "" {
		return fmt.Errorf("не задан Yandex Summarizer токен")
	}
	if context.Message.ReplyToMessage == nil && len(context.Args()) == 1 {
		return ReplyAndRemove("Бот заберёт статью по ссылке (или сам текст больше 200 символов) и сделает её краткое описание.\nПример использования:\n<code>/tldr ссылка</code>.\nИли отправь в ответ на какое-либо сообщение с ссылкой.", *context)
	}

	link := ""
	message := &gotgbot.Message{}

	if context.Message.ReplyToMessage == nil {
		message = context.Message
	} else {
		message = context.Message.ReplyToMessage
	}

	for _, entity := range message.Entities {
		if entity.Type == "url" || entity.Type == "text_link" {
			link = entity.Url
			if link == "" {
				link = gotgbot.ParseEntity(message.Text, entity).Text
			}
		}
	}

	if link == "" {
		for _, entity := range message.CaptionEntities {
			if entity.Type == "url" || entity.Type == "text_link" {
				link = entity.Url
				if link == "" {
					link = gotgbot.ParseEntity(message.Text, entity).Text
				}
			}
		}
	}

	if link == "" {
		if len(message.Text) < 200 && len(message.Caption) < 200 {
			return ReplyAndRemove("Бот заберёт статью по ссылке (или сам текст больше 200 символов) и сделает её краткое описание.\nПример использования:\n<code>/tldr ссылка</code>.\nИли отправь в ответ на какое-либо сообщение с ссылкой.", *context)
		}
		if message.Text != "" {
			link, err = createPage(message.Text)
		}
		if message.Caption != "" {
			link, err = createPage(message.Caption)
		}
		if err != nil {
			return err
		}
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://300.ya.ru/api/sharing-url",
		bytes.NewBuffer([]byte(`{"article_url": "`+link+`"}`)))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "OAuth "+Config.YandexSummarizerToken)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		link, err = webProxy(link)
		if err != nil {
			return fmt.Errorf("webProxy error: %s", err.Error())
		}
		req, err = http.NewRequest("POST", "https://300.ya.ru/api/sharing-url",
			bytes.NewBuffer([]byte(`{"article_url": "`+link+`"}`)))
		if err != nil {
			return err
		}
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Authorization", "OAuth "+Config.YandexSummarizerToken)

		resp, err = client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			if len(message.Text) < 200 && len(message.Caption) < 200 {
				return fmt.Errorf("webProxy-link status code error: %d %s", resp.StatusCode, resp.Status)
			}
			if message.Text != "" {
				link, err = createPage(message.Text)
			}
			if message.Caption != "" {
				link, err = createPage(message.Caption)
			}
			if err != nil {
				return err
			}
			req, err = http.NewRequest("POST", "https://300.ya.ru/api/sharing-url",
				bytes.NewBuffer([]byte(`{"article_url": "`+link+`"}`)))
			if err != nil {
				return err
			}
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("Authorization", "OAuth "+Config.YandexSummarizerToken)

			resp, err = client.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				return fmt.Errorf("textProxy-link status code error: %d %s", resp.StatusCode, resp.Status)
			}
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if gjson.Get(string(body), "status").Str != "success" {
		return fmt.Errorf("ошибка, статус: %v", gjson.Get(string(body), "status").Str)
	}

	res, err := http.Get(gjson.Get(string(body), "sharing_url").Str)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("300 status code error: %d %s", res.StatusCode, res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return err
	}

	text := doc.Find(".summary .summary-content .summary-text").Text()

	text = regexp.MustCompile(`\n`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`•`).ReplaceAllString(text, "\n•")
	text = regexp.MustCompile(`[ ]+`).ReplaceAllString(text, ` `)

	if utf8.RuneCountInString(text) > 4000 {
		text = string([]rune(text)[:4000])
	}

	if strings.Contains(link, "zavtrabot.nexon.su") {
		os.Remove(strings.Replace(link, "https://", "/home/nginx/", -1))
	}

	_, err = context.Message.Reply(bot, text, &gotgbot.SendMessageOpts{ParseMode: "HTML"})
	return err
}

func webProxy(url string) (link string, error error) {
	linkName := fmt.Sprintf("%x", md5.Sum([]byte(url)))

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(fmt.Sprintf("/home/nginx/zavtrabot.nexon.su/%x.html", linkName), body, 0644)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("https://zavtrabot.nexon.su/%x.html", linkName), nil
}

func createPage(text string) (link string, error error) {
	linkName := fmt.Sprintf("%x", md5.Sum([]byte(text)))

	text = "<!doctype html><html><head><title>Пересказ сообщения</title></head><body><p>" + text + "</p></body></html>"

	err := os.WriteFile(fmt.Sprintf("/home/nginx/zavtrabot.nexon.su/%x.html", linkName), []byte(text), 0644)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("https://zavtrabot.nexon.su/%x.html", linkName), nil
}
