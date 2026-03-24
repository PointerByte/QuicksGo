// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

// Package server assembles the HTTP server, shared middleware, route groups,
// and graceful shutdown flow for the application.
package server_Gin

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/PointerByte/QuicksGo/config/utilities"
	"github.com/PointerByte/QuicksGo/config/utilities/jobs"
	"github.com/PointerByte/QuicksGo/config/utilities/traces"
	"github.com/PointerByte/QuicksGo/logger/builder"
	middlewaresLogger "github.com/PointerByte/QuicksGo/logger/middlewares"
	"github.com/PointerByte/QuicksGo/security/middlewares"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/time/rate"
)

var quit chan os.Signal

func init() {
	quit = make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	mode := viper.GetString("server.gin.mode")
	gin.SetMode(mode)
}

func loadConfigDefaultGin() {
	viper.SetDefault("server.gin.mode", gin.ReleaseMode)
	viper.SetDefault("server.gin.UseH2C", true)
	viper.SetDefault("server.gin.rate.Limit", 1000)
	viper.SetDefault("server.gin.rate.burst", 2000)
	viper.SetDefault("jwt.transport", "header")
}

var sleepFn = time.Sleep
var listenAndServeFn = func(srv *http.Server) error {
	return srv.ListenAndServe()
}
var shutdownServerFn = func(srv *http.Server, ctx context.Context) error {
	return srv.Shutdown(ctx)
}
var builderNewFn = builder.New
var startJobsFn = jobs.StartJobs
var stopFn = Stop
var waitForShutdownSignalFn = waitForShutdownSignal
var runAsyncFn = func(fn func()) {
	go fn()
}

var initLogger = builder.InitLogger

var initOtel = traces.InitOtel

type handlerShutdown func(ctx context.Context) error

var shutdownList []handlerShutdown
var loadEnv = utilities.LoadEnv

func loadConfig(prefixPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := loadEnv(prefixPath); err != nil {
		return err
	}
	path := filepath.Join(prefixPath, viper.GetString("logger.dir"))

	lp, err := initLogger(ctx, path)
	if err != nil {
		return err
	}
	shutdownList = append(shutdownList, lp.Shutdown)

	shutdownOtel, err := initOtel(ctx)
	if err != nil {
		return err
	}
	shutdownList = append(shutdownList, shutdownOtel)
	return nil
}

func limiter() gin.HandlerFunc {
	rateLimit := viper.GetFloat64("server.gin.rate.limit")
	if rateLimit == 0 {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	bursts := viper.GetInt("server.gin.rate.burst")
	rateLimiter := rate.NewLimiter(rate.Limit(rateLimit), bursts)
	return func(c *gin.Context) {
		if !rateLimiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests, please try again later",
			})
			return
		}
		c.Next()
	}
}

var engine *gin.Engine

// GetEngine returns the shared Gin engine initialized by CreateApp.
//
// Its main purpose is to expose the configured router after package startup so
// application code can keep registering routes, groups, middleware, or test
// requests against the same engine instance that the HTTP server will use.
//
// Typical usage is:
//   - call CreateApp to load configuration and build the server
//   - call GetEngine to access the initialized router
//   - register endpoints directly or through groups returned by GetRoute
//   - pass the returned server to Start
//
// It returns nil until CreateApp finishes successfully.
func GetEngine() *gin.Engine {
	return engine
}

var tlsConfig *tls.Config

// SetTLSsConfig sets the TLS configuration that CreateApp should attach to the
// returned http.Server.
//
// It can be used to inject a custom TLS configuration before starting the
// server. If automatic TLS is also enabled, the auto-generated configuration
// may replace the previously assigned value.
func SetTLSsConfig(config *tls.Config) {
	tlsConfig = config
}

// resolveTLSAutoConfig configures a TLS setup backed by autocert when the related
// `gin.autotls.*` settings are enabled.
func resolveTLSAutoConfig() {
	if !viper.GetBool("gin.autotls.enable") {
		return
	}

	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(viper.GetString("gin.autotls.domain")),
		Cache:      autocert.DirCache(viper.GetString("gin.autotls.dirCache")),
	}
	var versionTLS uint16
	switch viper.GetString("gin.autotls.version") {
	case "tlsv10":
		versionTLS = tls.VersionTLS10
	case "tlsv11":
		versionTLS = tls.VersionTLS11
	case "tlsv12":
		versionTLS = tls.VersionTLS12
	case "tlsv13":
		versionTLS = tls.VersionTLS13
	}
	tlsConfig = &tls.Config{
		GetCertificate: m.GetCertificate,
		MinVersion:     versionTLS,
	}
}

// CreateApp builds the configured HTTP server for the application.
//
// It loads configuration files and environment overrides, initializes logging
// and OpenTelemetry, sets up the Gin engine, registers shared middleware, and
// creates the route groups declared in configuration.
var CreateApp = createApp

func createApp(optionsJWT ...middlewares.JWTMiddlewareOption) (*http.Server, error) {
	// Load default config.
	if err := loadConfig("."); err != nil {
		return nil, err
	}

	// Configure env defaults.
	loadConfigDefaultGin()

	// Initialize engine.
	engine = gin.New()
	engine.Use(cors.Default())
	engine.NoRoute(notFound())
	engine.NoMethod(noMethod())
	engine.Use(
		gin.Recovery(),
		authMiddleware(optionsJWT...),
		limiter(),
		middlewares.SecurityHeaders(),
		traces.MiddlewareOtel(),
		middlewaresLogger.InitLogger(),
		middlewaresLogger.LoggerWithConfig(),
		middlewaresLogger.CaptureBody(),
	)
	engine.UseH2C = viper.GetBool("server.gin.UseH2C")

	// Create API route groups.
	routes := make(map[string]*gin.RouterGroup)
	groups := viper.GetStringSlice("server.groups")
	for _, g := range groups {
		routes[g] = engine.Group(g)
		routes[g].GET("/refresh", refreshGin())
		routes[g].GET("/health", healthGin())
	}
	setRoute(routes)
	resolveTLSAutoConfig()
	return &http.Server{
		Addr:      viper.GetString("server.gin.port"),
		Handler:   engine,
		TLSConfig: tlsConfig,
	}, nil
}

func authMiddleware(optionsJWT ...middlewares.JWTMiddlewareOption) gin.HandlerFunc {
	switch strings.ToLower(strings.TrimSpace(viper.GetString("jwt.transport"))) {
	case "cookie", "cookies":
		return middlewares.RequireJWTCookie()
	default:
		return middlewares.RequireJWT(optionsJWT...)
	}
}

// Start runs the provided HTTP server and coordinates the shutdown workflow.
//
// It starts the listener, triggers global jobs startup, logs the configured
// port, and then executes the registered shutdown handlers before stopping the
// server.
var Start = func(srv *http.Server) {
	start(srv)
}

const timeout = 30 * time.Second

func start(srv *http.Server) {
	ctx := context.Background()
	ctxLogger := builderNewFn(ctx)

	// Start server.
	runAsyncFn(func() {
		if srv.TLSConfig != nil {
			if err := listenAndServeFn(srv); err != nil && !errors.Is(err, http.ErrServerClosed) {
				panic(err)
			}
		}
		if err := listenAndServeFn(srv); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(err)
		}
	})

	// Start global jobs.
	startJobsFn()
	port := viper.GetString("server.gin.port")
	ctxLogger.Info(fmt.Sprintf("Server started on port %s", port))

	shutdownFn(srv)
}

func shutdownFn(srv *http.Server) {
	// Graceful shutdown.
	waitForShutdownSignalFn()

	ctx := context.Background()
	ctxLogger := builderNewFn(ctx)

	// Execute shutdown handlers.
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for _, s := range shutdownList {
		s(ctx)
	}

	if err := shutdownServerFn(srv, ctx); err != nil {
		ctxLogger.Error(fmt.Errorf("shutdown force: %v", err))
		return
	}
	ctxLogger.Info("Server stopped successfully")
}

func waitForShutdownSignal() {
	<-quit

	ctx := context.Background()
	ctxLogger := builderNewFn(ctx)
	ctxLogger.Info("Signal received, turning off...")
}

// Stop triggers the package shutdown signal used by the graceful-stop flow.
//
// It sends a SIGTERM-like signal through the internal channel after a short
// delay so the same shutdown path can be used from runtime code and tests.
func Stop() {
	sleepFn(time.Second)
	quit <- syscall.SIGTERM
}

// SetModeTest configures the package for test execution.
//
// It enables logger test mode, switches Gin to test mode, and sets Viper
// defaults that disable behaviors not needed during tests.
func SetModeTest() {
	builder.EnableModeTest()
	gin.SetMode(gin.TestMode)
	viper.SetDefault("server.modeTest", true)
	viper.SetDefault("aws.logger.upload", false)
}
