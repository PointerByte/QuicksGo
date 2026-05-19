// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package grpc

import (
	"github.com/PointerByte/GoForge/logger/builder"
	"github.com/PointerByte/GoForge/logger/middlewares/common"
)

// DisableBody marks the current gRPC request-scoped logger context so the final
// structured log can omit request and response bodies independently.
func DisableBody(ctxLogger *builder.Context, disableRequestBody bool, disableResponseBody bool) {
	ctxLogger.Set(common.DisableRequestBodyKey, disableRequestBody)
	ctxLogger.Set(common.DisableResponseBodyKey, disableResponseBody)
}

// DisableTraceBody marks whether downstream trace services should omit their
// request and response bodies when builder.Context.TraceEnd finishes a trace.
func DisableTraceBody(ctxLogger *builder.Context, disableRequestBody bool, disableResponseBody bool) {
	ctxLogger.Set(string(common.DisableTraceRequestBodyKey), disableRequestBody)
	ctxLogger.Set(string(common.DisableTraceResponseBodyKey), disableResponseBody)
}
