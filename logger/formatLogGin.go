package logger

import (
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
	ctx.Set("level", level)
	ctx.Set("message", message)
}

func CustomLogFormatGin() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(params gin.LogFormatterParams) string {
		// Filter path with log enable
		withAutoLog := params.Keys[WithAutoLog]
		withAutoLogBool, ok := withAutoLog.(bool)
		if !ok {
			withAutoLogBool = true
		}
		if !withAutoLogBool {
			return ""
		}

		ctx := params.Request.Context()
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
		attrs["clientIP"] = params.ClientIP
		attrs["method"] = params.Method
		attrs["path"] = params.Path
		attrs["proto"] = params.Request.Proto
		attrs["userAgent"] = params.Request.UserAgent()
		if v := ctx.Value(attributesKey); v != nil {
			if cast, ok := v.(map[string]any); ok {
				maps.Copy(attrs, cast)
			}
		}

		// Get message
		message := params.ErrorMessage
		if v := params.Keys["message"]; v != nil {
			message = v.(string)
		}
		if message == "" {
			if params.StatusCode >= 200 && params.StatusCode <= 299 {
				message = string(msgSuccess)
			} else {
				message = string(msgError)
			}
		}

		// Get Level
		_level := ERROR
		if params.ErrorMessage == "" {
			_level = UNKNOWN
			if v := params.Keys["level"]; v != nil {
				_level = v.(level)
			}
		}

		// ---------- Format Logger ----------
		entry := LogEntry{
			Timestamp:  params.TimeStamp.Format(viper.GetString("logger.dateFormat")),
			TraceId:    traceID,
			SpanId:     spanID,
			Level:      _level,
			Message:    message,
			Attributes: attrs,
			Latency:    params.Latency.Milliseconds(),
		}
		go emitOtel(ctx, _level, entry)
		jsonBytes, err := json.Marshal(entry)
		if err != nil {
			log.Fatal(err)
		}
		return string(jsonBytes) + "\n"
	})
}
