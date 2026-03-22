// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"

	viperdata "github.com/PointerByte/QuicksGo/logger/viperData"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
)

func Test_newCofigLoggerProvider(t *testing.T) {
	origNew := new
	origNewLoggerProvider := newLoggerProvider
	origResourceDefault := resourceDefault
	origResourceNewWithAttributes := newSchemaless
	origResourceMerge := resourceMerge

	defer func() {
		new = origNew
		newLoggerProvider = origNewLoggerProvider
		resourceDefault = origResourceDefault
		newSchemaless = origResourceNewWithAttributes
		resourceMerge = origResourceMerge
	}()

	tests := []struct {
		name               string
		setupViper         func()
		setupMocks         func(t *testing.T, wantProvider *sdklog.LoggerProvider)
		wantErr            bool
		wantSameProvider   bool
		wantProviderCalled bool
	}{
		{
			name: "new exporter error",
			setupViper: func() {
				viper.Set("app.name", "test-app")
				viper.Set("app.version", "1.0.0")
			},
			setupMocks: func(t *testing.T, wantProvider *sdklog.LoggerProvider) {
				t.Helper()

				new = func(ctx context.Context, opts ...otlploghttp.Option) (*otlploghttp.Exporter, error) {
					return nil, errors.New("exporter error")
				}
				newLoggerProvider = func(opts ...sdklog.LoggerProviderOption) *sdklog.LoggerProvider {
					t.Fatal("newLoggerProvider should not be called")
					return nil
				}
				resourceDefault = func() *resource.Resource {
					t.Fatal("resourceDefault should not be called")
					return nil
				}
				newSchemaless = func(attrs ...attribute.KeyValue) *resource.Resource {
					t.Fatal("resourceNewWithAttributes should not be called")
					return nil
				}
				resourceMerge = func(a, b *resource.Resource) (*resource.Resource, error) {
					t.Fatal("resourceMerge should not be called")
					return nil, nil
				}
			},
			wantErr: true,
		},
		{
			name: "resource merge error",
			setupViper: func() {
				viper.Set("app.name", "test-app")
				viper.Set("app.version", "1.0.0")
			},
			setupMocks: func(t *testing.T, wantProvider *sdklog.LoggerProvider) {
				t.Helper()

				new = func(ctx context.Context, opts ...otlploghttp.Option) (*otlploghttp.Exporter, error) {
					return &otlploghttp.Exporter{}, nil
				}
				resourceDefault = func() *resource.Resource {
					return resource.Empty()
				}
				newSchemaless = func(attrs ...attribute.KeyValue) *resource.Resource {
					return resource.Empty()
				}
				resourceMerge = func(a, b *resource.Resource) (*resource.Resource, error) {
					return nil, errors.New("merge error")
				}
				newLoggerProvider = func(opts ...sdklog.LoggerProviderOption) *sdklog.LoggerProvider {
					t.Fatal("newLoggerProvider should not be called")
					return nil
				}
			},
			wantErr: true,
		},
		{
			name: "success",
			setupViper: func() {
				viper.Set("app.name", "test-app")
				viper.Set("app.version", "1.0.0")
			},
			setupMocks: func(t *testing.T, wantProvider *sdklog.LoggerProvider) {
				t.Helper()

				called := false

				new = func(ctx context.Context, opts ...otlploghttp.Option) (*otlploghttp.Exporter, error) {
					return &otlploghttp.Exporter{}, nil
				}
				resourceDefault = func() *resource.Resource {
					return resource.Empty()
				}
				newSchemaless = func(attrs ...attribute.KeyValue) *resource.Resource {
					return resource.Empty()
				}
				resourceMerge = func(a, b *resource.Resource) (*resource.Resource, error) {
					return resource.Empty(), nil
				}
				newLoggerProvider = func(opts ...sdklog.LoggerProviderOption) *sdklog.LoggerProvider {
					called = true
					if len(opts) != 2 {
						t.Fatalf("newLoggerProvider() options = %d, want 2", len(opts))
					}
					return wantProvider
				}

				t.Cleanup(func() {
					if !called {
						t.Fatal("expected newLoggerProvider to be called")
					}
				})
			},
			wantErr:          false,
			wantSameProvider: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viperdata.ResetViperDataSingleton()

			tt.setupViper()

			wantProvider := sdklog.NewLoggerProvider()
			tt.setupMocks(t, wantProvider)

			got, err := newCofigLoggerProvider(context.Background())
			if (err != nil) != tt.wantErr {
				t.Fatalf("newCofigLoggerProvider() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				if got != nil {
					t.Fatalf("newCofigLoggerProvider() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("newCofigLoggerProvider() returned nil")
			}

			if tt.wantSameProvider && got != wantProvider {
				t.Fatalf("newCofigLoggerProvider() provider = %p, want %p", got, wantProvider)
			}
		})
	}
}

func TestInitLogger(t *testing.T) {
	tmpDir := t.TempDir()

	origProviderFactory := _newCofigLoggerProvider
	origFilepathAbs := filepathAbs
	defer func() {
		_newCofigLoggerProvider = origProviderFactory
		filepathAbs = origFilepathAbs
	}()

	tests := []struct {
		name             string
		ctx              context.Context
		dir              string
		setupViper       func()
		setupProvider    func(t *testing.T)
		setupFilepathAbs func(t *testing.T)
		wantErr          bool
		validate         func(t *testing.T, lp *sdklog.LoggerProvider, err error)
	}{
		{
			name: "success with log rotation enabled",
			ctx:  context.Background(),
			dir:  tmpDir,
			setupViper: func() {
				viper.Set(string(viperdata.AppAtribute), "my-app")
				viper.Set(string(viperdata.LoggerRotateEnableAtribute), true)
				viper.Set(string(viperdata.LoggerRotateMaxSizeAtribute), 10)
				viper.Set(string(viperdata.LoggerRotateMaxAgeAtribute), 7)
				viper.Set(string(viperdata.LoggerRotateMaxBackupsAtribute), 3)
				viper.Set(string(viperdata.LoggerCompressMaxAgeAtribute), true)
			},
			setupProvider: func(t *testing.T) {
				t.Helper()
				_newCofigLoggerProvider = func(ctx context.Context) (*sdklog.LoggerProvider, error) {
					return sdklog.NewLoggerProvider(), nil
				}
			},
			setupFilepathAbs: func(t *testing.T) {
				t.Helper()
				filepathAbs = filepath.Abs
			},
			wantErr: false,
			validate: func(t *testing.T, lp *sdklog.LoggerProvider, err error) {
				t.Helper()

				if err != nil {
					t.Fatalf("InitLogger() unexpected error = %v", err)
				}
				if lp == nil {
					t.Fatal("InitLogger() returned nil logger provider")
				}

				if err := lp.Shutdown(context.Background()); err != nil {
					t.Fatalf("LoggerProvider.Shutdown() error = %v", err)
				}
			},
		},
		{
			name: "provider error with log rotation enabled",
			ctx:  context.Background(),
			dir:  tmpDir,
			setupViper: func() {
				viper.Set(string(viperdata.AppAtribute), "my-app")
				viper.Set(string(viperdata.LoggerRotateEnableAtribute), true)
				viper.Set(string(viperdata.LoggerRotateMaxSizeAtribute), 10)
				viper.Set(string(viperdata.LoggerRotateMaxAgeAtribute), 7)
				viper.Set(string(viperdata.LoggerRotateMaxBackupsAtribute), 3)
				viper.Set(string(viperdata.LoggerCompressMaxAgeAtribute), true)
			},
			setupProvider: func(t *testing.T) {
				t.Helper()
				_newCofigLoggerProvider = func(ctx context.Context) (*sdklog.LoggerProvider, error) {
					return nil, errors.New("provider error")
				}
			},
			setupFilepathAbs: func(t *testing.T) {
				t.Helper()
				filepathAbs = filepath.Abs
			},
			wantErr: true,
			validate: func(t *testing.T, lp *sdklog.LoggerProvider, err error) {
				t.Helper()
				if err == nil {
					t.Fatal("InitLogger() expected error, got nil")
				}
				if lp != nil {
					t.Fatal("InitLogger() returned non-nil logger provider on error")
				}
			},
		},
		{
			name: "filepathAbs returns error",
			ctx:  context.Background(),
			dir:  tmpDir,
			setupViper: func() {
				viper.Set(string(viperdata.AppAtribute), "my-app")
				viper.Set(string(viperdata.LoggerRotateEnableAtribute), true)
				viper.Set(string(viperdata.LoggerRotateMaxSizeAtribute), 10)
				viper.Set(string(viperdata.LoggerRotateMaxAgeAtribute), 7)
				viper.Set(string(viperdata.LoggerRotateMaxBackupsAtribute), 3)
				viper.Set(string(viperdata.LoggerCompressMaxAgeAtribute), true)
			},
			setupProvider: func(t *testing.T) {
				t.Helper()
				_newCofigLoggerProvider = func(ctx context.Context) (*sdklog.LoggerProvider, error) {
					t.Fatal("_newCofigLoggerProvider should not be called when filepathAbs fails")
					return nil, nil
				}
			},
			setupFilepathAbs: func(t *testing.T) {
				t.Helper()
				filepathAbs = func(path string) (string, error) {
					return "", errors.New("filepathAbs error")
				}
			},
			wantErr: true,
			validate: func(t *testing.T, lp *sdklog.LoggerProvider, err error) {
				t.Helper()
				if err == nil {
					t.Fatal("InitLogger() expected error, got nil")
				}
				if err.Error() != "filepathAbs error" {
					t.Fatalf("InitLogger() error = %v, want %q", err, "filepathAbs error")
				}
				if lp != nil {
					t.Fatal("InitLogger() returned non-nil logger provider on filepathAbs error")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viperdata.ResetViperDataSingleton()

			_newCofigLoggerProvider = origProviderFactory
			filepathAbs = origFilepathAbs

			tt.setupViper()
			tt.setupProvider(t)
			tt.setupFilepathAbs(t)

			lp, err := InitLogger(tt.ctx, tt.dir)
			if (err != nil) != tt.wantErr {
				t.Fatalf("InitLogger() error = %v, wantErr %v", err, tt.wantErr)
			}

			tt.validate(t, lp, err)
		})
	}
}

func TestEnableModeTest(t *testing.T) {
	viper.Reset()
	DisableModeTest()
	EnableModeTest()

	got := viper.GetBool(string(viperdata.LoggerModeTestAtribute))
	if !got {
		t.Fatalf("EnableModeTest() = %v, want true", got)
	}
}

func TestDisableModeTest(t *testing.T) {
	viper.Reset()
	EnableModeTest()
	DisableModeTest()

	got := viper.GetBool(string(viperdata.LoggerModeTestAtribute))
	if got {
		t.Fatalf("DisableModeTest() = %v, want false", got)
	}
}

func Test_setLevel(t *testing.T) {
	tests := []struct {
		name  string
		level string
		want  slog.Level
	}{
		{
			name:  "debug level",
			level: "debug",
			want:  slog.LevelDebug,
		},
		{
			name:  "info level",
			level: "info",
			want:  slog.LevelInfo,
		},
		{
			name:  "warn level",
			level: "warn",
			want:  slog.LevelWarn,
		},
		{
			name:  "error level",
			level: "error",
			want:  slog.LevelError,
		},
		{
			name:  "default level",
			level: "something-else",
			want:  slog.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			viper.Reset()
			viperdata.ResetViperDataSingleton()

			viper.Set(string(viperdata.LoggerLevelAtribute), tt.level)

			got := setLevel()

			if got != tt.want {
				t.Errorf("setLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}
