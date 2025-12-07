package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
)

type Configuration struct {
	Token                 string   `json:"token"`
	AppID                 int      `json:"app_id"`
	AppHash               string   `json:"app_hash"`
	BotApiUrl             string   `json:"bot_api_url"`
	AllowedUpdates        []string `json:"allowed_updates"`
	Listen                string   `json:"listen"`
	EndpointPublicURL     string   `json:"endpoint_public_url"`
	MaxConnections        int64    `json:"max_connections"`
	Chat                  int64    `json:"chat"`
	ReserveChat           int64    `json:"reserve_chat"`
	CommentChat           int64    `json:"comment_chat"`
	StreamChannel         int64    `json:"stream_channel"`
	Channel               int64    `json:"channel"`
	Admins                []int64  `json:"admins"`
	Moders                []int64  `json:"moders"`
	SysAdmin              int64    `json:"sysadmin"`
	CurrencyKey           string   `json:"currency_key"`
	OpenAIKey             string   `json:"openai_key"`
	ReleasesUrl           string   `json:"releases_url"`
	NHentaiCookie         string   `json:"nhentai_cookie"`
	YandexSummarizerToken string   `json:"yandex_summarizer_token"`
	FontPath              string   `json:"fontpath"`
	Proxy                 string   `json:"proxy"`
}

func ConfigInit() error {
	if _, err := os.Stat("config.json"); err == nil {
		ConfigFile, err := os.Open("config.json")
		if err != nil {
			return err
		}
		err = json.NewDecoder(ConfigFile).Decode(&Config)
		if err != nil {
			return err
		}
	} else if os.IsNotExist(err) {
		Config.Admins = []int64{}
		Config.Moders = []int64{}
		Config.BotApiUrl = "https://api.telegram.org"
		Config.AllowedUpdates = []string{"message", "channel_post", "callback_query", "chat_member"}
		jsonData, err := json.MarshalIndent(Config, "", "\t")
		if err != nil {
			return err
		}
		err = os.WriteFile("config.json", jsonData, 0600)
		if err != nil {
			return err
		}
	}
	if Config.Token == "" {
		return fmt.Errorf("sting 'token' not found in config.json")
	}
	if Config.Chat == 0 {
		return fmt.Errorf("integer 'chat' not found in config.json")
	}
	if Config.Proxy != "" {
		proxyUrl, err := url.Parse(Config.Proxy)
		if err != nil {
			return err
		}
		HTTPClientProxy = http.ProxyURL(proxyUrl)
	}
	return nil
}
