package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/ManuelXL56/logger"
	"github.com/ManuelXL56/rotate"
)

func main() {
	// Initialize log rotation: 10 MB per file in "logs" directory
	rotator, err := rotate.New("logs", "logger.log", 100*(1<<10))
	if err != nil {
		log.Fatal(err)
	}

	// Create a logger that writes JSON at Debug level to the rotator
	log := logger.New(
		logger.WithOutput(rotator),
		logger.WithLevel(logger.InfoLevel),
		logger.WithJSON(),
	)

	log.Info("Application started", "pid", os.Getpid())
	log.Debug("Debugging mode on")
	log.Warn("Low disk space", "path", filepath.Join("logs"))
	log.Error("An error occurred", "error", "sample error")

}
