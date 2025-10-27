package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"quicksgo/logger"
	"quicksgo/security"
	"quicksgo/telemetry"
	"syscall"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

var (
	ctxMain      context.Context
	quit         chan os.Signal
	shutdownOtel telemetry.ShutdownOtel
)

func setModeGin() {
	mode := viper.GetString("server.gin.mode")
	if mode == "" {
		mode = gin.ReleaseMode
	}
	// Mode Gin
	switch mode {
	case gin.DebugMode:
		gin.SetMode(mode)
	case gin.ReleaseMode:
		gin.SetMode(mode)
	case gin.TestMode:
		gin.SetMode(mode)
	default:
		logger.Panic(ctxMain, errors.New("gin mode invalid"))
	}
}

func init() {
	ctxMain = logger.ContextLogger(context.Background())
	quit = make(chan os.Signal, 1)
	setModeGin()
	globalRoutes = make(map[string]*gin.RouterGroup)
}

// LoadConfig loads the configuration file and environment variables into Viper.
// It automatically converts all configuration keys to lowercase to ensure consistency.
// The function supports both .env.local and JSON configuration files.
func LoadConfig() error {
	// Load .env
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()
	// Load .env.local
	viper.SetConfigFile(".env.local")
	viper.SetConfigType("env")
	viper.AutomaticEnv()
	// Load application.json
	viper.SetConfigName("application")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")

	// Read configuration file
	if err := ReadInConfig(); err != nil {
		return err
	}

	// Configure Logs
	err := logger.InitLogger()
	if err != nil {
		return err
	}

	// Config telmetry with Otel
	shutdownOtel, err = telemetry.InitOtel(ctxMain)
	if err != nil {
		return err
	}
	return nil
}

// CreateApp initializes and configures a Gin-based HTTP server instance.
// It loads environment configurations, sets up logging, applies global middlewares,
// and registers basic API routes such as the health/status endpoint.
//
// Returns:
//   - engine: a fully configured *gin.Engine ready to be started.
//   - err: error to config engine gin
//
// This function also sets up security headers and ensures consistent application
// initialization behavior across environments.
func createApp() (*gin.Engine, error) {
	// Configure env
	if err := LoadConfig(); err != nil {
		return nil, err
	}

	// Initialize engine
	engine := gin.New()
	engine.Use(
		gin.Recovery(),
		logger.MiddlewaresInitLogger(),
		logger.CustomLogFormatGin(),
		security.SecurityHeaders(),
		gzip.Gzip(gzip.DefaultCompression),
	)
	if viper.GetBool("otlp.enable") {
		engine.Use(telemetry.GetMiddleware())
	}

	// Grupo form API
	groups := viper.GetStringSlice("server.basePaths")
	for _, group := range groups {
		globalRoutes[group] = engine.Group(group)
	}
	return engine, nil
}

// Shutdown gracefully shuts down the provided HTTP server when an interrupt or termination signal is received.
// It listens for OS signals (SIGINT, SIGTERM), and once triggered, it attempts a graceful shutdown
// with a 10-second timeout to allow ongoing requests to complete.
// If the server fails to shut down within that period, it returns an error.
// Logs are recorded before and after the shutdown process.
func Shutdown(srv *http.Server) error {
	fileLog := viper.Get("fileLog").(*os.File)
	defer fileLog.Close()
	defer logger.ClearFile()

	// Graceful shutdown
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logger.Info(ctx, "Signal received, turning off...")
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown force: %v", err)
	}
	logger.Info(ctx, "Server stopped successfully")
	return nil
}

// Start launches the given HTTP server asynchronously and handles graceful shutdown.
// It checks if TLS is enabled and starts the appropriate listener.
// Once the server is running, it logs the port information.
// The function also handles server shutdown and OpenTelemetry cleanup via the provided shutdownOtel function.
//
// Parameters:
//   - srv: the *http.Server instance to run.
//
// This function blocks until the server is shut down.
// Any errors during server execution or shutdown are logged.
func start(srv *http.Server) {
	defer func() {
		if err := shutdownOtel(ctxMain); err != err {
			logger.Error(ctxMain, err)
		}
	}()
	// Start server
	go func() {
		if srv.TLSConfig != nil {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error(ctxMain, err)
			}
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(ctxMain, err)
		}
	}()
	logger.Info(ctxMain, fmt.Sprintf("Server started on port %s", viper.GetString("server.port")))
	if err := Shutdown(srv); err != err {
		logger.Error(ctxMain, err)
	}
}
