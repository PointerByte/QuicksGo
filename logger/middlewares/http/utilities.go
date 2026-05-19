// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"github.com/PointerByte/GoForge/logger/builder"
	"github.com/PointerByte/GoForge/logger/middlewares/common"
	"github.com/gin-gonic/gin"
)

// DisableBody marks the current Gin request so MiddlewareLoggerWithConfig
// omits request and response bodies from the final log entry.
//
// Internally this stores independent request and response body flags in the
// gin.Context so LoggerWithConfig can decide what to include.
func DisableBody(ctx *gin.Context, disableRequestBody bool, disableResponseBody bool) {
	ctx.Set(common.DisableRequestBodyKey, disableRequestBody)
	ctx.Set(common.DisableResponseBodyKey, disableResponseBody)
}

// DisableTraceBody marks whether downstream trace services should omit their
// request and response bodies when the current request-scoped logger finishes a
// trace with builder.Context.TraceEnd.
//
// Unlike DisableBody, these flags are stored only in the logger context because
// they control formatter.Service trace entries, not the final Gin access log.
func DisableTraceBody(ctx *gin.Context, disableRequestBody bool, disableResponseBody bool) {
	ctxLogger := builder.New(ctx.Request.Context())
	ctxLogger.Set(string(common.DisableTraceRequestBodyKey), disableRequestBody)
	ctxLogger.Set(string(common.DisableTraceResponseBodyKey), disableResponseBody)
}
