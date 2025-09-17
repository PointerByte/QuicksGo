package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func InitLogger() (*os.File, error) {
	// Create/open log file
	path := filepath.Join(viper.GetString("logger.dir"), viper.GetString("service.name")+".log")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("could not open %s: %v", path, err)
	}

	// MultiWriter -> archivo + consola
	mw := io.MultiWriter(f, os.Stdout)

	// Redirect Gin logs to file and console at the same time
	gin.DefaultWriter = mw
	gin.DefaultErrorWriter = mw

	// Logs del paquete estándar log
	log.SetOutput(mw)
	log.SetFlags(0)
	log.SetPrefix("")

	return f, nil
}
