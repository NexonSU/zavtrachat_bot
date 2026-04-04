package main

import (
	cntx "context"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	mcpp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
)

func GetAiTools() ([]tool.BaseTool, error) {
	var err error

	//context
	context := cntx.Background()

	//mcp client
	cli, err := client.NewStreamableHttpClient("http://127.0.0.1:8040")
	if err != nil {
		return nil, err
	}

	err = cli.Start(context)
	if err != nil {
		return nil, err
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	_, err = cli.Initialize(context, initReq)
	if err != nil {
		return nil, err
	}

	//tools request
	tools, err := mcpp.GetTools(context, &mcpp.Config{
		Cli: cli,
	})
	if err != nil {
		return nil, err
	}

	tools = append(tools, visionQueryTool())

	return tools, nil
}
