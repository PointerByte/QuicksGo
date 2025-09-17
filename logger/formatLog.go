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

func print(ctx context.Context, level level, message string) {
	// ---------- Trace / Span ----------
	spanCtx := trace.SpanContextFromContext(ctx)
	var traceID string
	var spanID string
	if spanCtx.IsValid() {
		traceID = spanCtx.TraceID().String()
		spanID = spanCtx.SpanID().String()
	}

	// ---------- Atributes ----------
	attrs := make(map[string]any)
	if v := ctx.Value("Attributes"); v != nil {
		if cast, ok := v.(map[string]any); ok {
			maps.Copy(attrs, cast)
		}
	}

	// ---------- Get Line ----------
	funcName, file, line := traceCaller(3)

	// ---------- Format Logger ----------
	entry := LogEntry{
		Timestamp:  time.Now().Format(viper.GetString("logger.dateFormat")),
		IdTrace:    traceID,
		IdSpan:     spanID,
		Level:      level,
		Message:    message,
		Attributes: attrs,
		File:       file,
		FuncName:   funcName,
		Line:       line,
	}
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(string(jsonBytes))
}

func Info(ctx context.Context, message string) {
	print(ctx, INFO, message)
}

func Error(ctx context.Context, message string) {
	print(ctx, ERROR, message)
}

func Warning(ctx context.Context, message string) {
	print(ctx, WARNING, message)
}
