// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package server_Gin

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

var customNoMethodHandler gin.HandlerFunc

// SetNoMethod overrides the default handler used when a route exists but the
// HTTP method is not allowed.
func SetNoMethod(handler gin.HandlerFunc) {
	customNoMethodHandler = handler
}

func noMethod() gin.HandlerFunc {
	if customNoMethodHandler != nil {
		return customNoMethodHandler
	}
	return func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"message": "Method not allow",
		})
	}
}
