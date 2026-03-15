package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rvald/code-rig/internal/agent"
	"github.com/rvald/code-rig/internal/config"
	"github.com/rvald/code-rig/internal/environment"
	"github.com/rvald/code-rig/internal/model"
)

func TestParseFlagsToConfigOverride(t *testing.T) {
	app := NewApp()
	cmd := app.Command()

	// Simulate command line arguments passed to the root command setup by Cobra
	cmd.SetArgs([]string{
		"--task", "Fix the bug",
		"--model", "gpt-4",
		"--cost-limit", "5.0",
		"--config", "test.yaml",
	})

	// To avoid executing a real Run, we hijack it or just execute parsing
	// but Execute() runs PreRun, Run, etc. Since we just want flag parsing:
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if app.Task != "Fix the bug" {
		t.Errorf("Task = %q, want 'Fix the bug'", app.Task)
	}
	if len(app.ConfigFiles) != 1 || app.ConfigFiles[0] != "test.yaml" {
		t.Errorf("ConfigFiles = %v, want ['test.yaml']", app.ConfigFiles)
	}

	override := app.GetCLIOverrideConfig()

	agent := override.Agent
	if agent["cost_limit"] != 5.0 {
		t.Errorf("Agent cost_limit = %v, want 5.0", agent["cost_limit"])
	}

	model := override.Model
	if model["model_name"] != "gpt-4" {
		t.Errorf("Model model_name = %v, want 'gpt-4'", model["model_name"])
	}
}

func TestBuildFinalConfig(t *testing.T) {
	app := NewApp()
	app.ConfigFiles = []string{"agent.step_limit=10"} // valid key-value spec
	app.CostLimit = 5.0

	finalConfig, err := app.BuildFinalConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if finalConfig.Agent["step_limit"] != float64(10) {
		t.Errorf("step_limit from spec = %v, want 10", finalConfig.Agent["step_limit"])
	}
	if finalConfig.Agent["cost_limit"] != 5.0 {
		t.Errorf("cost_limit from CLI flag = %v, want 5.0", finalConfig.Agent["cost_limit"])
	}
}

func TestAssembleAgent(t *testing.T) {
	rawCfg := config.RawConfig{
		Agent: map[string]any{
			"system_template":   "sys",
			"instance_template": "inst",
		},
		Model: map[string]any{
			"model_name": "test-mock",
		},
	}

	app := NewApp()

	ag, err := app.AssembleAgent(rawCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ag == nil {
		t.Fatal("expected non-nil agent")
	}
}

func TestPromptForTask(t *testing.T) {
	app := NewApp()
	input := "Fix the login bug\nEOF\n"
	r := strings.NewReader(input)

	task, err := app.GetTask(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task != "Fix the login bug\nEOF" { // adjusted for trimspace
		t.Errorf("Task = %q", task)
	}
}

func TestAppRunEndToEnd(t *testing.T) {
	app := NewApp()
	app.Task = "test task"
	app.ModelFactory = func(cfg model.ModelConfig) agent.Model {
		return &MockModel{Responses: []agent.Message{{Role: "assistant", Extra: map[string]any{"actions": []agent.Action{{Command: "echo hi"}}}}}}
	}
	app.EnvFactory = func(cfg environment.LocalEnvironmentConfig) agent.Environment {
		return &MockEnv{Outputs: []agent.Observation{{Output: "COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT\ndone\n", ReturnCode: 0}}}
	}

	app.ConfigFiles = []string{"agent.system_template=sys", "agent.instance_template=inst", "model.model_name=mock"}

	// We need to bypass reading from stdin since we hardcoded Task
	err := app.Run(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Mocks

type MockModel struct {
	Responses []agent.Message
	CallCount int
}

func (m *MockModel) Query(messages []agent.Message) (agent.Message, error) {
	if m.CallCount >= len(m.Responses) {
		return agent.Message{}, fmt.Errorf("no more responses")
	}
	resp := m.Responses[m.CallCount]
	m.CallCount++
	return resp, nil
}

func (m *MockModel) FormatMessage(role, content string, extra map[string]any) agent.Message {
	return agent.Message{Role: role, Content: content, Extra: extra}
}

func (m *MockModel) FormatObservationMessages(message agent.Message, outputs []agent.Observation) []agent.Message {
	return []agent.Message{}
}

func (m *MockModel) GetTemplateVars() map[string]any { return nil }
func (m *MockModel) Serialize() map[string]any       { return nil }

type MockEnv struct {
	Outputs   []agent.Observation
	CallCount int
}

func (e *MockEnv) Execute(action agent.Action) (agent.Observation, error) {
	if e.CallCount >= len(e.Outputs) {
		return agent.Observation{}, fmt.Errorf("no more outputs")
	}
	obs := e.Outputs[e.CallCount]
	e.CallCount++

	const sentinel = "COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT"
	if idx := strings.Index(obs.Output, sentinel); idx >= 0 {
		submission := obs.Output[idx+len(sentinel):]
		submission = strings.TrimPrefix(submission, "\n")
		return agent.Observation{}, agent.NewSubmittedError(submission)
	}

	return obs, nil
}

func (e *MockEnv) GetTemplateVars() map[string]any { return nil }
func (e *MockEnv) Serialize() map[string]any       { return nil }
