// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	viperdata "github.com/PointerByte/QuicksGo/logger/viperData"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"gopkg.in/natefinch/lumberjack.v2"
)

var new = otlploghttp.New
var newLoggerProvider = sdklog.NewLoggerProvider
var resourceDefault = resource.Default
var newSchemaless = resource.NewSchemaless
var resourceMerge = resource.Merge

func newCofigLoggerProvider(ctx context.Context) (*sdklog.LoggerProvider, error) {
	exporter, err := new(ctx)
	if err != nil {
		return nil, err
	}
	res, err := resourceMerge(
		resourceDefault(),
		newSchemaless(
			semconv.ServiceName(viperdata.GetViperData(string(viperdata.AppAtribute)).(string)),
			semconv.ServiceVersion(viperdata.GetViperData(string(viperdata.AppVersionAtribute)).(string)),
		),
	)
	if err != nil {
		return nil, err
	}

	provider := newLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)
	return provider, nil
}

var _newCofigLoggerProvider = newCofigLoggerProvider
var filepathAbs = filepath.Abs

// InitLogger initializes and configures the application's logger.
// It builds the log file path, configures the OpenTelemetry logger provider,
// and returns the logger provider so it can be shut down gracefully when needed.
//
// When running the application as a server, logging is already initialized
// automatically, so calling this function manually is not necessary.
// However, in non-server contexts, you can call InitLogger to set up logging.
func InitLogger(ctx context.Context, dir string) (*sdklog.LoggerProvider, error) {
	// ---- File path configuration ----
	filePath := viperdata.GetViperData(string(viperdata.AppAtribute)).(string) + ".log"
	fileStr := filepath.Join(dir, filePath)

	// Save path complete
	bsFile, err := filepathAbs(fileStr)
	if err != nil {
		return nil, err
	}
	dir = filepath.Dir(bsFile)
	fullPath := filepath.Join(dir, filePath)

	// ---- Telemetry ----
	lp, err := _newCofigLoggerProvider(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger provider: %v", err)
	}
	handlerOtel := otelslog.NewHandler(
		"github.com/PointerByte/QuicksGo/logger",
		otelslog.WithLoggerProvider(lp),
		otelslog.WithSource(true),
	)

	mw := io.MultiWriter(os.Stdout)
	if viperdata.GetViperData(string(viperdata.LoggerRotateEnableAtribute)).(bool) {
		// ---- Lumberjack Logger ----
		logFile := &lumberjack.Logger{
			Filename:   fullPath,
			MaxSize:    viperdata.GetViperData(string(viperdata.LoggerRotateMaxSizeAtribute)).(int),
			MaxAge:     viperdata.GetViperData(string(viperdata.LoggerRotateMaxAgeAtribute)).(int),
			MaxBackups: viperdata.GetViperData(string(viperdata.LoggerRotateMaxBackupsAtribute)).(int),
			Compress:   viperdata.GetViperData(string(viperdata.LoggerCompressMaxAgeAtribute)).(bool),
		}
		// --- MultiWriter: file + console ---
		mw = io.MultiWriter(os.Stdout, logFile)
	} else {
		mw = os.Stdout
	}

	// ---- New handler slog ----
	newJsonHandler := newHandler(setLevel(), mw, handlerOtel)
	slog.SetDefault(slog.New(newJsonHandler))

	return lp, nil
}

func setLevel() slog.Level {
	switch viperdata.GetViperData(string(viperdata.LoggerLevelAtribute)).(string) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func EnableModeTest() {
	viper.SetDefault(string(viperdata.LoggerModeTestAtribute), true)
}

func DisableModeTest() {
	viper.SetDefault(string(viperdata.LoggerModeTestAtribute), false)
}
