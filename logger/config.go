package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func InitLogger() error {
	// Create/open log file
	path := filepath.Join(viper.GetString("logger.dir"), viper.GetString("service.name")+".log")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("could not open %s: %v", path, err)
	}
	defer f.Close()
	// Redirect Gin logs to file and console at the same time
	gin.DefaultWriter = io.MultiWriter(f, os.Stdout)
	return nil
}
