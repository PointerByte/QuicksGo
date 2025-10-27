package logger

import (
	"context"
	"encoding/json"
	"log"
	"maps"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/trace"
)

func MiddlewaresInitLogger() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		withAutoLog := ctx.Value(WithAutoLog)
		withAutoLogBool, ok := withAutoLog.(bool)
		if !ok || !withAutoLogBool {
			ctxLogger := ContextLogger(ctx.Request.Context())
			ctx.Request = ctx.Request.WithContext(ctxLogger)
			ctx.Next()
			return
		}
	}
}

func SetMessageLog(ctx *gin.Context, level level, message string) {
	ctx.Set(levelKey, level)
	ctx.Set(messageKey, message)
}

func (l *LogFormat) SetTraceID(traceID string) {
	l.TraceID = traceID
}

func (l *LogFormat) SetSpanID(spanID string) {
	l.SpanID = spanID
}

func (l *LogFormat) getDefaultLogGin(params gin.LogFormatterParams) string {
	ctx := params.Request.Context()

	// ---------- Trace / Span ----------
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		l.SetTraceID(spanCtx.TraceID().String())
		l.SetSpanID(spanCtx.SpanID().String())
	}

	// ---------- Atributes ----------
	attrs := make(map[string]any)
	attrs["service"] = viper.GetString("service.name")
	attrs["clientIP"] = params.ClientIP
	attrs["method"] = params.Method
	attrs["path"] = params.Path
	attrs["proto"] = params.Request.Proto
	attrs["userAgent"] = params.Request.UserAgent()
	if v := params.Keys[attributesKey]; v != nil {
		if cast, ok := v.(map[string]any); ok {
			maps.Copy(attrs, cast)
		}
	}

	// Get message
	message := params.ErrorMessage
	if v := params.Keys[messageKey]; v != nil {
		message = v.(string)
	}
	if message == "" {
		if params.StatusCode >= 200 && params.StatusCode <= 299 {
			message = string(MsgSuccess)
		} else {
			message = string(MsgError)
		}
	}

	// Get Level
	_level := ERROR
	if params.ErrorMessage == "" {
		_level = UNKNOWN
		if v := params.Keys[levelKey]; v != nil {
			_level = v.(level)
		}
	}

	// ---------- Format Logger ----------
	entry := LogFormat{
		Timestamp:  params.TimeStamp.Format(viper.GetString("logger.dateFormat")),
		TraceID:    l.TraceID,
		SpanID:     l.TraceID,
		Level:      _level,
		Message:    message,
		Attributes: attrs,
		Latency:    params.Latency.Milliseconds(),
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

func EmitOtel(ctx context.Context, traceID string, spanID string, level level, result string) {
	go emitOtel(ctx, traceID, spanID, level, result)
}

// CustomLogFormatGin returns a Gin middleware that provides a custom log formatter.
//
// This middleware wraps gin.LoggerWithFormatter to define a custom log format
// through the LogFormat struct. Before logging, it checks the "WithAutoLog"
// key stored in the request context (params.Keys[WithAutoLog]); if that key
// is present and set to false, the log entry will be skipped.
//
// Example usage:
//
//	r := gin.New()
//	r.Use(CustomLogFormatGin())
//
// This allows dynamic control over which requests are logged.
func CustomLogFormatGin() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(params gin.LogFormatterParams) string {
		// Retrieve the "WithAutoLog" value from the request context if present.
		withAutoLog := params.Keys[WithAutoLog]

		// Try to cast the value to bool; default to true if not found or invalid.
		withAutoLogBool, ok := withAutoLog.(bool)
		if !ok {
			withAutoLogBool = true
		}

		// Skip logging if WithAutoLog is explicitly set to false.
		if !withAutoLogBool {
			return ""
		}

		// Create a new log formatter and generate the formatted output.
		newLogFormat := new(LogFormat)
		result := newLogFormat.getDefaultLogGin(params)
		return result + "\n"
	})
}
