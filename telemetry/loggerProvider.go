package telemetry

import (
	"sync"

	sdklog "go.opentelemetry.io/otel/sdk/log"
)

var (
	globalLoggerProvider *sdklog.LoggerProvider
	once                 sync.Once
)

// SetLoggerProvider registers the global OpenTelemetry LoggerProvider instance.
//
// It sets the provided *sdklog.LoggerProvider only once using sync.Once to ensure
// thread-safe initialization and prevent multiple logger provider registrations
// across the application lifecycle.
func SetLoggerProvider(lp *sdklog.LoggerProvider) {
	once.Do(func() {
		globalLoggerProvider = lp
	})
}

// GetLoggerProvider returns the current global OpenTelemetry LoggerProvider.
//
// It provides access to the singleton logger provider instance previously
// registered via SetLoggerProvider, allowing other components to obtain
// a shared logging provider for consistent telemetry output.
func GetLoggerProvider() *sdklog.LoggerProvider {
	return globalLoggerProvider
}
