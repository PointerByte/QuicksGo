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
	msgSuccess messageLog = "Successful operation"
	msgError   messageLog = "Operation failure"
)

type KeyContex string

const (
	startTimeKey  KeyContex = "startTime"
	attributesKey KeyContex = "Attributes"
	WithAutoLog   KeyContex = "withAutoLog"
)
