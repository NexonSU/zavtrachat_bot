package main

import (
	cntx "context"

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
	modelCfg, err := openai.NewChatModel(context, &openai.ChatModelConfig{
		APIKey:  Config.AIToken,
		BaseURL: Config.AIURL,
		Model:   Config.AIModel,
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
