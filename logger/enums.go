package logger

type level string

const (
	INFO    level = "INFO"
	ERROR   level = "ERROR"
	WARNING level = "WARNING"
	FATAL   level = "FATAL"
	PANIC   level = "PANIC"
	UNKNOWN level = "UNKNOWN"
)

type messageLog string

const (
	MsgSuccess messageLog = "Successful operation"
	MsgError   messageLog = "Operation failure"
)

type KeyContex string

const (
	startTimeKey  KeyContex = "startTime"
	levelKey      KeyContex = "level"
	messageKey    KeyContex = "message"
	attributesKey KeyContex = "attributes"
	WithAutoLog   KeyContex = "withAutoLog"
)
