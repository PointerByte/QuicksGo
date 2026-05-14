// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package middlewares

type keyContex string

const (
	traceIDKey      keyContex = "traceID"
	detailsKey      keyContex = "details"
	disableBodyKey  keyContex = "disableBodyKey"
	requestBodyKey  keyContex = "requestBody"
	responseBodyKey keyContex = "responseBody"
	methodKey       keyContex = "method"
	lineKey         keyContex = "line"
)
