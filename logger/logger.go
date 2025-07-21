package logger

import (
	"io"
	"log"
	"sync"
	"time"
)

// entry represents a log entry
type entry struct {
	Level   Level
	Time    time.Time
	Message string
	Fields  []any // alternating keys and values
}

// formatter serializes an Entry to bytes
type formatter interface {
	format(*entry) ([]byte, error)
}

// Logger is the main logging structure
type Logger struct {
	mu        sync.Mutex
	out       io.Writer
	level     Level
	formatter formatter
}

// New creates a Logger with the given options
func New(opts ...Option) *Logger {
	l := &Logger{
		out:       io.Discard,
		level:     InfoLevel,
		formatter: newTextFormatter(),
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// log logs an internal Entry
func (l *Logger) log(level Level, msg string, args ...any) {
	if level.getPriorityLevel() < l.level.getPriorityLevel() {
		return
	}

	entry := &entry{
		Level:   level,
		Time:    time.Now(),
		Message: msg,
		Fields:  args,
	}

	// format
	data, err := l.formatter.format(entry)
	if err != nil {
		log.Fatal(err)
	}

	l.mu.Lock()
	l.out.Write(data)
	l.mu.Unlock()

	if level == FatalLevel {
		// terminate the application
		log.Fatal(msg)
	}
}

// Debug logs a Debug level message
func (l *Logger) Debug(msg string, args ...any) {
	l.log(DebugLevel, msg, args...)
}

// Info logs an Info level message
func (l *Logger) Info(msg string, args ...any) {
	l.log(InfoLevel, msg, args...)
}

// Warn logs a Warning level message
func (l *Logger) Warn(msg string, args ...any) {
	l.log(WarningLevel, msg, args...)
}

// Error logs an Error level message
func (l *Logger) Error(msg string, args ...any) {
	l.log(ErrorLevel, msg, args...)
}

// Fatal logs a Fatal level message and panics
func (l *Logger) Fatal(msg string, args ...any) {
	l.log(FatalLevel, msg, args...)
}
