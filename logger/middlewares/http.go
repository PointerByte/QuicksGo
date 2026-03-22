// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package middlewares

import (
	"bytes"
	"io"

	"github.com/PointerByte/QuicksGo/logger/builder"
	"github.com/PointerByte/QuicksGo/logger/formatter"
	"github.com/PointerByte/QuicksGo/logger/utilities"
	viperdata "github.com/PointerByte/QuicksGo/logger/viperData"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// InitLogger creates or reuses the request-scoped logger context,
// extracts distributed-tracing headers, starts the server span, and stores the
// base HTTP metadata that will later be used in structured logs.
func InitLogger() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// ---- contexto base del request ----
		parent := otel.GetTextMapPropagator().Extract(
			ctx.Request.Context(),
			propagation.HeaderCarrier(ctx.Request.Header),
		)

		ctxLogger := builder.New(parent)
		appName := viperdata.GetViperData(string(viperdata.AppAtribute)).(string)
		tracer := otel.Tracer(appName)

		var span trace.Span
		ctxLogger.Context, span = tracer.Start(
			ctxLogger.Context,
			appName,
			trace.WithSpanKind(trace.SpanKindServer),
		)

		// ---- Get TraceID ----
		traceID := span.SpanContext().TraceID()
		if traceID.IsValid() {
			ctxLogger.Set(traceIDKey, traceID.String())
		}

		datosKibana := formatter.KibanaData{
			System:   appName,
			Client:   ctx.ClientIP(),
			Protocol: ctx.Request.Proto,
			Method:   ctx.Request.Method,
			Path:     ctx.Request.URL.Path,
		}
		datosKibana.SetHeaders(ctx.Request.Header)
		ctxLogger.Set(detailsKey, datosKibana)

		// ---- reinyectar contexto con span ----
		ctx.Request = ctx.Request.WithContext(ctxLogger)

		// Continuamos
		ctx.Next()

		// ---- cerrar span al final del request ----
		span.End()
	}
}

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// CaptureBody captures the raw request and response bodies and stores
// them in the gin.Context under requestBodyKey and responseBodyKey.
//
// This middleware only captures the payloads. Whether they are finally included
// in details.request and details.response depends on MiddlewareLoggerWithConfig
// and the value stored under disableBodyKey.
func CaptureBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		// ---- Capturar request ----
		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// ---- Capturar response ----
		responseBody := &bytes.Buffer{}
		writer := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           responseBody,
		}
		c.Writer = writer

		// continuar pipeline
		c.Next()

		// guardar en contexto
		c.Set(requestBodyKey, string(requestBody))
		c.Set(responseBodyKey, responseBody.String())
	}
}

// DisableBody marks the current Gin request so MiddlewareLoggerWithConfig
// omits request and response bodies from the final log entry.
//
// Internally this stores disableBodyKey=true in the gin.Context.
func DisableBody(ctx *gin.Context) {
	ctx.Set(disableBodyKey, true)
}

// LoggerWithConfig emits the final HTTP log entry using Gin's
// LoggerWithConfig hook and the package logger helpers.
//
// Body handling is controlled through disableBodyKey in gin.Context:
//   - if disableBodyKey is present and false, captured request/response bodies
//     are copied into details.request and details.response
//   - if disableBodyKey is present and true, bodies are intentionally omitted
//
// This middleware expects the request-scoped context produced by
// LoggerWithConfig and is commonly paired with MiddlewareCaptureBody.
func LoggerWithConfig() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: func(param gin.LogFormatterParams) string {
			ctx := param.Request.Context()
			ctxLogger := builder.New(ctx)
			if v, ok := param.Keys[methodKey]; ok {
				ctxLogger.Method = v.(string)
			}
			if v, ok := param.Keys[lineKey]; ok {
				ctxLogger.Line = v.(int)
			}

			if v, ok := param.Keys[disableBodyKey]; ok {
				if !v.(bool) {
					var requestBody any
					var responseBody any
					if param.Keys != nil {
						if v, ok := param.Keys[requestBodyKey]; ok {
							requestBody = v
						}
						if v, ok := param.Keys[responseBodyKey]; ok {
							responseBody = v
						}
					}
					ctxLogger.Details.Request = requestBody
					ctxLogger.Details.Response = responseBody
					ctxLogger.Set(detailsKey, ctxLogger.Details)
				}
			}

			if value, ok := param.Keys[formatter.InfoLevel]; ok {
				if msg, ok := value.(string); ok {
					ctxLogger.Info(msg)
					return ""
				}
			}
			if value, ok := param.Keys[formatter.DebugLevel]; ok {
				if msg, ok := value.(string); ok {
					ctxLogger.Debug(msg)
					return ""
				}
			}
			if value, ok := param.Keys[formatter.WarnLevel]; ok {
				if msg, ok := value.(string); ok {
					ctxLogger.Warn(msg)
					return ""
				}
			}
			if value, ok := param.Keys[formatter.ErrorLevel]; ok {
				if msg, ok := value.(error); ok {
					ctxLogger.Error(msg)
					return ""
				}
			}
			return ""
		},
		SkipPaths:       viperdata.GetViperData(string(viperdata.GinLoggerWithConfigSkipPathsAtribute)).([]string),
		SkipQueryString: viperdata.GetViperData(string(viperdata.GinLoggerWithConfigSkipQueryStringAtribute)).(bool),
		Skip: func(c *gin.Context) bool {
			return !viperdata.GetViperData(string(viperdata.GinLoggerWithConfigEnabledAtribute)).(bool)
		},
	})
}

// PrintInfo schedules an info-level log message for the current Gin request and
// stores the caller metadata so the formatter can include method and line.
func PrintInfo(ctx *gin.Context, message string) {
	method, line := utilities.TraceCaller(2)
	ctx.Set(methodKey, method)
	ctx.Set(lineKey, line)
	ctx.Set(InfoLevel, message)
}

// PrintDebug schedules a debug-level log message for the current Gin request
// and stores the caller metadata used by the formatter.
func PrintDebug(ctx *gin.Context, message string) {
	method, line := utilities.TraceCaller(2)
	ctx.Set(methodKey, method)
	ctx.Set(lineKey, line)
	ctx.Set(DebugLevel, message)
}

// PrintWarn schedules a warn-level log message for the current Gin request and
// stores the caller metadata used by the formatter.
func PrintWarn(ctx *gin.Context, message string) {
	method, line := utilities.TraceCaller(2)
	ctx.Set(methodKey, method)
	ctx.Set(lineKey, line)
	ctx.Set(WarnLevel, message)
}

// PrintError schedules an error-level log message for the current Gin request
// and stores the caller metadata used by the formatter.
func PrintError(ctx *gin.Context, err error) {
	method, line := utilities.TraceCaller(2)
	ctx.Set(methodKey, method)
	ctx.Set(lineKey, line)
	ctx.Set(ErrorLevel, err)
}
