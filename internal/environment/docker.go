package environment

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rvald/code-rig/internal/agent"
	"github.com/rvald/code-rig/internal/utils"
)

var _ agent.Environment = (*DockerEnvironment)(nil)

type DockerEnvironment struct {
	Config      DockerEnvironmentConfig
	ContainerID string
}

func NewDockerEnvironment(cfg DockerEnvironmentConfig) (*DockerEnvironment, error) {
	cfg.ApplyDefaults()
	env := &DockerEnvironment{Config: cfg}

	if err := env.startContainer(); err != nil {
		return nil, err
	}
	return env, nil
}

func (e *DockerEnvironment) startContainer() error {
	containerName := fmt.Sprintf("code-rig-%s", uuid.New().String()[:8])

	args := []string{
		"run", "-d", "--name", containerName, "-w", e.Config.Cwd,
	}
	args = append(args, e.Config.RunArgs...)
	args = append(args, e.Config.Image, "sleep", e.Config.ContainerTimeout)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(e.Config.PullTimeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.Config.Executable, args...)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("starting container failed: %v\nstderr: %s", err, errBuf.String())
	}

	e.ContainerID = strings.TrimSpace(outBuf.String())
	return nil
}

func (e *DockerEnvironment) Cleanup() {
	if e.ContainerID == "" {
		return
	}
	// "docker rm -f <id>"
	cmd := exec.Command(e.Config.Executable, "rm", "-f", e.ContainerID)
	_ = cmd.Run() // Best effort
	e.ContainerID = ""
}

func (e *DockerEnvironment) Execute(action agent.Action) (agent.Observation, error) {
	if e.ContainerID == "" {
		return agent.Observation{ReturnCode: -1, ExceptionInfo: "Container not started"}, nil
	}

	args := []string{"exec", "-w", e.Config.Cwd}

	// ForwardEnv values would go here (skip parsing os.Environ for now to keep it simpler)
	for k, v := range e.Config.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, e.ContainerID)
	args = append(args, e.Config.Interpreter...)
	args = append(args, action.Command)

	timeout := e.Config.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.Config.Executable, args...)

	out, err := cmd.CombinedOutput()
	outputStr := string(out)
	
	obs := agent.Observation{
		Output: outputStr,
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			obs.ReturnCode = -1
			obs.ExceptionInfo = "Command timed out"
			return obs, nil
		}

		if exitErr, ok := err.(*exec.ExitError); ok {
			obs.ReturnCode = exitErr.ExitCode()
		} else {
			obs.ReturnCode = -1
			obs.ExceptionInfo = err.Error()
		}
	} else {
		obs.ReturnCode = 0
	}

	// Step 2.2 Task Sentinel Detection
	lines := strings.Split(strings.TrimLeft(outputStr, " \t\r\n"), "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT" && obs.ReturnCode == 0 {
		submission := strings.Join(lines[1:], "\n")
		msg := agent.Message{
			Role:    "exit",
			Content: submission,
			Extra: map[string]any{
				"exit_status": "Submitted",
				"submission":  submission,
			},
		}
		return obs, &agent.SubmittedError{
			InterruptAgentFlowError: agent.InterruptAgentFlowError{
				Messages: []agent.Message{msg},
			},
		}
	}

	return obs, nil
}

func (e *DockerEnvironment) GetTemplateVars() map[string]any {
	configVars := map[string]any{
		"cwd":     e.Config.Cwd,
		"timeout": e.Config.Timeout,
		"image":   e.Config.Image,
	}

	// Platform vars inside a container aren't the host's, but we'll mock them generically for the template rendering
	platformVars := map[string]any{
		"system":  "linux",
		"machine": "unknown", // can't run `uname -m` without an extra container exec call
		"node":    e.ContainerID,
	}

	envVars := make(map[string]any)
	for k, v := range e.Config.Env {
		envVars[k] = v
	}

	return utils.RecursiveMerge(configVars, platformVars, envVars)
}

func (e *DockerEnvironment) Serialize() map[string]any {
	return map[string]any{
		"info": map[string]any{
			"config": map[string]any{
				"environment": map[string]any{
					"cwd":         e.Config.Cwd,
					"env":         e.Config.Env,
					"timeout":     e.Config.Timeout,
					"image":       e.Config.Image,
					"forward_env": e.Config.ForwardEnv,
				},
				"environment_type": "environment.DockerEnvironment",
			},
		},
	}
}
