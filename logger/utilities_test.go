package logger

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"quicksgo/telemetry"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

// ---- Mocks específicos para otel log ----

// MockExporter implementa sdklog.Exporter
type MockExporter struct {
	mock.Mock
}

func (m *MockExporter) Export(ctx context.Context, recs []sdklog.Record) error {
	args := m.Called(ctx, recs)
	return args.Error(0)
}

func (m *MockExporter) ForceFlush(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockExporter) Shutdown(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestClearFile(t *testing.T) {
	tmpDir := t.TempDir()
	validPath := filepath.Join(tmpDir, "valid.log")
	invalidPath := string([]byte{0}) // Invalid path

	// Prepara archivo válido con contenido
	if err := os.WriteFile(validPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to write to test file: %v", err)
	}

	tests := []struct {
		name       string
		setupViper func()
		wantErr    bool
	}{
		{
			name: "success",
			setupViper: func() {
				viper.Set("logger.path", validPath)
			},
			wantErr: false,
		},
		{
			name: "open fails",
			setupViper: func() {
				viper.Set("logger.path", invalidPath)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupViper()
			err := ClearFile()
			if (err != nil) != tt.wantErr {
				t.Errorf("ClearFile() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func Test_emitOtel(t *testing.T) {
	viper.Set("service.name", "unit-test-service")
	viper.Set("otlp.enable", true)

	t.Run("sin provider: no panica", func(t *testing.T) {
		// Aún no seteamos el provider (GetLoggerProvider() == nil).
		assert.NotPanics(t, func() {
			jsonBytes, _ := json.Marshal(LogFormat{Message: "no provider"})
			_emitOtel(context.Background(), "", "", INFO, string(jsonBytes))
		})
	})

	t.Run("con provider: cubre INFO/WARN/ERROR/FATAL/default y Export n veces", func(t *testing.T) {
		// Arrange: exporter mock + processor real
		exp := new(MockExporter)

		// Casos a cubrir → harán que pase por cada rama del switch (líneas “rojas”)
		cases := []struct {
			name  string
			level level
		}{
			{"info", INFO},
			{"warn", WARNING},
			{"error", ERROR},
			{"fatal", FATAL},
			{"default", UNKNOWN}, // cae en default/undefined
		}

		// Esperamos una llamada a Export por cada caso
		callCount := len(cases)
		// Puedes usar Times(callCount) o un Run para chequear que siempre haya >=1 record
		exp.On("Export", mock.Anything, mock.Anything).Times(callCount).Return(nil).Run(
			func(args mock.Arguments) {
				// Sanidad: Export recibe un slice con al menos 1 record
				if recs, ok := args.Get(1).([]sdklog.Record); ok {
					if len(recs) == 0 {
						t.Errorf("Export recibió slice vacío de records")
					}
				}
			},
		)
		exp.On("ForceFlush", mock.Anything).Maybe().Return(nil)
		exp.On("Shutdown", mock.Anything).Maybe().Return(nil)

		proc := sdklog.NewSimpleProcessor(exp)
		lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(proc))

		// Setear provider UNA sola vez (el paquete telemetry usa sync.Once)
		telemetry.SetLoggerProvider(lp)

		// Act: emitir un log por cada severidad para cubrir todas las ramas
		for _, tc := range cases {
			jsonBytes, _ := json.Marshal(LogFormat{
				TraceID: "trace-123",
				SpanID:  "span-456",
				Level:   tc.level,
				Message: "cover " + tc.name,
			})
			_emitOtel(context.Background(), "", "", tc.level, string(jsonBytes))
		}

		// Assert
		exp.AssertNumberOfCalls(t, "Export", callCount)
		exp.AssertExpectations(t)
	})
}
