package agent

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestInteractiveConfigDefaults(t *testing.T) {
	// Mode should default to "confirm" in the constructor or config parser
	cfg := InteractiveAgentConfig{
		AgentConfig: AgentConfig{SystemTemplate: "sys"},
		Mode:        "confirm",
		ConfirmExit: true,
	}
	if cfg.Mode != "confirm" {
		t.Errorf("Mode = %q, want 'confirm'", cfg.Mode)
	}
	if len(cfg.WhitelistActions) != 0 {
		t.Error("WhitelistActions should be empty")
	}
}

func TestNewInteractiveAgentIsDefaultAgent(t *testing.T) {
	cfg := InteractiveAgentConfig{Mode: "confirm"}
	agent := NewInteractiveAgent(cfg, &MockModel{}, &MockEnv{})

	if agent == nil {
		t.Fatal("expected non-nil agent")
	}
	// It should have access to DefaultAgent fields/methods
	if agent.cost != 0.0 {
		t.Errorf("expected inherited cost to be 0")
	}
}

func TestInteractiveAddMessagesPrints(t *testing.T) {
	var buf bytes.Buffer
	agent := NewInteractiveAgent(InteractiveAgentConfig{}, &MockModel{}, &MockEnv{})
	agent.Stdout = &buf // Assuming you add this field for testing

	msg := Message{Role: "user", Content: "hello world"}
	agent.addMessages(msg) // We must ensure this calls the *InteractiveAgent* method, not the embedded one

	if len(agent.messages) != 1 {
		t.Errorf("message not appended to trajectory")
	}

	output := buf.String()
	if !strings.Contains(output, "user:") || !strings.Contains(output, "hello world") {
		t.Errorf("output did not contain expected formatting, got %q", output)
	}
}

func TestQueryHumanMode(t *testing.T) {
	var inBuf, outBuf bytes.Buffer
	inBuf.WriteString("ls -la\n") // User types this

	agent := NewInteractiveAgent(InteractiveAgentConfig{Mode: "human"}, &MockModel{}, &MockEnv{})
	agent.Stdin = &inBuf
	agent.Stdout = &outBuf

	msg, err := agent.query()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// It should NOT have called the MockModel
	if agent.nCalls != 0 {
		t.Errorf("nCalls = %d, want 0 (human mode skips LLM)", agent.nCalls)
	}

	// It should have built a synthetic message
	if msg.Role != "user" {
		t.Errorf("Role = %q, want 'user'", msg.Role)
	}
	actions, ok := msg.Extra["actions"].([]Action)
	if !ok || len(actions) != 1 || actions[0].Command != "ls -la" {
		t.Errorf("actions = %v, want [{Command: 'ls -la'}]", actions)
	}
}

func TestExecuteActionsConfirmModeReject(t *testing.T) {
	var inBuf bytes.Buffer
	inBuf.WriteString("no, do something else\n") // User rejects

	env := &MockEnv{}
	agent := NewInteractiveAgent(InteractiveAgentConfig{Mode: "confirm"}, &MockModel{}, env)
	agent.Stdin = &inBuf

	msg := Message{Extra: map[string]any{"actions": []Action{{Command: "rm -rf /"}}}}

	err := agent.executeActions(msg)

	var flowErr *InterruptAgentFlowError
	if !errors.As(err, &flowErr) {
		t.Fatalf("expected InterruptAgentFlowError from rejection, got %T: %v", err, err)
	}
	if env.CallCount != 0 {
		t.Error("environment should not execute rejected commands")
	}
	if flowErr.Messages[0].Extra["interrupt_type"] != "UserRejection" {
		t.Errorf("interrupt_type = %v", flowErr.Messages[0].Extra["interrupt_type"])
	}
}
