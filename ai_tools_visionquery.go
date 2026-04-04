package main

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

func visionQueryTool() tool.InvokableTool {
	info := &schema.ToolInfo{
		Name: "vision-query",
		Desc: "Invokable tool to query vision LLM model",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Desc:     "User prompt",
				Type:     schema.String,
				Required: true,
			},
			"base64": {
				Desc:     "Base64 of image",
				Type:     schema.String,
				Required: true,
			},
			"mimetype": {
				Desc:     "mimetype of image",
				Type:     schema.String,
				Required: true,
			},
		}),
	}

	return utils.NewTool(info, visionQuery)
}

type visionQueryToolParams struct {
	Query    string `json:"query"`
	Base64   string `json:"base64"`
	MimeType string `json:"mimetype"`
}

func visionQuery(ctx context.Context, params *visionQueryToolParams) (string, error) {
	b64, ok := ctx.Value("b64").(string)
	if !ok {
		b64 = params.Base64
	}
	mimeType, ok := ctx.Value("mimeType").(string)
	if !ok {
		mimeType = params.Base64
	}
	resp, err := AIVisionModel.Generate(ctx, []*schema.Message{{
		Role: schema.User,
		UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: params.Query},
			{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{
					Base64Data: toPtr(b64),
					MIMEType:   mimeType,
				},
			}},
		},
	}})
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}
