package logger

import (
	"encoding/json"
	"log"
	"maps"
	"runtime"
	"time"

	"github.com/spf13/viper"
)

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

func getDefaultLog(ctx *Context, level Level, message string) string {
	// ---------- Atributes ----------
	attrs := make(map[string]any)
	if v, ok := ctx.Get(attributesKey); ok {
		if cast, ok := v.(map[string]any); ok {
			maps.Copy(attrs, cast)
		}
	}

	// ---------- Get Line ----------
	funcName, file, line := traceCaller(3)

	var traceID string
	var spanID string
	if v, ok := ctx.Get(TraceIdOtel); ok {
		traceID = v.(string)
	}
	if v, ok := ctx.Get(SpanIdOtel); ok {
		spanID = v.(string)
	}
	// ---------- Format Logger ----------
	entry := LogFormat{
		Timestamp:  time.Now().Format(viper.GetString("logger.dateFormat")),
		TraceID:    traceID,
		SpanID:     spanID,
		Level:      level,
		Message:    message,
		Attributes: attrs,
		File:       file,
		FuncName:   funcName,
		Line:       line,
		Time:       ctx.GetLatency().Milliseconds(),
	}
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		log.Fatal(err)
	}
	return string(jsonBytes)
}

func customLogFormat(ctx *Context, level Level, message string) string {
	// Get func format logger
	result := getDefaultLog(ctx, level, message)

	// Emit to Otel
	traceID, ok1 := ctx.Get(TraceIdOtel)
	spanID, ok2 := ctx.Get(SpanIdOtel)
	if ok1 && ok2 {
		go emitOtel(ctx, traceID.(string), spanID.(string), level, result)
	}

	// Return result
	return result + "\n"
}

// Info logs an informational message asynchronously using the default logger.
//
// The log message is formatted via log format with the INFO level.
func (c *Context) Info(message string) {
	go log.Print(customLogFormat(c, INFO, message))
}

// Error logs an error message asynchronously using the default logger.
//
// It formats the error using the ERROR log level. The message includes
// contextual information from the provided context.
func (c *Context) Error(err error) {
	go log.Print(customLogFormat(c, INFO, customLogFormat(c, ERROR, err.Error())))
}

// Warning logs a warning message asynchronously using the default logger.
//
// It uses the WARNING log level to indicate non-critical issues or potential problems.
func (c *Context) Warning(message string) {
	go log.Print(customLogFormat(c, WARNING, message))
}

// Fatal logs a fatal error message and then terminates the program.
//
// This function does not run asynchronously — it calls log.Fatal directly,
// which prints the message and exits the application.
func (c *Context) Fatal(err error) {
	log.Fatal(customLogFormat(c, FATAL, err.Error()))
}

// Panic logs an error message and then triggers a panic.
//
// The log entry is formatted using the PANIC log level before invoking log.Panic.
func (c *Context) Panic(err error) {
	log.Panic(customLogFormat(c, PANIC, err.Error()))
}
