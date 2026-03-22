// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package server_Gin

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/PointerByte/QuicksGo/logger/builder"
	viperdata "github.com/PointerByte/QuicksGo/logger/viperData"
	jwtservice "github.com/PointerByte/QuicksGo/security/auth/jwt"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

func resetServerTestState(t *testing.T) {
	t.Helper()

	origLoadEnv := loadEnv
	origInitLogger := initLogger
	origInitOtel := initOtel
	origSleepFn := sleepFn
	origListenAndServeFn := listenAndServeFn
	origShutdownServerFn := shutdownServerFn
	origBuilderNewFn := builderNewFn
	origStartJobsFn := startJobsFn
	origStopFn := stopFn
	origWaitForShutdownSignalFn := waitForShutdownSignalFn
	origRunAsyncFn := runAsyncFn
	origQuit := quit
	origEngine := engine
	origTLSConfig := tlsConfig
	origShutdownList := shutdownList
	origHealthHandler := customHealthHandler
	origNoMethodHandler := customNoMethodHandler
	origNoRouteHandler := customNoRouteHandler
	origGlobalRoute := globalRoute
	origGinMode := gin.Mode()

	viper.Reset()
	viperdata.ResetViperDataSingleton()
	engine = nil
	tlsConfig = nil
	shutdownList = nil
	customHealthHandler = nil
	customNoMethodHandler = nil
	customNoRouteHandler = nil
	globalRoute = nil
	quit = make(chan os.Signal, 1)
	loadEnv = utilitiesLoadEnvNoop
	initLogger = builder.InitLogger
	initOtel = tracesInitNoop
	sleepFn = time.Sleep
	listenAndServeFn = func(srv *http.Server) error { return srv.ListenAndServe() }
	shutdownServerFn = func(srv *http.Server, ctx context.Context) error { return srv.Shutdown(ctx) }
	builderNewFn = builder.New
	startJobsFn = func() {}
	stopFn = Stop
	waitForShutdownSignalFn = waitForShutdownSignal
	runAsyncFn = func(fn func()) { go fn() }
	gin.SetMode(gin.TestMode)
	viper.Set("app.name", "test-app")
	viper.Set("app.version", "1.0.0")
	builder.EnableModeTest()

	t.Cleanup(func() {
		loadEnv = origLoadEnv
		initLogger = origInitLogger
		initOtel = origInitOtel
		sleepFn = origSleepFn
		listenAndServeFn = origListenAndServeFn
		shutdownServerFn = origShutdownServerFn
		builderNewFn = origBuilderNewFn
		startJobsFn = origStartJobsFn
		stopFn = origStopFn
		waitForShutdownSignalFn = origWaitForShutdownSignalFn
		runAsyncFn = origRunAsyncFn
		quit = origQuit
		engine = origEngine
		tlsConfig = origTLSConfig
		shutdownList = origShutdownList
		customHealthHandler = origHealthHandler
		customNoMethodHandler = origNoMethodHandler
		customNoRouteHandler = origNoRouteHandler
		globalRoute = origGlobalRoute
		gin.SetMode(origGinMode)
		viper.Reset()
		viperdata.ResetViperDataSingleton()
	})
}

func utilitiesLoadEnvNoop(string) error {
	return nil
}

func tracesInitNoop(context.Context) (func(context.Context) error, error) {
	return func(context.Context) error { return nil }, nil
}

func writeApplicationJSON(t *testing.T, dir string, data map[string]any) {
	t.Helper()

	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "application.json"), payload, 0o600); err != nil {
		t.Fatalf("write application.json: %v", err)
	}
}

func newLoggerProviderNoop(context.Context, string) (*sdklog.LoggerProvider, error) {
	return sdklog.NewLoggerProvider(), nil
}

func TestLoadConfigDefaultGin(t *testing.T) {
	resetServerTestState(t)

	loadConfigDefaultGin()

	if got := viper.GetString("server.gin.mode"); got != gin.ReleaseMode {
		t.Fatalf("expected release mode, got %q", got)
	}
	if !viper.GetBool("server.gin.UseH2C") {
		t.Fatal("expected server.gin.UseH2C default true")
	}
	if got := viper.GetInt("server.gin.rate.limit"); got != 1000 {
		t.Fatalf("expected limit 1000, got %d", got)
	}
	if got := viper.GetInt("server.gin.rate.burst"); got != 2000 {
		t.Fatalf("expected burst 2000, got %d", got)
	}
}

func TestLoadConfig(t *testing.T) {
	t.Run("load env error", func(t *testing.T) {
		resetServerTestState(t)
		wantErr := errors.New("load env")
		loadEnv = func(string) error { return wantErr }

		err := loadConfig(t.TempDir())
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected %v, got %v", wantErr, err)
		}
	})

	t.Run("init logger error", func(t *testing.T) {
		resetServerTestState(t)
		dir := t.TempDir()
		writeApplicationJSON(t, dir, map[string]any{"logger": map[string]any{"dir": "logs"}})
		wantErr := errors.New("logger")
		initLogger = func(context.Context, string) (*sdklog.LoggerProvider, error) {
			return nil, wantErr
		}

		err := loadConfig(dir)
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected %v, got %v", wantErr, err)
		}
	})

	t.Run("init otel error", func(t *testing.T) {
		resetServerTestState(t)
		dir := t.TempDir()
		writeApplicationJSON(t, dir, map[string]any{"logger": map[string]any{"dir": "logs"}})
		wantErr := errors.New("otel")
		initLogger = newLoggerProviderNoop
		initOtel = func(context.Context) (func(context.Context) error, error) {
			return nil, wantErr
		}

		err := loadConfig(dir)
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected %v, got %v", wantErr, err)
		}
	})

	t.Run("success", func(t *testing.T) {
		resetServerTestState(t)
		dir := t.TempDir()
		writeApplicationJSON(t, dir, map[string]any{
			"app":    map[string]any{"name": "svc"},
			"logger": map[string]any{"dir": "logs"},
		})
		initLogger = newLoggerProviderNoop
		initOtel = tracesInitNoop

		if err := loadConfig(dir); err != nil {
			t.Fatalf("loadConfig returned error: %v", err)
		}
		if len(shutdownList) != 2 {
			t.Fatalf("expected 2 shutdown handlers, got %d", len(shutdownList))
		}
	})
}

func TestLimiter(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		resetServerTestState(t)
		viper.Set("server.gin.rate.limit", 0)

		router := gin.New()
		router.Use(limiter())
		router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", rec.Code)
		}
	})

	t.Run("rate limited", func(t *testing.T) {
		resetServerTestState(t)
		viper.Set("server.gin.rate.limit", 1)
		viper.Set("server.gin.rate.burst", 1)

		router := gin.New()
		router.Use(limiter())
		router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

		rec1 := httptest.NewRecorder()
		router.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec1.Code != http.StatusNoContent {
			t.Fatalf("expected first request 204, got %d", rec1.Code)
		}

		rec2 := httptest.NewRecorder()
		router.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec2.Code != http.StatusTooManyRequests {
			t.Fatalf("expected second request 429, got %d", rec2.Code)
		}
	})
}

func TestHandlersAndRoutes(t *testing.T) {
	t.Run("default health handler", func(t *testing.T) {
		resetServerTestState(t)
		viper.Set("app.name", "svc")
		viper.Set("app.version", "1.0.0")

		router := gin.New()
		router.GET("/health", health())

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if body := rec.Body.String(); body == "" {
			t.Fatal("expected body")
		}
	})

	t.Run("custom health handler", func(t *testing.T) {
		resetServerTestState(t)
		SetCustomHealthHandler(func(c *gin.Context) { c.Status(http.StatusCreated) })

		router := gin.New()
		router.GET("/health", health())
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", rec.Code)
		}
	})

	t.Run("no method default and custom", func(t *testing.T) {
		resetServerTestState(t)
		router := gin.New()
		router.HandleMethodNotAllowed = true
		router.NoMethod(noMethod())
		router.GET("/items", func(c *gin.Context) { c.Status(http.StatusOK) })

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/items", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected default 200, got %d", rec.Code)
		}

		SetNoMethod(func(c *gin.Context) { c.Status(http.StatusAccepted) })
		router = gin.New()
		router.HandleMethodNotAllowed = true
		router.NoMethod(noMethod())
		router.GET("/items", func(c *gin.Context) { c.Status(http.StatusOK) })
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/items", nil))
		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected custom 202, got %d", rec.Code)
		}
	})

	t.Run("no route default and custom", func(t *testing.T) {
		resetServerTestState(t)
		router := gin.New()
		router.NoRoute(notFound())

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected default 200, got %d", rec.Code)
		}

		SetNoRoute(func(c *gin.Context) { c.Status(http.StatusNotFound) })
		router = gin.New()
		router.NoRoute(notFound())
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected custom 404, got %d", rec.Code)
		}
	})

	t.Run("set and get route", func(t *testing.T) {
		resetServerTestState(t)
		router := gin.New()
		group := router.Group("/api")
		setRoute(map[string]*gin.RouterGroup{"/api": group})
		if got := GetRoute("/api"); got != group {
			t.Fatal("expected same route group")
		}
		if got := GetRoute("/missing"); got != nil {
			t.Fatalf("expected nil missing route, got %#v", got)
		}
	})
}

func TestCreateApp(t *testing.T) {
	t.Run("load config error", func(t *testing.T) {
		resetServerTestState(t)
		wantErr := errors.New("config error")
		loadEnv = func(string) error { return wantErr }

		srv, err := createApp()
		if !errors.Is(err, wantErr) || srv != nil {
			t.Fatalf("expected config error, got srv=%v err=%v", srv, err)
		}
	})

	t.Run("success builds server and routes", func(t *testing.T) {
		resetServerTestState(t)
		loadEnv = func(string) error {
			viper.Set("logger.dir", "logs")
			viper.Set("server.port", ":8080")
			viper.Set("server.gin.port", ":8080")
			viper.Set("server.groups", []string{"/api/v1"})
			viper.Set("server.gin.UseH2C", true)
			viper.Set("jwt.enable", false)
			viper.Set("app.name", "svc")
			viper.Set("app.version", "1.0.0")
			return nil
		}
		initLogger = newLoggerProviderNoop
		initOtel = tracesInitNoop

		srv, err := createApp()
		if err != nil {
			t.Fatalf("createApp returned error: %v", err)
		}
		if srv == nil || srv.Handler == nil {
			t.Fatal("expected configured server")
		}
		if srv.Addr != ":8080" {
			t.Fatalf("expected :8080, got %q", srv.Addr)
		}
		if GetEngine() == nil {
			t.Fatal("expected engine to be set")
		}
		if !GetEngine().UseH2C {
			t.Fatal("expected UseH2C enabled")
		}
		if GetRoute("/api/v1") == nil {
			t.Fatal("expected route group to exist")
		}

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		GetEngine().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected health endpoint 200, got %d", rec.Code)
		}
	})

	t.Run("uses preconfigured tls config when auto tls disabled", func(t *testing.T) {
		resetServerTestState(t)
		loadEnv = func(string) error {
			viper.Set("logger.dir", "logs")
			viper.Set("server.port", ":8443")
			viper.Set("server.gin.port", ":8443")
			viper.Set("server.groups", []string{"/api/v1"})
			viper.Set("gin.autotls.enable", false)
			return nil
		}
		initLogger = newLoggerProviderNoop
		initOtel = tracesInitNoop

		expectedTLS := &tls.Config{MinVersion: tls.VersionTLS12}
		SetTLSsConfig(expectedTLS)

		srv, err := createApp()
		if err != nil {
			t.Fatalf("createApp returned error: %v", err)
		}
		if srv.TLSConfig != expectedTLS {
			t.Fatal("expected custom tls config to be preserved")
		}
	})

	t.Run("supports jwt cookie transport", func(t *testing.T) {
		resetServerTestState(t)
		loadEnv = func(string) error {
			viper.Set("logger.dir", "logs")
			viper.Set("server.port", ":8080")
			viper.Set("server.gin.port", ":8080")
			viper.Set("server.groups", []string{"/api/v1"})
			viper.Set("server.gin.UseH2C", true)
			viper.Set("jwt.enable", true)
			viper.Set("jwt.transport", "cookie")
			viper.Set("jwt.algorithm", "HS256")
			viper.Set("jwt.hmac.secret", "cookie-secret")
			viper.Set("jwt.cookie.name", "session_token")
			viper.Set("app.name", "svc")
			viper.Set("app.version", "1.0.0")
			return nil
		}
		initLogger = newLoggerProviderNoop
		initOtel = tracesInitNoop

		srv, err := createApp()
		if err != nil {
			t.Fatalf("createApp returned error: %v", err)
		}
		if srv == nil || srv.Handler == nil {
			t.Fatal("expected configured server")
		}

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		GetEngine().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected missing cookie request 401, got %d", rec.Code)
		}

		jwtService, err := jwtservice.NewConfiguredService(jwtservice.ConfigServiceInput{})
		if err != nil {
			t.Fatalf("expected jwt service without error, got %v", err)
		}
		token, err := jwtService.Create(map[string]any{"user_id": "42"})
		if err != nil {
			t.Fatalf("expected token without error, got %v", err)
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
		GetEngine().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected cookie-authenticated health endpoint 200, got %d", rec.Code)
		}
	})
}

func TestTLSConfigurationHelpers(t *testing.T) {
	t.Run("SetTLSsConfig", func(t *testing.T) {
		resetServerTestState(t)

		expected := &tls.Config{MinVersion: tls.VersionTLS13}
		SetTLSsConfig(expected)

		if tlsConfig != expected {
			t.Fatal("expected tlsConfig to match injected config")
		}
	})

	tests := []struct {
		name       string
		enabled    bool
		version    string
		wantNil    bool
		wantMinTLS uint16
	}{
		{name: "disabled", enabled: false, wantNil: true},
		{name: "tls10", enabled: true, version: "tlsv10", wantMinTLS: tls.VersionTLS10},
		{name: "tls11", enabled: true, version: "tlsv11", wantMinTLS: tls.VersionTLS11},
		{name: "tls12", enabled: true, version: "tlsv12", wantMinTLS: tls.VersionTLS12},
		{name: "tls13", enabled: true, version: "tlsv13", wantMinTLS: tls.VersionTLS13},
		{name: "unknown version", enabled: true, version: "unknown", wantMinTLS: 0},
	}

	for _, tt := range tests {
		t.Run("enableAutoTLS "+tt.name, func(t *testing.T) {
			resetServerTestState(t)
			viper.Set("gin.autotls.enable", tt.enabled)
			viper.Set("gin.autotls.version", tt.version)
			viper.Set("gin.autotls.domain", "example.com")
			viper.Set("gin.autotls.dirCache", t.TempDir())

			resolveTLSAutoConfig()

			if tt.wantNil {
				if tlsConfig != nil {
					t.Fatalf("expected nil tlsConfig, got %#v", tlsConfig)
				}
				return
			}

			if tlsConfig == nil {
				t.Fatal("expected tlsConfig to be initialized")
			}
			if tlsConfig.GetCertificate == nil {
				t.Fatal("expected GetCertificate to be configured")
			}
			if tlsConfig.MinVersion != tt.wantMinTLS {
				t.Fatalf("expected MinVersion %d, got %d", tt.wantMinTLS, tlsConfig.MinVersion)
			}
		})
	}
}

func TestStartAndShutdown(t *testing.T) {
	t.Run("start without tls", func(t *testing.T) {
		resetServerTestState(t)
		var listenCalls int32
		var jobsCalls int32
		var stopCalls int32
		var shutdownCalls int32

		runAsyncFn = func(fn func()) { go fn() }
		listenAndServeFn = func(*http.Server) error {
			atomic.AddInt32(&listenCalls, 1)
			return http.ErrServerClosed
		}
		startJobsFn = func() {
			atomic.AddInt32(&jobsCalls, 1)
		}
		waitForShutdownSignalFn = func() {
			atomic.AddInt32(&stopCalls, 1)
		}
		shutdownServerFn = func(*http.Server, context.Context) error {
			atomic.AddInt32(&shutdownCalls, 1)
			return nil
		}

		viper.Set("server.gin.port", ":7070")
		start(&http.Server{})

		if atomic.LoadInt32(&jobsCalls) != 1 {
			t.Fatalf("expected 1 jobs start, got %d", jobsCalls)
		}
		if atomic.LoadInt32(&stopCalls) != 1 {
			t.Fatalf("expected 1 stop call, got %d", stopCalls)
		}
		if atomic.LoadInt32(&shutdownCalls) != 1 {
			t.Fatalf("expected 1 shutdown call, got %d", shutdownCalls)
		}

		time.Sleep(20 * time.Millisecond)
		if atomic.LoadInt32(&listenCalls) != 1 {
			t.Fatalf("expected 1 listen call, got %d", listenCalls)
		}
	})

	t.Run("start with tls listens twice in current implementation", func(t *testing.T) {
		resetServerTestState(t)
		var listenCalls int32

		runAsyncFn = func(fn func()) { fn() }
		listenAndServeFn = func(*http.Server) error {
			atomic.AddInt32(&listenCalls, 1)
			return http.ErrServerClosed
		}
		startJobsFn = func() {}
		waitForShutdownSignalFn = func() {}
		shutdownServerFn = func(*http.Server, context.Context) error { return nil }

		start(&http.Server{TLSConfig: &tls.Config{}})

		if atomic.LoadInt32(&listenCalls) != 2 {
			t.Fatalf("expected 2 listen calls for tls branch, got %d", listenCalls)
		}
	})

	t.Run("shutdown server error path", func(t *testing.T) {
		resetServerTestState(t)
		waitForShutdownSignalFn = func() {}
		wantErr := errors.New("shutdown")
		shutdownServerFn = func(*http.Server, context.Context) error {
			return wantErr
		}

		shutdownFn(&http.Server{})
	})
}

func TestStartPanicsOnUnexpectedListenError(t *testing.T) {
	resetServerTestState(t)

	waitForShutdownSignalFn = func() {}
	startJobsFn = func() {}
	shutdownServerFn = func(*http.Server, context.Context) error { return nil }
	runAsyncFn = func(fn func()) { fn() }
	listenAndServeFn = func(*http.Server) error {
		return errors.New("boom")
	}

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected panic")
		}
	}()

	start(&http.Server{})
}

func TestStopAndSetModeTest(t *testing.T) {
	t.Run("stop sends signal", func(t *testing.T) {
		resetServerTestState(t)
		sleepFn = func(time.Duration) {}
		quit = make(chan os.Signal, 1)

		Stop()

		select {
		case sig := <-quit:
			if sig != syscall.SIGTERM {
				t.Fatalf("expected SIGTERM, got %v", sig)
			}
		default:
			t.Fatal("expected signal to be sent")
		}
	})

	t.Run("set mode test", func(t *testing.T) {
		resetServerTestState(t)
		SetModeTest()

		if got := gin.Mode(); got != gin.TestMode {
			t.Fatalf("expected gin test mode, got %q", got)
		}
		if !viper.GetBool("server.modeTest") {
			t.Fatal("expected server.modeTest true")
		}
		if viper.GetBool("aws.logger.upload") {
			t.Fatal("expected aws.logger.upload false")
		}
	})
}
