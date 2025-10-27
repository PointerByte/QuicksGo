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

func customLogFormat(ctx context.Context, level level, message string) string {
	// Filter path with log enable
	withAutoLog := ctx.Value(WithAutoLog)
	withAutoLogBool, ok := withAutoLog.(bool)
	if !ok {
		withAutoLogBool = true
	}
	if !withAutoLogBool {
		return ""
	}

	// ---------- Trace / Span ----------
	var traceID string
	var spanID string
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		traceID = spanCtx.TraceID().String()
		spanID = spanCtx.SpanID().String()
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
	entry := LogEntry{
		Timestamp:  time.Now().Format(viper.GetString("logger.dateFormat")),
		TraceId:    traceID,
		SpanId:     spanID,
		Level:      level,
		Message:    message,
		Attributes: attrs,
		File:       file,
		FuncName:   funcName,
		Line:       line,
		Latency:    latency,
	}
	go emitOtel(ctx, level, entry)
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		log.Fatal(err)
	}
	result := string(jsonBytes)
	return result + "\n"
}

func Info(ctx context.Context, message string) {
	go log.Print(customLogFormat(ctx, INFO, message))
}

func Error(ctx context.Context, err error) {
	go log.Print(customLogFormat(ctx, INFO, customLogFormat(ctx, ERROR, err.Error())))
}

func Warning(ctx context.Context, message string) {
	go log.Print(customLogFormat(ctx, WARNING, message))
}

func Fatal(ctx context.Context, err error) {
	log.Fatal(customLogFormat(ctx, FATAL, err.Error()))
}

func Panic(ctx context.Context, err error) {
	log.Panic(customLogFormat(ctx, PANIC, err.Error()))
}
