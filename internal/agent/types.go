package agent

type AgentConfig struct {
	SystemTemplate string
	InstanceTemplate string
	StepLimit int
	CostLimit float64
	OutputPath string
}

type Message struct {
    Role    string         `json:"role"`
    Content string         `json:"content"`
    Extra   map[string]any `json:"extra,omitempty"`
}

type Action struct {
    Command    string `json:"command"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

type Observation struct {
	Output        string `json:"output"`
	ReturnCode    int    `json:"returncode"`
	ExceptionInfo string `json:"exception_info"`
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