// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

// Package builder provides the runtime integration layer for the logger.
//
// It contains the pieces most applications use directly:
//   - logger initialization through InitLogger
//   - request-scoped context creation through MiddlewareInitLogger
//   - HTTP log emission through MiddlewareLoggerWithConfig
//   - request/response body capture through MiddlewareCaptureBody
//   - convenience logging helpers for Gin handlers
//
// Body capture and body emission are intentionally separate concerns:
// MiddlewareCaptureBody stores the raw request and response payloads in the
// gin.Context, while MiddlewareLoggerWithConfig decides whether those payloads
// are copied into the final structured log.
//
// To explicitly omit request and response bodies from the final log entry for a
// given request, call DisableBody inside the Gin handler or in a middleware
// that runs before the log formatter is executed.
package builder
