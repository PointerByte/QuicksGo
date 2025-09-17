package logger

import (
	"encoding/json"
	"errors"
	"log"
	"maps"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func MiddlewareErrorMessage() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if ctx.Request.Response.StatusCode >= 200 && ctx.Request.Response.StatusCode <= 299 {
			ctx.Error(errors.New(string(msgSuccess)))
			return
		}
		ctx.Error(errors.New(string(msgError)))
	}
}

func SetMessage(ctx *gin.Context, level level, message string) {
	ctx.Set("level", level)
	ctx.Set("message", message)
}

func CustomLogFormat(exceptPath []string) gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(params gin.LogFormatterParams) string {
		if slices.Contains(exceptPath, params.Path) {
			return ""
		}

		// ---------- Trace / Span ----------
		traceID := params.Request.Header.Get("X-B3-TraceId")
		spanID := params.Request.Header.Get("X-B3-SpanId")

		// ---------- Atributes ----------
		attrs := make(map[string]any)
		attrs["clientIP"] = params.ClientIP
		attrs["method"] = params.Method
		attrs["path"] = params.Path
		attrs["proto"] = params.Request.Proto
		attrs["userAgent"] = params.Request.UserAgent()
		if v := params.Request.Context().Value("Attributes"); v != nil {
			if cast, ok := v.(map[string]any); ok {
				maps.Copy(attrs, cast)
			}
		}

		// Get message
		message := params.ErrorMessage
		if v := params.Request.Context().Value("message"); v != nil {
			message = v.(string)
		}

		// Get Level
		_level := ERROR
		if params.ErrorMessage == "" {
			_level = UNKNOWN
			if v := params.Request.Context().Value("level"); v != nil {
				_level = v.(level)
			}
		}

		// ---------- Format Logger ----------
		entry := LogEntry{
			Timestamp:  params.TimeStamp.Format(viper.GetString("logger.dateFormat")),
			IdTrace:    traceID,
			IdSpan:     spanID,
			Level:      _level,
			Message:    message,
			Attributes: attrs,
			Latency:    params.Latency.Milliseconds(),
		}
		jsonBytes, err := json.Marshal(entry)
		if err != nil {
			log.Fatal(err)
		}
		return string(jsonBytes) + "\n"
	})
}
