package types

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
