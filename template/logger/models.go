package logger

type LogFormat struct {
	Timestamp  string         `json:"timestamp"`
	TraceID    string         `json:"idTrace,omitempty"`
	SpanID     string         `json:"idSpan,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
	Level      Level          `json:"level"`
	Message    string         `json:"message"`
	File       string         `json:"file,omitempty"`
	FuncName   string         `json:"funcName,omitempty"`
	Line       int            `json:"line,omitempty"`
	Time       int64          `json:"time"`
}
