// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package server_Gin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func health() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"aplicacion": viper.GetString("app.name"),
			"appVersion": viper.GetString("app.version"),
		})
	}

}
