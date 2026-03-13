package agent

type InterruptAgentFlowError struct {
	Messages []Message
}

func (e *InterruptAgentFlowError) Error() string {
    return "agent flow interrupted"
}

type SubmittedError struct {
	InterruptAgentFlowError
}

func (e *SubmittedError) Unwrap() error {
	return &e.InterruptAgentFlowError
}

func NewSubmittedError(submission string) *SubmittedError {
    return &SubmittedError{
        InterruptAgentFlowError: InterruptAgentFlowError{
            Messages: []Message{{
                Role:    "exit",
                Content: "Submitted",
                Extra:   map[string]any{"exit_status": "Submitted", "submission": submission},
            }},
        },
    }
}

type LimitsExceededError struct {
    InterruptAgentFlowError
}

func NewLimitsExceededError() *LimitsExceededError {
    return &LimitsExceededError{
        InterruptAgentFlowError: InterruptAgentFlowError{
            Messages: []Message{{
                Role:    "exit",
                Content: "LimitsExceeded",
                Extra:   map[string]any{"exit_status": "LimitsExceeded", "submission": ""},
            }},
        },
    }
}

type FormatError struct {
    InterruptAgentFlowError
}