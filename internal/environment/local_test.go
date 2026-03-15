package environment

import (
	"errors"
	"strings"
	"testing"

	"github.com/rvald/code-rig/internal/agent"
)

// --- Phase 1: Config ---

func TestLocalEnvironmentConfigDefaults(t *testing.T) {
	cfg := LocalEnvironmentConfig{}
	if cfg.Cwd != "" {
		t.Errorf("Cwd = %q, want empty", cfg.Cwd)
	}
	if cfg.Env != nil {
		t.Error("Env should be nil for a zero-value struct (constructor initializes it)")
	}
	if cfg.Timeout != 0 {
		t.Errorf("Timeout = %d, want 0 (constructor sets default)", cfg.Timeout)
	}
}

// --- Phase 2: Constructor ---

func TestNewLocalEnvironment(t *testing.T) {
	env := NewLocalEnvironment(LocalEnvironmentConfig{})
	if env == nil {
		t.Fatal("env should not be nil")
	}
	if env.config.Timeout != 30 {
		t.Errorf("default Timeout = %d, want 30", env.config.Timeout)
	}
	if env.config.Env == nil {
		t.Error("Env map should be initialized")
	}
}

func TestNewLocalEnvironmentCustomConfig(t *testing.T) {
	cfg := LocalEnvironmentConfig{
		Cwd:     "/tmp",
		Timeout: 10,
		Env:     map[string]string{"FOO": "bar"},
	}
	env := NewLocalEnvironment(cfg)
	if env.config.Cwd != "/tmp" {
		t.Errorf("Cwd = %q, want '/tmp'", env.config.Cwd)
	}
	if env.config.Timeout != 10 {
		t.Errorf("Timeout = %d, want 10", env.config.Timeout)
	}
	if env.config.Env["FOO"] != "bar" {
		t.Errorf("Env[FOO] = %q, want 'bar'", env.config.Env["FOO"])
	}
}

// --- Phase 3: Execute — Happy Path ---

func TestExecuteEchoCommand(t *testing.T) {
	env := NewLocalEnvironment(LocalEnvironmentConfig{})
	action := Action{Command: "echo hello"}

	obs, err := env.Execute(action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(obs.Output) != "hello" {
		t.Errorf("Output = %q, want 'hello'", obs.Output)
	}
	if obs.ReturnCode != 0 {
		t.Errorf("ReturnCode = %d, want 0", obs.ReturnCode)
	}
	if obs.ExceptionInfo != "" {
		t.Errorf("ExceptionInfo = %q, want empty", obs.ExceptionInfo)
	}
}

func TestExecuteWithCustomCwd(t *testing.T) {
	dir := t.TempDir()
	env := NewLocalEnvironment(LocalEnvironmentConfig{Cwd: dir})
	action := Action{Command: "pwd"}

	obs, err := env.Execute(action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(obs.Output)
	if got != dir {
		t.Errorf("pwd output = %q, want %q", got, dir)
	}
}

func TestExecuteWithCustomEnvVars(t *testing.T) {
	env := NewLocalEnvironment(LocalEnvironmentConfig{
		Env: map[string]string{"MY_TEST_VAR": "hello_from_config"},
	})
	action := Action{Command: "echo $MY_TEST_VAR"}

	obs, err := env.Execute(action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(obs.Output) != "hello_from_config" {
		t.Errorf("Output = %q, want 'hello_from_config'", obs.Output)
	}
}

// --- Phase 4: Execute — Error Cases ---

func TestExecuteNonzeroExitCode(t *testing.T) {
	env := NewLocalEnvironment(LocalEnvironmentConfig{})
	action := Action{Command: "exit 42"}

	obs, err := env.Execute(action)
	if err != nil {
		t.Fatalf("unexpected error (should not propagate): %v", err)
	}
	if obs.ReturnCode != 42 {
		t.Errorf("ReturnCode = %d, want 42", obs.ReturnCode)
	}
	if obs.ExceptionInfo != "" {
		t.Errorf("ExceptionInfo = %q, want empty (nonzero exit is not an exception)", obs.ExceptionInfo)
	}
}

func TestExecuteStderrMergedIntoOutput(t *testing.T) {
	env := NewLocalEnvironment(LocalEnvironmentConfig{})
	action := Action{Command: "echo 'to stdout' && echo 'to stderr' >&2"}

	obs, err := env.Execute(action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(obs.Output, "to stdout") {
		t.Errorf("Output should contain 'to stdout', got %q", obs.Output)
	}
	if !strings.Contains(obs.Output, "to stderr") {
		t.Errorf("Output should contain 'to stderr' (merged from stderr), got %q", obs.Output)
	}
}

func TestExecuteTimeout(t *testing.T) {
	env := NewLocalEnvironment(LocalEnvironmentConfig{Timeout: 1})
	action := Action{Command: "sleep 30"}

	obs, err := env.Execute(action)
	if err != nil {
		t.Fatalf("unexpected error (timeout should not propagate): %v", err)
	}
	if obs.ReturnCode != -1 {
		t.Errorf("ReturnCode = %d, want -1", obs.ReturnCode)
	}
	if obs.ExceptionInfo == "" {
		t.Error("ExceptionInfo should describe the timeout")
	}
}

func TestExecuteCommandNotFound(t *testing.T) {
	env := NewLocalEnvironment(LocalEnvironmentConfig{})
	action := Action{Command: "nonexistent_command_xyz_12345"}

	obs, err := env.Execute(action)
	if err != nil {
		t.Fatalf("unexpected error (should be packed into observation): %v", err)
	}
	if obs.ReturnCode != 127 {
		t.Errorf("ReturnCode = %d, want 127", obs.ReturnCode)
	}
}

// --- Phase 5: Submission Detection ---

func TestCheckFinishedDetectsSubmission(t *testing.T) {
	env := NewLocalEnvironment(LocalEnvironmentConfig{})
	action := Action{Command: `printf "COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT\nmy answer\n"`}

	_, err := env.Execute(action)

	var submitted *SubmittedError
	if !errors.As(err, &submitted) {
		t.Fatalf("expected SubmittedError, got %T: %v", err, err)
	}
	if submitted.Submission != "my answer\n" {
		t.Errorf("Submission = %q, want %q", submitted.Submission, "my answer\n")
	}
	if submitted.ExitStatus != "Submitted" {
		t.Errorf("ExitStatus = %q, want 'Submitted'", submitted.ExitStatus)
	}
}

func TestCheckFinishedIgnoresNonzeroExitCode(t *testing.T) {
	env := NewLocalEnvironment(LocalEnvironmentConfig{})
	action := Action{Command: `printf "COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT\nmy answer\n" && exit 1`}

	obs, err := env.Execute(action)
	if err != nil {
		t.Fatalf("should not return error for failed command with sentinel: %v", err)
	}
	if obs.ReturnCode == 0 {
		t.Error("ReturnCode should be nonzero")
	}
}

func TestCheckFinishedPassesThroughNormalOutput(t *testing.T) {
	env := NewLocalEnvironment(LocalEnvironmentConfig{})
	action := Action{Command: "echo 'just a normal command'"}

	obs, err := env.Execute(action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs.ReturnCode != 0 {
		t.Errorf("ReturnCode = %d, want 0", obs.ReturnCode)
	}
	if !strings.Contains(obs.Output, "just a normal command") {
		t.Errorf("Output = %q, should contain 'just a normal command'", obs.Output)
	}
}

func TestCheckFinishedMultilineSubmission(t *testing.T) {
	env := NewLocalEnvironment(LocalEnvironmentConfig{})
	action := Action{Command: `printf "COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT\nline1\nline2\nline3\n"`}

	_, err := env.Execute(action)

	var submitted *SubmittedError
	if !errors.As(err, &submitted) {
		t.Fatalf("expected SubmittedError, got %T: %v", err, err)
	}
	expected := "line1\nline2\nline3\n"
	if submitted.Submission != expected {
		t.Errorf("Submission = %q, want %q", submitted.Submission, expected)
	}
}

func TestCheckFinishedLeadingWhitespace(t *testing.T) {
	env := NewLocalEnvironment(LocalEnvironmentConfig{})
	action := Action{Command: `printf "\n  COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT\nmy answer\n"`}

	_, err := env.Execute(action)

	var submitted *SubmittedError
	if !errors.As(err, &submitted) {
		t.Fatalf("expected SubmittedError even with leading whitespace, got %T: %v", err, err)
	}
	if submitted.Submission != "my answer\n" {
		t.Errorf("Submission = %q, want %q", submitted.Submission, "my answer\n")
	}
}

// --- Phase 6: GetTemplateVars ---

func TestGetTemplateVars(t *testing.T) {
	env := NewLocalEnvironment(LocalEnvironmentConfig{Cwd: "/tmp", Timeout: 10})
	vars := env.GetTemplateVars()

	if vars["cwd"] != "/tmp" {
		t.Errorf("cwd = %v, want '/tmp'", vars["cwd"])
	}
	if vars["timeout"] != 10 {
		t.Errorf("timeout = %v, want 10", vars["timeout"])
	}
	if _, ok := vars["system"]; !ok {
		t.Error("should have 'system' key from platform info")
	}
	if _, ok := vars["machine"]; !ok {
		t.Error("should have 'machine' key from platform info")
	}
}

// --- Phase 7: Serialize ---

func TestSerialize(t *testing.T) {
	env := NewLocalEnvironment(LocalEnvironmentConfig{Cwd: "/tmp", Timeout: 10})
	data := env.Serialize()

	info, ok := data["info"].(map[string]any)
	if !ok {
		t.Fatal("data should have 'info' key with map value")
	}
	config, ok := info["config"].(map[string]any)
	if !ok {
		t.Fatal("info should have 'config' key with map value")
	}
	envConfig, ok := config["environment"].(map[string]any)
	if !ok {
		t.Fatal("config should have 'environment' key with map value")
	}
	if envConfig["cwd"] != "/tmp" {
		t.Errorf("cwd = %v, want '/tmp'", envConfig["cwd"])
	}
	if envConfig["timeout"] != 10 {
		t.Errorf("timeout = %v, want 10", envConfig["timeout"])
	}
	envType, ok := config["environment_type"].(string)
	if !ok || envType == "" {
		t.Error("should have 'environment_type' string")
	}
}

// --- Phase 8: Interface Compliance ---

func TestLocalEnvironmentSatisfiesInterface(t *testing.T) {
	var e agent.Environment = NewLocalEnvironment(LocalEnvironmentConfig{})
	if e == nil {
		t.Fatal("LocalEnvironment should satisfy agent.Environment")
	}
}
