package model

import (
	"encoding/json"
	"fmt"

	"github.com/rvald/code-rig/internal/agent"
	"github.com/rvald/code-rig/internal/utils"
	"github.com/sashabaranov/go-openai"
)

func parseToolCallActions(toolCalls []openai.ToolCall, errorTemplate string) ([]agent.Action, error) {
	if len(toolCalls) == 0 {
		return nil, newFormatError("No tool calls found in the response. Every response MUST include at least one tool call.", errorTemplate)
	}

	var actions []agent.Action
	for _, tc := range toolCalls {
		errorMsg := ""
		if tc.Function.Name != "bash" {
			errorMsg += fmt.Sprintf("Unknown tool '%s'. ", tc.Function.Name)
		}

		var args map[string]any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			errorMsg += fmt.Sprintf("Error parsing tool call arguments: %v. ", err)
		}

		cmdStr, ok := args["command"].(string)
		if !ok {
			errorMsg += "Missing 'command' argument in bash tool call. "
		}

		if errorMsg != "" {
			return nil, newFormatError(errorMsg, errorTemplate)
		}

		actions = append(actions, agent.Action{
			Command:    cmdStr,
			ToolCallID: tc.ID,
		})
	}
	return actions, nil
}

func newFormatError(errMsg string, tmpl string) error {
	rendered, _ := utils.RenderTemplate(tmpl, map[string]any{"Error": errMsg})
	msg := agent.Message{
		Role:    "user",
		Content: rendered,
		Extra:   map[string]any{"interrupt_type": "FormatError"},
	}
	return &agent.FormatError{
		InterruptAgentFlowError: agent.InterruptAgentFlowError{
			Messages: []agent.Message{msg},
		},
	}
}

func (m *OpenAIModel) FormatObservationMessages(message agent.Message, outputs []agent.Observation) []agent.Message {
	var rawActions any
	if message.Extra != nil {
		rawActions = message.Extra["actions"]
	}
	actions, ok := rawActions.([]agent.Action)
	if !ok || len(actions) == 0 {
		return nil
	}

	var results []agent.Message
	for i, action := range actions {
		var obs agent.Observation
		if i < len(outputs) {
			obs = outputs[i]
		} else {
			obs = agent.Observation{ReturnCode: -1, ExceptionInfo: "action was not executed"}
		}

		content, _ := utils.RenderTemplate(m.config.ObservationTemplate, map[string]any{
			"Output": obs,
		})

		results = append(results, agent.Message{
			Role:    "tool",
			Content: content,
			Extra: map[string]any{
				"tool_call_id": action.ToolCallID,
				"returncode":   obs.ReturnCode,
			},
		})
	}
	return results
}

