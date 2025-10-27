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

var std = log.New(io.Discard, "", 0)

func initLogger() error {
	// Create/open log file
	dir := viper.GetString("logger.dir")
	filePath := viper.GetString("service.name") + ".log"
	fileStr := filepath.Join(dir, filePath)

	// Create folder if it does not exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create dir %s: %v", dir, err)
	}

	// Save path complete
	bsFile, _ := filepath.Abs(fileStr)
	dir = filepath.Dir(bsFile)
	viper.SetDefault("logger.path", filepath.Join(dir, filePath))

	// Open or create file
	f, err := os.OpenFile(fileStr, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("could not open %s: %v", fileStr, err)
	}
	viper.SetDefault("fileLog", f)

	// MultiWriter -> file + console
	mw := io.MultiWriter(f, os.Stdout)

	// Our dedicated logger
	std.SetOutput(mw)
	std.SetFlags(0)

	// Redirect Gin logs to file and console at the same time
	gin.DefaultWriter = mw
	gin.DefaultErrorWriter = mw

	//  Standard log package logs
	log.SetOutput(mw)
	log.SetFlags(0)
	log.SetPrefix("")

	// Config otlp logs
	return nil
}
