package logger

type Level string

const (
	TraceLevel   Level = "TRACE"
	DebugLevel   Level = "DEBUG"
	InfoLevel    Level = "INFO"
	WarningLevel Level = "WARNING"
	ErrorLevel   Level = "ERROR"
	FatalLevel   Level = "FATAL"
	UnknowLevel  Level = "UNKNOW"
)

// get priority level of log
func (l Level) getPriorityLevel() uint8 {
	switch l {
	case TraceLevel:
		return 0
	case DebugLevel:
		return 1
	case InfoLevel:
		return 2
	case WarningLevel:
		return 3
	case ErrorLevel:
		return 4
	case FatalLevel:
		return 5
	default:
		return 6
	}
}
