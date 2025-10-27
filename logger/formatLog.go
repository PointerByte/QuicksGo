package logger

import (
	"context"
	"encoding/json"
	"log"
	"maps"
	"runtime"
	"time"

	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/trace"
)

func ContextLogger(ctx context.Context) context.Context {
	return context.WithValue(ctx, startTimeKey, time.Now())
}

func traceCaller(skip int) (funcName string, file string, line int) {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown", "unknown", 0
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown", file, line
	}
	return fn.Name(), file, line
}

func (l *LogFormat) getDefaultLog(ctx context.Context, level level, message string) string {
	// ---------- Trace / Span ----------
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		l.SetTraceID(spanCtx.TraceID().String())
		l.SetSpanID(spanCtx.SpanID().String())
	}

	// ---------- Atributes ----------
	attrs := make(map[string]any)
	if v := ctx.Value(attributesKey); v != nil {
		if cast, ok := v.(map[string]any); ok {
			maps.Copy(attrs, cast)
		}
	}

	// ---------- Get Line ----------
	funcName, file, line := traceCaller(3)

	var latency int64
	startTime := ctx.Value(startTimeKey)
	if startTime != nil {
		latency = time.Since(startTime.(time.Time)).Milliseconds()
	}

	// ---------- Format Logger ----------
	entry := LogFormat{
		Timestamp:  time.Now().Format(viper.GetString("logger.dateFormat")),
		TraceID:    l.TraceID,
		SpanID:     l.SpanID,
		Level:      level,
		Message:    message,
		Attributes: attrs,
		File:       file,
		FuncName:   funcName,
		Line:       line,
		Latency:    latency,
	}
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		log.Fatal(err)
	}
	result := string(jsonBytes)
	// This funtion is for emit log to otel
	EmitOtel(ctx, l.TraceID, l.SpanID, l.Level, result)
	return result
}

func customLogFormat(ctx context.Context, level level, message string) string {
	newLogFormat := new(LogFormat)
	result := newLogFormat.getDefaultLog(ctx, level, message)
	return result + "\n"
}

// Info logs an informational message asynchronously using the default logger.
//
// The log message is formatted via log format with the INFO level.
func Info(ctx context.Context, message string) {
	go log.Print(customLogFormat(ctx, INFO, message))
}

// Error logs an error message asynchronously using the default logger.
//
// It formats the error using the ERROR log level. The message includes
// contextual information from the provided context.
func Error(ctx context.Context, err error) {
	go log.Print(customLogFormat(ctx, INFO, customLogFormat(ctx, ERROR, err.Error())))
}

// Warning logs a warning message asynchronously using the default logger.
//
// It uses the WARNING log level to indicate non-critical issues or potential problems.
func Warning(ctx context.Context, message string) {
	go log.Print(customLogFormat(ctx, WARNING, message))
}

// Fatal logs a fatal error message and then terminates the program.
//
// This function does not run asynchronously — it calls log.Fatal directly,
// which prints the message and exits the application.
func Fatal(ctx context.Context, err error) {
	log.Fatal(customLogFormat(ctx, FATAL, err.Error()))
}

// Panic logs an error message and then triggers a panic.
//
// The log entry is formatted using the PANIC log level before invoking log.Panic.
func Panic(ctx context.Context, err error) {
	log.Panic(customLogFormat(ctx, PANIC, err.Error()))
}
