package environment

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type LocalEnvironmentConfig struct {
	Cwd     string            `json:"cwd" yaml:"cwd"`
	Env     map[string]string `json:"env" yaml:"env"`
	Timeout int               `json:"timeout" yaml:"timeout"`
}

func BuildEnvironmentConfigFromRawMap(raw map[string]any) (LocalEnvironmentConfig, error) {
	data, err := yaml.Marshal(raw)
	if err != nil {
		return LocalEnvironmentConfig{}, fmt.Errorf("marshaling env config: %w", err)
	}
	var cfg LocalEnvironmentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return LocalEnvironmentConfig{}, fmt.Errorf("unmarshaling env config: %w", err)
	}
	return cfg, nil
}

type DockerEnvironmentConfig struct {
	Image            string            `json:"image" yaml:"image"`
	Cwd              string            `json:"cwd" yaml:"cwd"`
	Env              map[string]string `json:"env" yaml:"env"`
	ForwardEnv       []string          `json:"forward_env" yaml:"forward_env"`
	Timeout          int               `json:"timeout" yaml:"timeout"`                     // cmd timeout
	Executable       string            `json:"executable" yaml:"executable"`               // default "docker"
	RunArgs          []string          `json:"run_args" yaml:"run_args"`                   // default ["--rm"]
	ContainerTimeout string            `json:"container_timeout" yaml:"container_timeout"` // default "2h"
	PullTimeout      int               `json:"pull_timeout" yaml:"pull_timeout"`           // default 120s
	Interpreter      []string          `json:"interpreter" yaml:"interpreter"`             // default ["bash", "-lc"]
}

// Helper to fill defaults if missing
func (c *DockerEnvironmentConfig) ApplyDefaults() {
	if c.Cwd == "" {
		c.Cwd = "/"
	}
	if c.Executable == "" {
		c.Executable = "docker"
	}
	if c.ContainerTimeout == "" {
		c.ContainerTimeout = "2h"
	}
	if c.PullTimeout == 0 {
		c.PullTimeout = 120
	}
	if len(c.RunArgs) == 0 {
		c.RunArgs = []string{"--rm"}
	}
	if len(c.Interpreter) == 0 {
		c.Interpreter = []string{"bash", "-lc"}
	}
}

func BuildDockerEnvironmentConfigFromRawMap(raw map[string]any) (DockerEnvironmentConfig, error) {
	data, err := yaml.Marshal(raw)
	if err != nil {
		return DockerEnvironmentConfig{}, fmt.Errorf("marshaling docker env config: %w", err)
	}
	var cfg DockerEnvironmentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return DockerEnvironmentConfig{}, fmt.Errorf("unmarshaling docker env config: %w", err)
	}
	cfg.ApplyDefaults()
	return cfg, nil
}
