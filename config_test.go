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
		ctxLogger.Fatal(err)
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

func TestLimiter(t *testing.T) {
	type args struct {
		rate     float64
		burst    int
		requests int
	}
	tests := []struct {
		name         string
		args         args
		wantStatuses []int // expected HTTP status per request
		wantHeader   bool  // whether the handler should have been executed (checks header)
	}{
		{
			name: "disabled_allows_and_calls_next",
			args: args{
				rate:     0, // disabled
				burst:    10,
				requests: 1,
			},
			wantStatuses: []int{http.StatusOK},
			wantHeader:   true, // handler must run when disabled
		},
		{
			name: "enabled_blocks_second_immediate_request",
			args: args{
				rate:     1, // 1 req/s
				burst:    1, // allow one instantly, block next if immediate
				requests: 2,
			},
			wantStatuses: []int{http.StatusOK, http.StatusTooManyRequests},
			wantHeader:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Aislar configuración entre tests
			viper.Set("server.gin.ratelimit", tt.args.rate)
			viper.Set("server.gin.bursts", tt.args.burst)

			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(Limiter())

			// Handler que marca un header para saber si se ejecutó
			const headerKey = "X-Handler-Executed"
			r.GET("/ping", func(c *gin.Context) {
				c.Header(headerKey, "1")
				c.Status(http.StatusOK)
			})

			statuses := make([]int, 0, tt.args.requests)
			var sawHeader bool

			for i := 0; i < tt.args.requests; i++ {
				req := httptest.NewRequest(http.MethodGet, "/ping", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				statuses = append(statuses, w.Code)
				if w.Header().Get(headerKey) == "1" {
					sawHeader = true
				}
			}

			// Verificar códigos de estado
			if len(statuses) != len(tt.wantStatuses) {
				t.Fatalf("unexpected statuses length: got %d, want %d", len(statuses), len(tt.wantStatuses))
			}
			for i := range statuses {
				if statuses[i] != tt.wantStatuses[i] {
					t.Errorf("request #%d: status = %d, want %d", i+1, statuses[i], tt.wantStatuses[i])
				}
			}

			// Verificar si el handler se ejecutó al menos una vez cuando corresponde
			if sawHeader != tt.wantHeader {
				t.Errorf("handler executed header = %v, want %v", sawHeader, tt.wantHeader)
			}
		})
	}
}
