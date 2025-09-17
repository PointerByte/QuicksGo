package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"quicksgo/common"
	"quicksgo/logger"
	"quicksgo/security"
	"strings"
	"syscall"
	"time"

	"github.com/Cyprinus12138/otelgin"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func LoadConfig() error {
	viper.SetConfigFile(".env.local")
	viper.SetConfigType("env")
	viper.SetConfigName("application")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	return viper.ReadInConfig()
}

func CreateApp() (*http.Server, *gin.RouterGroup) {
	// Mode Gin
	gin.SetMode(gin.ReleaseMode)
	// Initialize engine
	engine := gin.New()
	// Middlewares: gin logger + recovery
	group := viper.GetString("server.group")
	engine.Use(
		logger.CustomLogFormat([]string{group + "/status"}),
		gin.Recovery(), security.SecurityHeaders(),
	)
	// Grupo de API
	apiGroup := engine.Group(group)
	// Health / ready / version
	apiGroup.GET("/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"version": viper.GetString("service.version")})
	})
	// JWT Auth (proximamente)
	//engine.Use(authFunc())
	if viper.GetBool("otel.exporter.enable") {
		engine.Use(otelgin.Middleware(viper.GetString("server.name")))
	}
	srv := &http.Server{
		Addr:              viper.GetString("server.port"),
		Handler:           engine,
		ReadHeaderTimeout: common.ReadHeaderTimeout,
	}
	return srv, apiGroup
}

func Shutdown(srv *http.Server) error {
	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logger.Info(ctx, "Signal received, turning off...")
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown force: %v", err)
	}
	logger.Info(ctx, "Server stopped successfully ✅")
	return nil
}

func main() {
	if err := LoadConfig(); err != nil {
		log.Fatal(err)
	}

	file, err := logger.InitLogger()
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	logger.Info(context.Background(), fmt.Sprintf("Starting Server on port %s", viper.GetString("server.port")))
	srv, _ := CreateApp()
	defer Shutdown(srv)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
