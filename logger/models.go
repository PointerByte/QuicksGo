package logger

type LogEntry struct {
	Timestamp  string         `json:"timestamp"`
	IdTrace    string         `json:"idTrace"`
	IdSpan     string         `json:"idSpan"`
	Attributes map[string]any `json:"attributes"`
	Level      level          `json:"level"`
	Message    string         `json:"message"`
	Method     string         `json:"method"`
	Line       int            `json:"line"`
	Latency    int64          `json:"latency"`
}
