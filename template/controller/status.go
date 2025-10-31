package controller

import (
	"net/http"

	"github.com/PointerByte/QuicksGo/logger"
	"github.com/PointerByte/QuicksGo/models"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func Status(ctx *gin.Context) {
	ctx.Set(logger.WithAutoLog, false)
	resp := models.GenericResponse[map[string]any](models.StatusSuccess, gin.H{"version": viper.GetString("service.version")})
	ctx.JSON(http.StatusOK, resp)
}
