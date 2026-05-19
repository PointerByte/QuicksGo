// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"bytes"
	"context"
	"io"

	"github.com/PointerByte/GoForge/logger/builder"
	"github.com/PointerByte/GoForge/logger/common"
	"github.com/PointerByte/GoForge/logger/formatter"
	"github.com/PointerByte/GoForge/logger/utilities"
	viperdata "github.com/PointerByte/GoForge/logger/viperData"
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

		// ---- Default disable request and response bodies ----
		ctx.Set(common.DisableRequestBodyKey, true)
		ctx.Set(common.DisableResponseBodyKey, true)

		// ---- Create logger context with span ----
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
			ctxLogger.Set(common.TraceIDKey, traceID.String())
		}

		details := formatter.Details{
			System:   appName,
			Client:   ctx.ClientIP(),
			Protocol: ctx.Request.Proto,
			Method:   ctx.Request.Method,
			Path:     ctx.Request.URL.Path,
		}
		details.SetHeaders(ctx.Request.Header)
		ctxLogger.Set(common.DetailsKey, details)

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
	body          *bytes.Buffer
	shouldCapture func() bool
}

func (r responseBodyWriter) Write(b []byte) (int, error) {
	if r.shouldCapture() {
		r.body.Write(b)
	}
	return r.ResponseWriter.Write(b)
}

func (r responseBodyWriter) WriteString(s string) (int, error) {
	if r.shouldCapture() {
		r.body.WriteString(s)
	}
	return r.ResponseWriter.WriteString(s)
}

type requestBodyCaptureReadCloser struct {
	io.ReadCloser
	body          *bytes.Buffer
	shouldCapture func() bool
}

func (r *requestBodyCaptureReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 && r.shouldCapture() {
		r.body.Write(p[:n])
	}
	return n, err
}

// CaptureBody captures raw request and response bodies only when body logging
// is enabled for the current request.
//
// Request body capture is lazy: the middleware wraps the request body and only
// stores bytes while the request body flag is enabled. If the handler enables
// request body logging but does not read the body, the middleware drains the
// remaining body after the handler completes so the final log can still include
// it. When request or response body logging is disabled, no payload is stored in
// gin.Context for that side.
func CaptureBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestBody := &bytes.Buffer{}
		var requestCapture *requestBodyCaptureReadCloser
		if c.Request.Body != nil {
			requestCapture = &requestBodyCaptureReadCloser{
				ReadCloser:    c.Request.Body,
				body:          requestBody,
				shouldCapture: func() bool { return shouldCaptureGinBody(c, common.DisableRequestBodyKey) },
			}
			c.Request.Body = requestCapture
		}

		responseBody := &bytes.Buffer{}
		writer := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           responseBody,
			shouldCapture:  func() bool { return shouldCaptureGinBody(c, common.DisableResponseBodyKey) },
		}
		c.Writer = writer

		c.Next()

		if shouldCaptureGinBody(c, common.DisableRequestBodyKey) {
			if requestCapture != nil {
				_, _ = io.Copy(io.Discard, requestCapture)
			}
			c.Set(common.RequestbodyKey, requestBody.String())
		}
		if shouldCaptureGinBody(c, common.DisableResponseBodyKey) {
			c.Set(common.ResponsebodyKey, responseBody.String())
		}
	}
}

func shouldCaptureGinBody(c *gin.Context, key common.KeyContex) bool {
	value, ok := c.Get(key)
	if !ok {
		return false
	}
	disabled, ok := value.(bool)
	return ok && !disabled
}

// LoggerWithConfig emits the final HTTP log entry using Gin's
// LoggerWithConfig hook and the package logger helpers.
//
// Body handling is controlled through disableRequestBodyKey and
// disableResponseBodyKey in gin.Context:
//   - if a flag is present and false, the captured body is copied into details
//   - if a flag is present and true, that body is intentionally omitted
//
// This middleware expects the request-scoped context produced by
// LoggerWithConfig and is commonly paired with MiddlewareCaptureBody.
func LoggerWithConfig() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: func(param gin.LogFormatterParams) string {
			ctx := param.Request.Context()
			ctxLogger := builder.New(ctx)
			if v, ok := param.Keys[common.MethodKey]; ok {
				ctxLogger.Method = v.(string)
			}
			if v, ok := param.Keys[common.LineKey]; ok {
				ctxLogger.Line = v.(int)
			}

			if v, ok := ctxLogger.Get(common.DetailsKey); ok {
				ctxLogger.Details = v.(formatter.Details)
			}
			if v, ok := param.Keys[common.DisableRequestBodyKey]; ok {
				if !v.(bool) {
					var requestBody any
					if param.Keys != nil {
						if v, ok := param.Keys[common.RequestbodyKey]; ok {
							requestBody = v
						}
					}
					ctxLogger.Details.Request = requestBody
				}
			}
			if v, ok := param.Keys[common.DisableResponseBodyKey]; ok {
				if !v.(bool) {
					var responseBody any
					if param.Keys != nil {
						if v, ok := param.Keys[common.ResponsebodyKey]; ok {
							responseBody = v
						}
					}
					ctxLogger.Details.Response = responseBody
				}
			}
			ctxLogger.Set(common.DetailsKey, ctxLogger.Details)

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
	ctxLogger := builder.New(ginRequestContext(ctx))
	method, line := utilities.TraceCaller(ctxLogger.GetTraceCallerSkip())
	ctx.Set(common.MethodKey, method)
	ctx.Set(common.LineKey, line)
	ctx.Set(formatter.InfoLevel, message)
}

// PrintDebug schedules a debug-level log message for the current Gin request
// and stores the caller metadata used by the formatter.
func PrintDebug(ctx *gin.Context, message string) {
	ctxLogger := builder.New(ginRequestContext(ctx))
	method, line := utilities.TraceCaller(ctxLogger.GetTraceCallerSkip())
	ctx.Set(common.MethodKey, method)
	ctx.Set(common.LineKey, line)
	ctx.Set(formatter.DebugLevel, message)
}

// PrintWarn schedules a warn-level log message for the current Gin request and
// stores the caller metadata used by the formatter.
func PrintWarn(ctx *gin.Context, message string) {
	ctxLogger := builder.New(ginRequestContext(ctx))
	method, line := utilities.TraceCaller(ctxLogger.GetTraceCallerSkip())
	ctx.Set(common.MethodKey, method)
	ctx.Set(common.LineKey, line)
	ctx.Set(formatter.WarnLevel, message)
}

// PrintError schedules an error-level log message for the current Gin request
// and stores the caller metadata used by the formatter.
func PrintError(ctx *gin.Context, err error) {
	ctxLogger := builder.New(ginRequestContext(ctx))
	method, line := utilities.TraceCaller(ctxLogger.GetTraceCallerSkip())
	ctx.Set(common.MethodKey, method)
	ctx.Set(common.LineKey, line)
	ctx.Set(formatter.ErrorLevel, err)
}

func ginRequestContext(ctx *gin.Context) context.Context {
	if ctx == nil || ctx.Request == nil {
		return context.Background()
	}
	return ctx.Request.Context()
}
