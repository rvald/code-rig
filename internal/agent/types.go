package agent

import (
	"fmt"

	"github.com/rvald/code-rig/internal/types"
	"gopkg.in/yaml.v3"
)

// Type aliases so the rest of the agent package can use unqualified names.
type Message = types.Message
type Action = types.Action
type Observation = types.Observation

// Error type aliases.
type InterruptAgentFlowError = types.InterruptAgentFlowError
type SubmittedError = types.SubmittedError
type LimitsExceededError = types.LimitsExceededError
type FormatError = types.FormatError

// Constructor aliases.
var NewSubmittedError = types.NewSubmittedError
var NewLimitsExceededError = types.NewLimitsExceededError

type AgentConfig struct {
	SystemTemplate   string  `json:"system_template" yaml:"system_template"`
	InstanceTemplate string  `json:"instance_template" yaml:"instance_template"`
	StepLimit        int     `json:"step_limit" yaml:"step_limit"`
	CostLimit        float64 `json:"cost_limit" yaml:"cost_limit"`
	OutputPath       string  `json:"output_path" yaml:"output_path"`
}

func BuildAgentConfigFromRawMap(raw map[string]any) (AgentConfig, error) {
	data, err := yaml.Marshal(raw)
	if err != nil {
		return AgentConfig{}, fmt.Errorf("marshaling agent config: %w", err)
	}
	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return AgentConfig{}, fmt.Errorf("unmarshaling agent config: %w", err)
	}
	return cfg, nil
}

func ValidateAgentConfig(cfg AgentConfig) error {
	if cfg.SystemTemplate == "" {
		return fmt.Errorf("agent config: system_template is required")
	}
	if cfg.InstanceTemplate == "" {
		return fmt.Errorf("agent config: instance_template is required")
	}
	return nil
}

type Model interface {
	Query(messages []Message) (Message, error)
	FormatMessage(role, content string, extra map[string]any) Message
	FormatObservationMessages(message Message, outputs []Observation) []Message
	GetTemplateVars() map[string]any
	Serialize() map[string]any
}

type Environment interface {
	Execute(action Action) (Observation, error)
	GetTemplateVars() map[string]any
	Serialize() map[string]any
}