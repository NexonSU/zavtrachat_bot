package main

import (
	"encoding/json"
	"log"
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
	MaxConnections        int      `json:"max_connections"`
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
}

func ConfigInit(file string) Configuration {
	var Config Configuration
	if _, err := os.Stat(file); err == nil {
		ConfigFile, err := os.Open(file)
		if err != nil {
			log.Fatal(err)
		}
		err = json.NewDecoder(ConfigFile).Decode(&Config)
		if err != nil {
			log.Fatal(err)
		}
	} else if os.IsNotExist(err) {
		Config.Admins = []int64{}
		Config.Moders = []int64{}
		Config.BotApiUrl = "https://api.telegram.org"
		Config.AllowedUpdates = []string{"message", "channel_post", "callback_query", "chat_member"}
		jsonData, _ := json.MarshalIndent(Config, "", "\t")
		_ = os.WriteFile(file, jsonData, 0600)
		log.Fatal(err)
	}
	return Config
}

var Config = ConfigInit("config.json")
