// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

// Package middlewares provides HTTP and gRPC middleware used by the logger
// module.
//
// It creates request-scoped logger contexts, captures request and response
// payloads when enabled, and emits structured logs enriched with distributed
// tracing metadata.
//
// Main entry points:
//   - InitLogger to initialize the request-scoped logger context for Gin
//   - CaptureBody to capture HTTP request and response payloads
//   - LoggerWithConfig to emit the final structured HTTP log entry
//   - UnaryServerInterceptor and StreamServerInterceptor for gRPC logging
package middlewares
