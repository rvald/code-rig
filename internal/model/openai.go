package model

import (
	"context"
	"fmt"

	"github.com/rvald/code-rig/internal/agent"
	"github.com/sashabaranov/go-openai"
)

type OpenAIModel struct {
	config ModelConfig
	client *openai.Client
}

func NewOpenAIModel(cfg ModelConfig, apiKey string) *OpenAIModel {
	client := openai.NewClient(apiKey)
	return &OpenAIModel{
		config: cfg,
		client: client,
	}
}

func (m *OpenAIModel) FormatMessage(role, content string, extra map[string]any) agent.Message {
	return agent.Message{
		Role:    role,
		Content: content,
		Extra:   extra,
	}
}

var bashTool = openai.Tool{
	Type: openai.ToolTypeFunction,
	Function: &openai.FunctionDefinition{
		Name:        "bash",
		Description: "Execute a bash command",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The bash command to execute",
				},
			},
			"required": []string{"command"},
		},
	},
}

func (m *OpenAIModel) Query(messages []agent.Message) (agent.Message, error) {
	var oaiMessages []openai.ChatCompletionMessage
	for _, msg := range messages {
		oaiMsg := openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
		if msg.Role == "tool" {
			if id, ok := msg.Extra["tool_call_id"].(string); ok {
				oaiMsg.ToolCallID = id
			}
		}
		oaiMessages = append(oaiMessages, oaiMsg)
	}

	req := openai.ChatCompletionRequest{
		Model:    m.config.ModelName,
		Messages: oaiMessages,
		Tools:    []openai.Tool{bashTool},
	}

	resp, err := m.client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		return agent.Message{}, fmt.Errorf("API query failed: %w", err)
	}

	choice := resp.Choices[0]

	actions, err := parseToolCallActions(choice.Message.ToolCalls, m.config.FormatErrorTemplate)
	if err != nil {
		return agent.Message{}, err
	}

	cost := float64(resp.Usage.TotalTokens) * 0.0001

	return agent.Message{
		Role:    choice.Message.Role,
		Content: choice.Message.Content,
		Extra: map[string]any{
			"actions": actions,
			"cost":    cost,
		},
	}, nil
}

func (m *OpenAIModel) GetTemplateVars() map[string]any {
	return nil
}

func (m *OpenAIModel) Serialize() map[string]any {
	return nil
}

