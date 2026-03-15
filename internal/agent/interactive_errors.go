package agent

type UserInterruptionError struct {
	InterruptAgentFlowError
}

func (e *UserInterruptionError) Unwrap() error {
	return &e.InterruptAgentFlowError
}
