package logger

import (
	"encoding/json"
	"errors"
	"log"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

const maxDepth = 64

func getLine(method string) (int, error) {
	pcs := make([]uintptr, maxDepth)
	n := runtime.Callers(2, pcs) // saltar getLine y su caller
	if n == 0 {
		return 0, errors.New("no stack frames")
	}
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if frame.Function == method || strings.HasSuffix(frame.Function, method) {
			return frame.Line, nil
		}
		if !more {
			break
		}
	}
	return 0, errors.New("function not found in stack")
}

func getLevelCode(status int) level {
	switch {
	case status >= 200 && status <= 299:
		return INFO
	default:
		return ERROR
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

func CustomLogFormat() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(params gin.LogFormatterParams) string {
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
		attrs["path"] = params.Path
		attrs["proto"] = params.Request.Proto
		attrs["userAgent"] = params.Request.UserAgent()
		if v := params.Request.Context().Value("Attributes"); v != nil {
			if cast, ok := v.(map[string]any); ok {
				for key, value := range cast {
					attrs[key] = value
				}
			}
		}

		// ---------- Get Line ----------
		line, err := getLine(params.Method)
		if err != nil {
			log.Fatal(err)
		}

		// ---------- Format Logger ----------
		entry := LogEntry{
			Timestamp:  params.TimeStamp.Format("2006-01-02T15:04:05.000Z"),
			IdTrace:    traceID,
			IdSpan:     spanID,
			Level:      getLevelCode(params.Request.Response.StatusCode),
			Message:    params.ErrorMessage,
			Attributes: attrs,
			Method:     params.Method,
			Line:       line,
			Latency:    params.Latency.Milliseconds(),
		}
		jsonBytes, err := json.Marshal(entry)
		if err != nil {
			log.Fatal(err)
		}
		return string(jsonBytes) + "\n"
	})
}
