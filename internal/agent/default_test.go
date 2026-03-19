package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rvald/code-rig/internal/utils"
	"gopkg.in/yaml.v3"
)

func TestAgentConfig(t *testing.T) {

	t.Run("system template", func(t *testing.T) {
		cfg := AgentConfig{
			SystemTemplate: "You are helper.",
		}

		got := cfg.SystemTemplate
		expected := "You are helper."

		if got != expected {
			t.Errorf("got %q want %q", got, expected)
		}
	})

	t.Run("step limit", func(t *testing.T) {
		cfg := AgentConfig{
			SystemTemplate: "You are helper.",
			StepLimit:      0,
		}

		got := cfg.StepLimit
		expected := 0

		if got != expected {
			t.Errorf("got %d want %d", got, expected)
		}
	})

	t.Run("costlimit", func(t *testing.T) {
		cfg := AgentConfig{
			SystemTemplate: "You are helper.",
			StepLimit:      0,
			CostLimit:      0,
		}

		got := cfg.CostLimit
		expected := 0.0

		if got != expected {
			t.Errorf("got %f want %f", got, expected)
		}
	})

	t.Run("output path", func(t *testing.T) {
		cfg := AgentConfig{
			SystemTemplate: "You are helper.",
			StepLimit:      0,
			OutputPath:     "test/path",
		}

		got := cfg.OutputPath
		expected := "test/path"

		if got != expected {
			t.Errorf("got %q want %q", got, expected)
		}
	})
}

func TestMessage(t *testing.T) {
	t.Run("role", func(t *testing.T) {
		msg := Message{Role: "assistant"}
		got := msg.Role
		expected := "assistant"
		if got != expected {
			t.Errorf("got %q want %q", got, expected)
		}
	})

	t.Run("content", func(t *testing.T) {
		msg := Message{Role: "assistant", Content: "thinking"}
		got := msg.Content
		expected := "thinking"
		if got != expected {
			t.Errorf("got %q want %q", got, expected)
		}
	})

	t.Run("extra", func(t *testing.T) {
		msg := Message{Role: "assistant", Content: "thinking", Extra: map[string]any{"cost": 0.5}}
		got := msg.Extra
		expected, ok := msg.Extra["cost"].(float64)
		if !ok || 0.5 != expected {
			t.Errorf("got %v want %f", got, expected)
		}
	})

}

func TestAction(t *testing.T) {
	t.Run("command", func(t *testing.T) {
		action := Action{Command: "echo hello"}
		got := action.Command
		expected := "echo hello"
		if got != expected {
			t.Errorf("got %q want %q", got, expected)
		}
	})

	t.Run("tool call id", func(t *testing.T) {
		action := Action{Command: "echo hello", ToolCallID: "call_0"}
		got := action.ToolCallID
		expected := "call_0"
		if got != expected {
			t.Errorf("got %q want %q", got, expected)
		}
	})

}

func TestObservation(t *testing.T) {
	t.Run("output", func(t *testing.T) {
		obs := Observation{Output: "hello\n"}
		got := obs.Output
		expected := "hello\n"
		if got != expected {
			t.Errorf("got %q want %q", got, expected)
		}
	})

	t.Run("return code", func(t *testing.T) {
		obs := Observation{Output: "hello\n", ReturnCode: 0}
		got := obs.ReturnCode
		expected := 0
		if got != expected {
			t.Errorf("got %q want %d", got, expected)
		}
	})

	t.Run("exception info", func(t *testing.T) {
		obs := Observation{Output: "hello\n", ReturnCode: 0}
		got := obs.ExceptionInfo
		expected := ""
		if got != expected {
			t.Errorf("got %q want %q", got, expected)
		}
	})
}

func TestMockModel(t *testing.T) {
	var m Model = &MockModel{}
	if m == nil {
		t.Fatal("MockModel should satisfy Model interface")
	}
}

func TestMockEnv(t *testing.T) {
	var e Environment = &MockEnv{}
	if e == nil {
		t.Fatal("MockEnv should satisfy Environment interface")
	}
}

func TestInterruptAgentFlowCarriesMessages(t *testing.T) {
	exitMsg := Message{Role: "exit", Content: "done", Extra: map[string]any{"exit_status": "Submitted"}}
	err := &SubmittedError{
		InterruptAgentFlowError: InterruptAgentFlowError{
			Messages: []Message{exitMsg},
		},
	}

	// It should satisfy the error interface
	var e error = err
	if e.Error() == "" {
		t.Error("Error() should return a non-empty string")
	}

	// It should be detectable as an InterruptAgentFlow
	var flow *InterruptAgentFlowError
	if !errors.As(err, &flow) {
		t.Fatal("SubmittedError should be unwrappable as InterruptAgentFlowError")
	}
	if len(flow.Messages) != 1 {
		t.Errorf("Messages len = %d, want 1", len(flow.Messages))
	}
}

func TestRecursiveMerge(t *testing.T) {
	a := map[string]any{"x": 1, "nested": map[string]any{"a": 1, "b": 2}}
	b := map[string]any{"y": 2, "nested": map[string]any{"b": 99, "c": 3}}
	result := utils.RecursiveMerge(a, b)

	if result["x"] != 1 {
		t.Errorf("x = %v, want 1", result["x"])
	}
	if result["y"] != 2 {
		t.Errorf("y = %v, want 2", result["y"])
	}
	nested := result["nested"].(map[string]any)
	if nested["a"] != 1 {
		t.Errorf("nested.a = %v, want 1", nested["a"])
	}
	if nested["b"] != 99 {
		t.Errorf("nested.b = %v, want 99 (later dict wins)", nested["b"])
	}
	if nested["c"] != 3 {
		t.Errorf("nested.c = %v, want 3", nested["c"])
	}
}

func TestRecursiveMergeNilSafe(t *testing.T) {
	result := utils.RecursiveMerge(nil, map[string]any{"a": 1}, nil)
	if result["a"] != 1 {
		t.Errorf("a = %v, want 1", result["a"])
	}
}

func TestNewDefaultAgent(t *testing.T) {
	cfg := AgentConfig{SystemTemplate: "sys", InstanceTemplate: "inst"}
	agent := NewDefaultAgent(cfg, &MockModel{}, &MockEnv{})

	if agent == nil {
		t.Fatal("agent should not be nil")
	}
	if len(agent.messages) != 0 {
		t.Errorf("messages len = %d, want 0", len(agent.messages))
	}
	if agent.cost != 0 {
		t.Errorf("cost = %f, want 0", agent.cost)
	}
	if agent.nCalls != 0 {
		t.Errorf("nCalls = %d, want 0", agent.nCalls)
	}
}

func TestAddMessages(t *testing.T) {
	agent := NewDefaultAgent(AgentConfig{}, &MockModel{}, &MockEnv{})

	added := agent.addMessages(
		Message{Role: "system", Content: "hello"},
		Message{Role: "user", Content: "world"},
	)

	if len(agent.messages) != 2 {
		t.Errorf("messages len = %d, want 2", len(agent.messages))
	}
	if len(added) != 2 {
		t.Errorf("returned len = %d, want 2", len(added))
	}
	if agent.messages[0].Role != "system" {
		t.Errorf("messages[0].Role = %q, want 'system'", agent.messages[0].Role)
	}
}

func TestQueryHappyPath(t *testing.T) {
	model := &MockModel{
		Responses: []Message{{
			Role:    "assistant",
			Content: "I will run a command",
			Extra: map[string]any{
				"actions": []Action{{Command: "echo hi"}},
				"cost":    0.5,
			},
		}},
	}
	agent := NewDefaultAgent(AgentConfig{}, model, &MockEnv{})
	// Seed with initial messages (query expects some context)
	agent.addMessages(Message{Role: "system", Content: "sys"})

	msg, err := agent.query()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Role != "assistant" {
		t.Errorf("Role = %q, want 'assistant'", msg.Role)
	}
	if agent.nCalls != 1 {
		t.Errorf("nCalls = %d, want 1", agent.nCalls)
	}
	if agent.cost != 0.5 {
		t.Errorf("cost = %f, want 0.5", agent.cost)
	}
	// Message should be in trajectory
	if len(agent.messages) != 2 { // system + assistant
		t.Errorf("messages len = %d, want 2", len(agent.messages))
	}
}

func TestQueryStepLimitExceeded(t *testing.T) {
	model := &MockModel{Responses: []Message{
		{Role: "assistant", Content: "r1", Extra: map[string]any{"cost": 0.1}},
		{Role: "assistant", Content: "r2", Extra: map[string]any{"cost": 0.1}},
	}}
	agent := NewDefaultAgent(AgentConfig{StepLimit: 1}, model, &MockEnv{})
	agent.addMessages(Message{Role: "system", Content: "sys"})

	// First query should succeed
	_, err := agent.query()
	if err != nil {
		t.Fatalf("first query should succeed: %v", err)
	}

	// Second query should fail with LimitsExceeded
	_, err = agent.query()
	var limitsErr *LimitsExceededError
	if !errors.As(err, &limitsErr) {
		t.Errorf("expected LimitsExceededError, got %T: %v", err, err)
	}
	// Model should only have been called once
	if model.CallCount != 1 {
		t.Errorf("model called %d times, want 1", model.CallCount)
	}
}

func TestQueryCostLimitExceeded(t *testing.T) {
	model := &MockModel{Responses: []Message{
		{Role: "assistant", Content: "r1", Extra: map[string]any{"cost": 0.6}},
		{Role: "assistant", Content: "r2", Extra: map[string]any{"cost": 0.6}},
	}}
	agent := NewDefaultAgent(AgentConfig{CostLimit: 1.0}, model, &MockEnv{})
	agent.addMessages(Message{Role: "system", Content: "sys"})

	// First query succeeds (cost becomes 0.6)
	_, err := agent.query()
	if err != nil {
		t.Fatalf("first query should succeed: %v", err)
	}

	// Second query should fail (0.6 < 1.0 so it proceeds, cost becomes 1.2)
	// Actually: Python checks `0 < cost_limit <= self.cost` BEFORE querying.
	// After first call, cost=0.6. 0 < 1.0 <= 0.6 is false, so second call proceeds.
	_, err = agent.query()
	if err != nil {
		t.Fatalf("second query should succeed: %v", err)
	}

	// Third query: cost=1.2. 0 < 1.0 <= 1.2 is true → LimitsExceeded
	_, err = agent.query()
	var limitsErr *LimitsExceededError
	if !errors.As(err, &limitsErr) {
		t.Errorf("expected LimitsExceededError, got %T: %v", err, err)
	}
}

func TestExecuteActions(t *testing.T) {
	env := &MockEnv{Outputs: []Observation{{Output: "hello\n", ReturnCode: 0}}}
	model := &MockModel{}
	agent := NewDefaultAgent(AgentConfig{}, model, env)

	msg := Message{
		Role:    "assistant",
		Content: "running command",
		Extra: map[string]any{
			"actions": []Action{{Command: "echo hello"}},
		},
	}
	observations, err := agent.executeActions(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(observations) != 1 {
		t.Fatalf("observations len = %d, want 1", len(observations))
	}
	if observations[0].Role != "tool" {
		t.Errorf("Role = %q, want 'tool'", observations[0].Role)
	}
	if env.CallCount != 1 {
		t.Errorf("env called %d times, want 1", env.CallCount)
	}
	// Observation content should contain the output
	if !strings.Contains(observations[0].Content, "hello") {
		t.Errorf("observation should contain 'hello', got %q", observations[0].Content)
	}
}

func TestExecuteActionsEmpty(t *testing.T) {
	env := &MockEnv{}
	agent := NewDefaultAgent(AgentConfig{}, &MockModel{}, env)

	msg := Message{Role: "assistant", Content: "no actions", Extra: map[string]any{}}
	observations, err := agent.executeActions(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.CallCount != 0 {
		t.Errorf("env should not be called, was called %d times", env.CallCount)
	}
	if len(observations) != 0 {
		t.Errorf("observations len = %d, want 0", len(observations))
	}
}

func TestStep(t *testing.T) {
	model := &MockModel{Responses: []Message{{
		Role:    "assistant",
		Content: "I'll echo",
		Extra:   map[string]any{"actions": []Action{{Command: "echo hi"}}, "cost": 0.1},
	}}}
	env := &MockEnv{Outputs: []Observation{{Output: "hi\n", ReturnCode: 0}}}
	agent := NewDefaultAgent(AgentConfig{}, model, env)
	agent.addMessages(Message{Role: "system", Content: "sys"}, Message{Role: "user", Content: "task"})

	err := agent.step()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 4 messages: system, user, assistant, observation
	if len(agent.messages) != 4 {
		t.Errorf("messages len = %d, want 4", len(agent.messages))
	}
	if agent.messages[2].Role != "assistant" {
		t.Errorf("messages[2].Role = %q, want 'assistant'", agent.messages[2].Role)
	}
	if agent.messages[3].Role != "tool" {
		t.Errorf("messages[3].Role = %q, want 'tool'", agent.messages[3].Role)
	}
}

func TestRunCompletesOnSubmission(t *testing.T) {
	model := &MockModel{Responses: []Message{{
		Role:    "assistant",
		Content: "finishing",
		Extra:   map[string]any{"actions": []Action{{Command: "echo done"}}, "cost": 0.1},
	}}}
	env := &MockEnv{Outputs: []Observation{{
		Output:     "COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT\nmy submission\n",
		ReturnCode: 0,
	}}}

	cfg := AgentConfig{
		SystemTemplate:   "You are a helper.",
		InstanceTemplate: "Task: {{.Task}}",
	}
	agent := NewDefaultAgent(cfg, model, env)
	result, err := agent.Run("fix a bug")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitStatus != "Submitted" {
		t.Errorf("ExitStatus = %q, want 'Submitted'", result.ExitStatus)
	}
	if result.Submission != "my submission\n" {
		t.Errorf("Submission = %q, want 'my submission\\n'", result.Submission)
	}
}

func TestSerialize(t *testing.T) {
	agent := NewDefaultAgent(AgentConfig{SystemTemplate: "sys", InstanceTemplate: "inst"}, &MockModel{}, &MockEnv{})
	agent.addMessages(Message{Role: "system", Content: "hello"})
	agent.cost = 1.5
	agent.nCalls = 3

	data := agent.serialize()
	info, ok := data["info"].(map[string]any)
	if !ok {
		t.Fatal("info should be a map")
	}
	stats := info["model_stats"].(map[string]any)
	if stats["instance_cost"] != 1.5 {
		t.Errorf("instance_cost = %v, want 1.5", stats["instance_cost"])
	}
	if stats["api_calls"] != 3 {
		t.Errorf("api_calls = %v, want 3", stats["api_calls"])
	}
}

func TestSaveCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trajectory.json")

	agent := NewDefaultAgent(
		AgentConfig{OutputPath: path, SystemTemplate: "s", InstanceTemplate: "i"},
		&MockModel{}, &MockEnv{},
	)
	agent.addMessages(Message{Role: "system", Content: "hello"})

	agent.Save(path)

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	var data map[string]any
	if err := json.Unmarshal(contents, &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if data["trajectory_format"] != "mini-swe-agent-1.1" {
		t.Errorf("trajectory_format = %v, want 'mini-swe-agent-1.1'", data["trajectory_format"])
	}
}

// Mocks ====================================================================

type MockModel struct {
	Responses []Message
	CallCount int
}

func (m *MockModel) Query(messages []Message) (Message, error) {
	if m.CallCount >= len(m.Responses) {
		return Message{}, fmt.Errorf("no more responses")
	}
	resp := m.Responses[m.CallCount]
	m.CallCount++
	return resp, nil
}

func (m *MockModel) FormatMessage(role, content string, extra map[string]any) Message {
	return Message{Role: role, Content: content, Extra: extra}
}

func (m *MockModel) FormatObservationMessages(message Message, outputs []Observation) []Message {
	var msgs []Message
	for _, obs := range outputs {
		msgs = append(msgs, Message{
			Role:    "tool",
			Content: fmt.Sprintf("<returncode>%d</returncode>\n<output>\n%s</output>", obs.ReturnCode, obs.Output),
			Extra:   map[string]any{"returncode": obs.ReturnCode},
		})
	}
	return msgs
}

func (m *MockModel) GetTemplateVars() map[string]any { return nil }
func (m *MockModel) Serialize() map[string]any       { return nil }

// ---

type MockEnv struct {
	Outputs   []Observation
	CallCount int
}

func (e *MockEnv) Execute(action Action) (Observation, error) {
	if e.CallCount >= len(e.Outputs) {
		return Observation{}, fmt.Errorf("no more outputs")
	}
	obs := e.Outputs[e.CallCount]
	e.CallCount++

	// Detect submission sentinel in the output
	const sentinel = "COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT"
	if idx := strings.Index(obs.Output, sentinel); idx >= 0 {
		submission := obs.Output[idx+len(sentinel):]
		submission = strings.TrimPrefix(submission, "\n")
		return Observation{}, NewSubmittedError(submission)
	}

	return obs, nil
}

func (e *MockEnv) GetTemplateVars() map[string]any { return nil }
func (e *MockEnv) Serialize() map[string]any       { return nil }

func TestBuildAgentConfig(t *testing.T) {
	raw := map[string]any{
		"system_template":   "You are a helper.",
		"instance_template": "Task: {{.Task}}",
		"step_limit":        5,
		"cost_limit":        2.0,
	}
	cfg, err := BuildAgentConfigFromRawMap(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SystemTemplate != "You are a helper." {
		t.Errorf("SystemTemplate = %q, want 'You are a helper.'", cfg.SystemTemplate)
	}
	if cfg.StepLimit != 5 {
		t.Errorf("StepLimit = %d, want 5", cfg.StepLimit)
	}
	if cfg.CostLimit != 2.0 {
		t.Errorf("CostLimit = %f, want 2.0", cfg.CostLimit)
	}
}

func TestBuildAgentConfigMissingRequired(t *testing.T) {
	raw := map[string]any{"step_limit": 5}
	cfg, err := BuildAgentConfigFromRawMap(raw)
	if err != nil {
		t.Fatalf("BuildAgentConfigFromRawMap itself shouldn't error: %v", err)
	}
	err = ValidateAgentConfig(cfg)
	if err == nil {
		t.Error("expected validation error for missing system_template and instance_template")
	}
}

func TestAgentConfigYAMLRoundtrip(t *testing.T) {
	input := `
system_template: "hello"
instance_template: "world"
step_limit: 5
cost_limit: 2.0
output_path: "/tmp/out.json"
`
	var cfg AgentConfig
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if cfg.SystemTemplate != "hello" {
		t.Errorf("SystemTemplate = %q, want 'hello'", cfg.SystemTemplate)
	}
	if cfg.StepLimit != 5 {
		t.Errorf("StepLimit = %d, want 5", cfg.StepLimit)
	}
}
