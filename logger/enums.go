package logger

type Level string

const (
	INFO    Level = "INFO"
	ERROR   Level = "ERROR"
	WARNING Level = "WARNING"
	FATAL   Level = "FATAL"
	PANIC   Level = "PANIC"
	UNKNOWN Level = "UNKNOWN"
)

type messageLog string

const (
	MsgSuccess messageLog = "Successful operation"
	MsgError   messageLog = "Operation failure"
)

type KeyContex string

const (
	LevelKey      KeyContex = "level"
	messageKey    KeyContex = "message"
	attributesKey KeyContex = "attributes"
	WithAutoLog   KeyContex = "withAutoLog"
	TraceIdOtel   KeyContex = "traceIdOtel"
	SpanIdOtel    KeyContex = "spanIdOtel"
)
