package logger

import (
	"context"
	"encoding/json"
	"log"
	"maps"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// MiddlewaresInitLogger returns a Gin middleware that initializes
func MiddlewaresInitLogger() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set(LevelKey, UNKNOWN)
		withAutoLog := ctx.Value(WithAutoLog)
		withAutoLogBool, ok := withAutoLog.(bool)
		if !ok || !withAutoLogBool {
			ctxLogger := New(ctx)
			traceID, ok1 := ctxLogger.Get(TraceIdOtel)
			spanID, ok2 := ctxLogger.Get(SpanIdOtel)
			if ok1 && ok2 {
				ctx.Set(TraceIdOtel, traceID.(string))
				ctx.Set(SpanIdOtel, spanID.(string))
			}
			ctx.Next()
			return
		}
	}
}

// SetMessageLog updates the request context with a log level and message.
func SetMessageLog(ctx *gin.Context, level Level, message string) {
	ctxLogger := context.WithValue(ctx.Request.Context(), LevelKey, level)
	ctx.Request = ctx.Request.WithContext(ctxLogger)
	ctx.Set(LevelKey, level)
	ctx.Set(messageKey, message)
}

func getDefaultLogGin(params gin.LogFormatterParams) string {
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
	level := ERROR
	if params.ErrorMessage == "" {
		level = UNKNOWN
		if v := params.Keys[LevelKey]; v != nil {
			level = v.(Level)
		}
	}

	traceID := params.Keys[TraceIdOtel]
	spanID := params.Keys[SpanIdOtel]

	// ---------- Format Logger ----------
	entry := LogFormat{
		Timestamp:  params.TimeStamp.Format(viper.GetString("logger.dateFormat")),
		TraceID:    traceID.(string),
		SpanID:     spanID.(string),
		Level:      level,
		Message:    message,
		Attributes: attrs,
		Time:       params.Latency.Milliseconds(),
	}
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		log.Fatal(err)
	}
	return string(jsonBytes)
}

type HandleLoggerFormatGin func(params gin.LogFormatterParams) string

var loggerFormatGin HandleLoggerFormatGin

func init() {
	loggerFormatGin = getDefaultLogGin
}

// SetLoggerFormatGin sets the global logger formatting handler for Gin formated.
// The provided function is stored and later used to format log output.
// It can be used to customize the log message structure across the application.
func SetLoggerFormatGin(fn HandleLoggerFormatGin) {
	loggerFormatGin = fn
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
		result := loggerFormatGin(params)

		// Emit to Otel
		traceID := params.Keys[TraceIdOtel]
		spanID := params.Keys[SpanIdOtel]
		level := params.Keys[LevelKey]
		go emitOtel(params.Request.Context(), traceID.(string), spanID.(string), level.(Level), result)

		return result + "\n"
	})
}
