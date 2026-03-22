// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package server_Gin

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

var customNoRouteHandler gin.HandlerFunc

// SetNoRoute overrides the default handler used when no route matches the
// incoming request path.
func SetNoRoute(handler gin.HandlerFunc) {
	customNoRouteHandler = handler
}

func notFound() gin.HandlerFunc {
	if customNoRouteHandler != nil {
		return customNoRouteHandler
	}
	return func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"message": "Path not found",
		})
	}
}
