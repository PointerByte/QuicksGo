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
	dir := viper.GetString("logger.dir")
	path := filepath.Join(dir, viper.GetString("service.name")+".log")

	// Create folder if it does not exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("could not create dir %s: %v", dir, err)
	}

	// Open or create file
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("could not open %s: %v", path, err)
	}

	// MultiWriter -> file + console
	mw := io.MultiWriter(f, os.Stdout)

	// Redirect Gin logs to file and console at the same time
	gin.DefaultWriter = mw
	gin.DefaultErrorWriter = mw

	//  Standard log package logs
	log.SetOutput(mw)
	log.SetFlags(0)
	log.SetPrefix("")

	return f, nil
}
