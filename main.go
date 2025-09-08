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

func loadConfig() error {
	viper.SetConfigName("application")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	return viper.ReadInConfig()
}

func createApp() (*http.Server, *gin.RouterGroup) {
	// Mode Gin
	gin.SetMode(gin.ReleaseMode)
	// Initialize engine
	engine := gin.New()
	// Middlewares: gin logger + recovery
	engine.Use(logger.CustomLogFormat(), gin.Recovery(), security.SecurityHeaders())
	// Grupo de API
	apiGroup := engine.Group("/api")
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

func shutdown(srv *http.Server) error {
	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	//log.Println("Recibida señal, apagando con gracia...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown force: %v", err)
	}
	//log.Println("Servidor detenido correctamente ✅")
	return nil
}

func main() {
	if err := loadConfig(); err != nil {
		log.Fatal(err)
	}
	srv, _ := createApp()
	go func() {
		//log.Printf("Servidor escuchando en http://localhost:%s (mode=%s)", port, gin.Mode())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("error starting server: %v", err)
		}
	}()
	if err := shutdown(srv); err != nil {
		log.Fatal(err)
	}
}
