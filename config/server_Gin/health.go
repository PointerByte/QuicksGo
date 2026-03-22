// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package server_Gin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

var customHealthHandler gin.HandlerFunc

// SetCustomHealthHandler replaces the default `/health` endpoint handler used
// for every configured route group.
func SetCustomHealthHandler(resp gin.HandlerFunc) {
	customHealthHandler = resp
}

func health() gin.HandlerFunc {
	if customHealthHandler != nil {
		return customHealthHandler
	}
	return func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"aplicacion": viper.GetString("app.name"),
			"appVersion": viper.GetString("app.version"),
		})
	}

}
