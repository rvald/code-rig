package environment

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/rvald/code-rig/internal/agent"
	"github.com/rvald/code-rig/internal/types"
	"github.com/rvald/code-rig/internal/utils"
)

// Type aliases so the environment package can use unqualified names.
type Action = types.Action
type Observation = types.Observation
type SubmittedError = types.SubmittedError

// Compile-time check that LocalEnvironment satisfies the agent.Environment interface.
var _ agent.Environment = (*LocalEnvironment)(nil)

const submissionSentinel = "COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT"

type LocalEnvironment struct {
	config LocalEnvironmentConfig
}

func NewLocalEnvironment(cfg LocalEnvironmentConfig) *LocalEnvironment {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30
	}
	if cfg.Env == nil {
		cfg.Env = make(map[string]string)
	}
	return &LocalEnvironment{config: cfg}
}

func (e *LocalEnvironment) Execute(action Action) (Observation, error) {
	cwd := e.config.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(e.config.Timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", action.Command)
	cmd.Dir = cwd
	cmd.Env = e.mergeEnv()

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	runErr := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return Observation{
			Output:        outBuf.String(),
			ReturnCode:    -1,
			ExceptionInfo: fmt.Sprintf("An error occurred while executing the command: %v", ctx.Err()),
		}, nil
	}

	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			obs := Observation{
				Output:        outBuf.String(),
				ReturnCode:    exitErr.ExitCode(),
				ExceptionInfo: "",
			}
			return e.checkFinished(obs)
		}
		// Other error (e.g. bash binary not found on system)
		return Observation{
			Output:        outBuf.String(),
			ReturnCode:    -1,
			ExceptionInfo: fmt.Sprintf("An error occurred while executing the command: %v", runErr),
		}, nil
	}

	obs := Observation{
		Output:     outBuf.String(),
		ReturnCode: 0,
	}
	return e.checkFinished(obs)
}

func (e *LocalEnvironment) mergeEnv() []string {
	env := os.Environ()
	for k, v := range e.config.Env {
		env = append(env, k+"="+v)
	}
	return env
}

func (e *LocalEnvironment) checkFinished(obs Observation) (Observation, error) {
	if obs.ReturnCode != 0 {
		return obs, nil
	}
	trimmed := strings.TrimLeft(obs.Output, " \t\n\r")
	lines := strings.SplitAfter(trimmed, "\n")
	if len(lines) == 0 {
		return obs, nil
	}
	if strings.TrimSpace(lines[0]) != submissionSentinel {
		return obs, nil
	}
	submission := strings.Join(lines[1:], "")
	return obs, types.NewSubmittedError(submission)
}

func (e *LocalEnvironment) GetTemplateVars() map[string]any {
	configVars := map[string]any{
		"cwd":     e.config.Cwd,
		"timeout": e.config.Timeout,
	}

	hostname, _ := os.Hostname()
	platformVars := map[string]any{
		"system":  runtime.GOOS,
		"machine": runtime.GOARCH,
		"node":    hostname,
	}

	envVars := make(map[string]any)
	for _, entry := range os.Environ() {
		if k, v, ok := strings.Cut(entry, "="); ok {
			envVars[k] = v
		}
	}

	return utils.RecursiveMerge(configVars, platformVars, envVars)
}

func (e *LocalEnvironment) Serialize() map[string]any {
	return map[string]any{
		"info": map[string]any{
			"config": map[string]any{
				"environment": map[string]any{
					"cwd":     e.config.Cwd,
					"env":     e.config.Env,
					"timeout": e.config.Timeout,
				},
				"environment_type": "environment.LocalEnvironment",
			},
		},
	}
}
