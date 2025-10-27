package logger

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"quicksgo/telemetry"
	"time"

	"github.com/spf13/viper"
	otellog "go.opentelemetry.io/otel/log"
)

// ClearFile empties the content of the file at the given path.
// It does NOT delete the file — just truncates it to 0 bytes.
func ClearFile() error {
	path := viper.GetString("logger.path")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	return nil
}

var emitOtel = func(ctx context.Context, _level level, result LogEntry) {
	_emitOtel(ctx, _level, result)
}

func _emitOtel(ctx context.Context, _level level, result LogEntry) {
	lp := telemetry.GetLoggerProvider()
	if lp == nil {
		return
	}
	otelLogger := lp.Logger(viper.GetString("service.name"))

	var rec otellog.Record
	now := time.Now()
	rec.SetTimestamp(now)
	rec.SetObservedTimestamp(now)

	switch _level {
	case INFO:
		rec.SetSeverity(otellog.SeverityInfo)
		rec.SetSeverityText("INFO")
	case WARNING:
		rec.SetSeverity(otellog.SeverityWarn)
		rec.SetSeverityText("WARN")
	case ERROR:
		rec.SetSeverity(otellog.SeverityError)
		rec.SetSeverityText("ERROR")
	case FATAL:
		rec.SetSeverity(otellog.SeverityFatal)
		rec.SetSeverityText("FATAL")
	default:
		rec.SetSeverity(otellog.SeverityUndefined)
		rec.SetSeverityText("UNDEFINED")
	}

	// Cuerpo del log
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		log.Fatal(err)
	}
	rec.SetBody(otellog.StringValue(string(jsonBytes)))

	// Correlación con la traza activa
	rec.AddAttributes(
		otellog.String("trace_id", result.TraceId),
		otellog.String("span_id", result.SpanId),
	)

	// Emite el log al pipeline de OTel
	otelLogger.Emit(ctx, rec)
}
