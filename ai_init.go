package main

import (
	cntx "context"
	"net/http"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/flow/agent/react"
)

func AiInit() error {
	var err error

	//system message
	AISystem = Config.AISystem

	//context
	context := cntx.Background()

	//model init
	headers := make(http.Header)
	headers.Add("Authorization", "Bearer "+Config.AIToken)
	modelCfg, err := openai.NewChatModel(context, &openai.ChatModelConfig{
		HTTPClient: &http.Client{
			Transport: &AuthTransport{
				Header: headers,
				Base:   http.DefaultTransport,
			},
		},
		BaseURL: Config.AIURL,
		Model:   Config.AIToolModel,
	})
	if err != nil {
		return err
	}

	AIAgent, err = react.NewAgent(context, &react.AgentConfig{
		ToolCallingModel: modelCfg,
	})
	if err != nil {
		return err
	}

	return nil
}
