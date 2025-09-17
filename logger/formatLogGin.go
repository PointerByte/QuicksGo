package logger

import (
	"encoding/json"
	"errors"
	"log"
	"maps"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/trace"
)

func getLevelCode(status int) level {
	switch {
	case status >= 500:
		return ERROR
	case status == 408 || status == 429:
		return WARNING
	case status >= 400: // 400, 401, 403, 404, 409, 422, etc.
		return INFO
	case status >= 100 && status < 400: // 1xx, 2xx, 3xx
		return INFO
	default:
		return ERROR // status fuera de rango válido
	}
}

func MiddlewareErrorMessage() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if ctx.Request.Response.StatusCode >= 200 && ctx.Request.Response.StatusCode <= 299 {
			ctx.Error(errors.New(string(msgSuccess)))
			return
		}
		ctx.Error(errors.New(string(msgError)))
	}
}

func SetMessage(ctx *gin.Context, message string) {
	ctx.Set("message", message)
}

func CustomLogFormat(exceptPath []string) gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(params gin.LogFormatterParams) string {
		if slices.Contains(exceptPath, params.Path) {
			return ""
		}

		// ---------- Trace / Span ----------
		spanCtx := trace.SpanContextFromContext(params.Request.Context())
		var traceID string
		var spanID string
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
		if v := params.Request.Context().Value("Attributes"); v != nil {
			if cast, ok := v.(map[string]any); ok {
				maps.Copy(attrs, cast)
			}
		}

		message := params.ErrorMessage
		if v := params.Request.Context().Value("message"); v != nil {
			message = v.(string)
		}

		// ---------- Format Logger ----------
		entry := LogEntry{
			Timestamp:  params.TimeStamp.Format(viper.GetString("logger.dateFormat")),
			IdTrace:    traceID,
			IdSpan:     spanID,
			Level:      getLevelCode(params.StatusCode),
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
