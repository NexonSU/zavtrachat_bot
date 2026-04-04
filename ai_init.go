package main

import (
	cntx "context"
	"net/http"

	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino/compose"
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
	AIToolModel, err = ollama.NewChatModel(context, &ollama.ChatModelConfig{
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
	AIVisionModel, err = ollama.NewChatModel(context, &ollama.ChatModelConfig{
		HTTPClient: &http.Client{
			Transport: &AuthTransport{
				Header: headers,
				Base:   http.DefaultTransport,
			},
		},
		BaseURL: Config.AIURL,
		Model:   Config.AIVisionModel,
	})
	if err != nil {
		return err
	}

	//tools request
	tools, err := GetAiTools()
	if err != nil {
		return err
	}

	AIAgent, err = react.NewAgent(context, &react.AgentConfig{
		ToolCallingModel: AIToolModel,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools:                tools,
			ToolArgumentsHandler: CheckToolRestrictions,
		},
	})
	if err != nil {
		return err
	}

	return nil
}
