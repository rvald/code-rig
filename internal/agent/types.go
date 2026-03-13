package agent

import (
	"github.com/rvald/code-rig/internal/types"
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
	SystemTemplate   string
	InstanceTemplate string
	StepLimit        int
	CostLimit        float64
	OutputPath       string
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