package logger

type Level string

const (
	InfoLevel    Level = "INFO"
	DebugLevel   Level = "DEBUG"
	WarningLevel Level = "WARNING"
	ErrorLevel   Level = "ERROR"
	FatalLevel   Level = "FATAL"
	UnknowLevel  Level = "UNKNOW"
)

// get priority level of log
func (l Level) getPriorityLevel() uint8 {
	switch l {
	case InfoLevel:
		return 1
	case DebugLevel:
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
