package environment

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/rvald/code-rig/internal/agent"
)

func TestDockerConfigDefaults(t *testing.T) {
	cfg := DockerEnvironmentConfig{
		Image: "ubuntu:latest",
	}
	cfg.ApplyDefaults()

	// We should have sensible defaults matching Python
	if cfg.Cwd != "" && cfg.Cwd != "/" {
		t.Errorf("Cwd = %q, want '/' or empty", cfg.Cwd)
	}
	if cfg.ContainerTimeout == "" {
		t.Errorf("expected default ContainerTimeout (e.g. '2h')")
	}
}

func TestDockerLifecycle(t *testing.T) {
	// Skip if docker isn't installed
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker executable not found, skipping TestDockerLifecycle")
	}

	cfg := DockerEnvironmentConfig{Image: "alpine:latest"}
	cfg.ApplyDefaults()
	// Override interpreter for alpine which uses sh, not bash
	cfg.Interpreter = []string{"sh", "-c"}

	env, err := NewDockerEnvironment(cfg)
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}
	defer env.Cleanup()

	if env.ContainerID == "" {
		t.Fatalf("expected a ContainerID to be populated")
	}

	// Verify container is actually running
	out, err := exec.Command("docker", "ps", "-q", "--no-trunc", "-f", "id="+env.ContainerID).Output()
	if err != nil || strings.TrimSpace(string(out)) != env.ContainerID {
		t.Errorf("container %s is not running", env.ContainerID)
	}

	// Test cleanup
	originalID := env.ContainerID
	env.Cleanup()
	out, _ = exec.Command("docker", "ps", "-q", "--no-trunc", "-f", "id="+originalID).Output()
	if strings.TrimSpace(string(out)) == originalID {
		t.Errorf("container %s was not killed during cleanup", originalID)
	}
	if env.ContainerID != "" {
		t.Errorf("expected ContainerID to be cleared, got %q", env.ContainerID)
	}
}

func TestDockerExecuteHappyPath(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker executable not found, skipping TestDockerExecuteHappyPath")
	}

	cfg := DockerEnvironmentConfig{
		Image:       "alpine:latest",
		Interpreter: []string{"sh", "-c"},
	}
	env, _ := NewDockerEnvironment(cfg)
	defer env.Cleanup()

	action := agent.Action{Command: "echo 'hello from docker'"}
	obs, err := env.Execute(action)

	if err != nil {
		t.Fatalf("unexpected Execute error: %v", err)
	}
	if obs.ReturnCode != 0 {
		t.Errorf("ReturnCode = %d, want 0", obs.ReturnCode)
	}
	if !strings.Contains(obs.Output, "hello from docker") {
		t.Errorf("Output = %q", obs.Output)
	}
}

func TestDockerExecuteSubmitted(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip()
	}

	cfg := DockerEnvironmentConfig{Image: "alpine:latest", Interpreter: []string{"sh", "-c"}}
	env, _ := NewDockerEnvironment(cfg)
	defer env.Cleanup()

	action := agent.Action{Command: "echo 'COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT\nmy fix'"}

	_, err := env.Execute(action)

	var subErr *agent.SubmittedError
	if !errors.As(err, &subErr) { // Assuming errors package missing, fallback to errors.As logic implicitly checked
		t.Fatalf("expected SubmittedError, got %v", err)
	}

	submission, ok := subErr.Messages[0].Extra["submission"].(string)
	if !ok || strings.TrimSpace(submission) != "my fix" {
		t.Errorf("submission payload = %q", submission)
	}
}
