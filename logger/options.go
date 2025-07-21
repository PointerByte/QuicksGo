package logger

import (
	"io"
	"os"
)

// Option configures a Logger
// Option applies a configuration to the Logger instance
// e.g., setting output writer, level, formatter, or hooks.
type Option func(*Logger)

// WithOutput sets the output writer (e.g., os.Stdout)
func WithOutput(w io.Writer) Option {
	mw := io.MultiWriter(w, os.Stdout)
	return func(l *Logger) {
		l.out = mw
	}
}

// WithLevel sets the minimum log level for output
func WithLevel(level Level) Option {
	return func(l *Logger) {
		l.level = level
	}
}

// WithJSON enables JSON formatting for log entries
func WithJSON() Option {
	return func(l *Logger) {
		l.formatter = new()
	}
}

// WithText enables plain text formatting (default)
func WithText() Option {
	return func(l *Logger) {
		l.formatter = newTextFormatter()
	}
}
