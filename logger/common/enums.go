// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package common

type KeyContex string

const (
	TraceIDKey                  KeyContex = "traceID"
	DetailsKey                  KeyContex = "details"
	DisableRequestBodyKey       KeyContex = "disableRequestBody"
	DisableResponseBodyKey      KeyContex = "disableResponseBody"
	DisableTraceRequestBodyKey  KeyContex = "disableTraceRequestBody"
	DisableTraceResponseBodyKey KeyContex = "disableTraceResponseBody"
	RequestbodyKey              KeyContex = "requestBody"
	ResponsebodyKey             KeyContex = "responseBody"
	MethodKey                   KeyContex = "method"
	LineKey                     KeyContex = "line"
)
