package model

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rvald/code-rig/internal/agent"
	"github.com/sashabaranov/go-openai"
)

func TestModelConfigDefaults(t *testing.T) {
	cfg := ModelConfig{
		ModelName: "gpt-4",
	}
	if cfg.ModelName != "gpt-4" {
		t.Errorf("ModelName = %q, want 'gpt-4'", cfg.ModelName)
	}
	if cfg.ModelKwargs != nil {
		t.Errorf("ModelKwargs should be nil or empty map initially")
	}
}

func TestNewOpenAIModel(t *testing.T) {
	cfg := ModelConfig{ModelName: "gpt-4"}
	model := NewOpenAIModel(cfg, "fake-api-key")
	if model == nil {
		t.Fatal("expected non-nil model")
	}
	if model.config.ModelName != "gpt-4" {
		t.Errorf("model name not stored")
	}
}

func TestParseToolCallsHappyPath(t *testing.T) {
	toolCalls := []openai.ToolCall{
		{
			ID: "call_123",
			Function: openai.FunctionCall{
				Name:      "bash",
				Arguments: `{"command": "echo hi"}`,
			},
		},
	}

	actions, err := parseToolCallActions(toolCalls, "Format error: {{.Error}}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Command != "echo hi" {
		t.Errorf("Command = %q, want 'echo hi'", actions[0].Command)
	}
	if actions[0].ToolCallID != "call_123" {
		t.Errorf("ToolCallID = %q, want 'call_123'", actions[0].ToolCallID)
	}
}

func TestParseToolCallsMalformed(t *testing.T) {
	tests := []struct {
		name  string
		calls []openai.ToolCall
	}{
		{"empty", []openai.ToolCall{}},
		{"wrong tool", []openai.ToolCall{{Function: openai.FunctionCall{Name: "python", Arguments: `{}`}}}},
		{"missing command", []openai.ToolCall{{Function: openai.FunctionCall{Name: "bash", Arguments: `{"foo": "bar"}`}}}},
		{"bad json", []openai.ToolCall{{Function: openai.FunctionCall{Name: "bash", Arguments: `{bad`}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseToolCallActions(tt.calls, "Error: {{.Error}}")
			var formatErr *agent.FormatError
			if !errors.As(err, &formatErr) {
				t.Errorf("expected FormatError, got %T", err)
			}
		})
	}
}

func TestFormatMessage(t *testing.T) {
	model := NewOpenAIModel(ModelConfig{}, "")
	msg := model.FormatMessage("user", "hello", map[string]any{"flag": true})

	if msg.Role != "user" {
		t.Errorf("Role = %q, want 'user'", msg.Role)
	}
	if msg.Content != "hello" {
		t.Errorf("Content = %q, want 'hello'", msg.Content)
	}
	if msg.Extra["flag"] != true {
		t.Errorf("Extra['flag'] = %v, want true", msg.Extra["flag"])
	}
}

func TestFormatObservationMessages(t *testing.T) {
	model := NewOpenAIModel(ModelConfig{
		ObservationTemplate: "Output: {{.Output.Output}}, Code: {{.Output.ReturnCode}}",
	}, "")

	msg := agent.Message{
		Extra: map[string]any{
			"actions": []agent.Action{
				{Command: "echo hi", ToolCallID: "call_1"},
			},
		},
	}
	outputs := []agent.Observation{
		{Output: "hi", ReturnCode: 0},
	}

	obsMsgs := model.FormatObservationMessages(msg, outputs)

	if len(obsMsgs) != 1 {
		t.Fatalf("expected 1 obs message, got %d", len(obsMsgs))
	}
	if obsMsgs[0].Role != "tool" {
		t.Errorf("Role = %q, want 'tool'", obsMsgs[0].Role)
	}
	if obsMsgs[0].Content != "Output: hi, Code: 0" {
		t.Errorf("Content = %q, want formatted string", obsMsgs[0].Content)
	}
	if obsMsgs[0].Extra["tool_call_id"] != "call_1" {
		t.Errorf("tool_call_id missing from Extra")
	}
}

func TestQueryAPICall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{"choices": [{"message": {"role": "assistant", "content": "thinking", "tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "bash", "arguments": "{\"command\":\"ls\"}"}}]}}], "usage": {"total_tokens": 100}}`
		w.Write([]byte(response))
	}))
	defer server.Close()

	cfg := openai.DefaultConfig("test")
	cfg.BaseURL = server.URL + "/v1"
	client := openai.NewClientWithConfig(cfg)

	model := &OpenAIModel{
		config: ModelConfig{ModelName: "test-model"},
		client: client,
	}

	msg, err := model.Query([]agent.Message{{Role: "user", Content: "do it"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions := msg.Extra["actions"].([]agent.Action)
	if len(actions) != 1 || actions[0].Command != "ls" {
		t.Errorf("actions not parsed correctly")
	}
	if msg.Content != "thinking" {
		t.Errorf("Content = %q, want 'thinking'", msg.Content)
	}
	if msg.Extra["cost"] != 0.01 { // 100 tokens * 0.0001
		t.Errorf("cost = %v, want 0.01", msg.Extra["cost"])
	}
}
