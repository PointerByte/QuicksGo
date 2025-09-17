package logger

type LogEntry struct {
	Timestamp  string         `json:"timestamp"`
	IdTrace    string         `json:"idTrace,omitempty"`
	IdSpan     string         `json:"idSpan,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
	Level      level          `json:"level"`
	Message    string         `json:"message"`
	File       string         `json:"file,omitempty"`
	FuncName   string         `json:"funcName,omitempty"`
	Line       int            `json:"line,omitempty"`
	Latency    int64          `json:"latency,omitempty"`
}
