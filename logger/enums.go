package logger

type level string

const (
	INFO    level = "INFO"
	ERROR   level = "ERROR"
	WARNING level = "WARNING"
	TRACE   level = "TRACE"
)

type messageLog string

const (
	msgSuccess messageLog = "successful operation"
	msgError   messageLog = "operation failure"
)
