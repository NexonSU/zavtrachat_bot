package main

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	tele "gopkg.in/telebot.v3"
)

type AdviceResp struct {
	ID    int    `json:"id,omitempty"`
	Text  string `json:"text,omitempty"`
	Sound string `json:"sound,omitempty"`
}

func Advice(context tele.Context) error {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	httpResponse, err := httpClient.Get("http://fucking-great-advice.ru/api/random")
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		Body.Close()
	}(httpResponse.Body)

	var advice AdviceResp
	err = json.NewDecoder(httpResponse.Body).Decode(&advice)
	if err != nil {
		return err
	}

	return ReplyAndRemove(advice.Text, context)
}
