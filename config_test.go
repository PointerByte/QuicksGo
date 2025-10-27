package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"quicksgo/logger"
	"quicksgo/telemetry"
	"syscall"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func getFreePort() string {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		logger.Fatal(ctxMain, err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return fmt.Sprintf(":%d", addr.Port)
}

func testSetMode(t *testing.T) {
	tests := []struct {
		name string
		mode string
	}{
		{
			name: "debug mode",
			mode: gin.DebugMode,
		},
		{
			name: "release mode",
			mode: gin.ReleaseMode,
		},
		{
			name: "test mode",
			mode: gin.TestMode,
		},
		{
			name: "panic",
			mode: "Diselo al covenant",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.SetDefault("server.gin.mode", tt.mode)
			if tt.name != "panic" {
				setModeGin()
				return
			}
			assert.Panics(t, func() {
				setModeGin()
			}, "expected panic from MustFail")
			logger.ClearFile()
		})
	}
}

func testConfig(t *testing.T) {
	tests2 := []struct {
		name    string
		errs    []error
		wantErr bool
	}{
		{
			name: "Success",
			errs: make([]error, 3),
		},
		{
			name:    "Error ReadInConfig",
			errs:    []error{errors.New("test error"), nil, nil},
			wantErr: true,
		},
		{
			name:    "Error InitLogger",
			errs:    []error{nil, errors.New("test error"), nil},
			wantErr: true,
		},
		{
			name:    "Error InitOtel",
			errs:    []error{nil, nil, errors.New("test error")},
			wantErr: true,
		},
	}
	for _, tt := range tests2 {
		t.Run(tt.name, func(t *testing.T) {
			DisableMocksConfig()
			logger.DisableMocks()
			telemetry.DisableMocks()

			// Set Mocks
			if tt.name == "Error ReadInConfig" {
				EnableMocksConfig()
				MocksConfig.On("ReadInConfig").Return(tt.errs[0]).Maybe()
				// Asserts Mocks
				defer MocksConfig.AssertExpectations(t)
			}
			if tt.name == "Error InitLogger" {
				logger.EnableMocks()
				logger.MocksLogger.On("InitLogger").Return(tt.errs[1]).Maybe()
				// Asserts Mocks
				defer logger.MocksLogger.AssertExpectations(t)
			}
			telemetry.EnableMocks()
			telemetry.MocksOtel.On("InitOtel", mock.Anything).Return(telemetry.HandlerShutdownOtel, tt.errs[2]).Maybe()
			go func() {
				time.Sleep(time.Second)
				quit <- syscall.SIGTERM
			}()

			// Create and start server
			viper.Set("otlp.enable", true)
			viper.Set("server.gin.mode", gin.TestMode)
			engine, err := CreateApp()
			if tt.wantErr {
				assert.Error(t, err)
			}
			srv := &http.Server{
				Addr:    getFreePort(),
				Handler: engine,
			}
			Start(srv)

			// Asserts Mocks
			telemetry.MocksOtel.AssertExpectations(t)
		})
	}
}

func TestMirrorHeaders(t *testing.T) {
	t.Parallel()

	// Configurar Gin en modo Test
	gin.SetMode(gin.TestMode)

	// Crear router con el middleware
	r := gin.New()
	r.Use(MirrorHeaders())

	// Ruta dummy para completar el ciclo del middleware
	r.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	// Crear request con headers de ejemplo
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Test", "HeaderValue")
	req.Header.Set("User-Agent", "Go-Test-Agent")

	// Recorder (mock del ResponseWriter)
	w := httptest.NewRecorder()

	// Ejecutar request
	r.ServeHTTP(w, req)

	// Verificar código HTTP
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Verificar que los headers se copiaron al response
	if got := w.Header().Get("X-Test"); got != "HeaderValue" {
		t.Errorf("expected mirrored header X-Test=HeaderValue, got %q", got)
	}
	if got := w.Header().Get("User-Agent"); got != "Go-Test-Agent" {
		t.Errorf("expected mirrored header User-Agent=Go-Test-Agent, got %q", got)
	}

	// Verificar cuerpo de respuesta
	if body := w.Body.String(); body != "pong" {
		t.Errorf("expected body pong, got %q", body)
	}
}
