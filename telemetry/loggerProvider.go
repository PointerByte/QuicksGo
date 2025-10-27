package telemetry

import (
	"sync"

	sdklog "go.opentelemetry.io/otel/sdk/log"
)

var (
	globalLoggerProvider *sdklog.LoggerProvider
	once                 sync.Once
)

func SetLoggerProvider(lp *sdklog.LoggerProvider) {
	once.Do(func() {
		globalLoggerProvider = lp
	})
}

func GetLoggerProvider() *sdklog.LoggerProvider {
	return globalLoggerProvider
}
