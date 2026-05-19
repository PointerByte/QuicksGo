# GoForge Logger

`logger` provides GoForge's structured logging layer. It configures a global
`slog` logger, formats entries as JSON, text, or a custom template, exports logs
through OpenTelemetry, and includes Gin and gRPC middleware for request-scoped
logs.

## Installation

```bash
go get github.com/PointerByte/GoForge/logger
```

## Packages

- `builder`: logger initialization, context-aware logging, and trace sections
- `common`: shared context keys used by HTTP and gRPC middleware
- `middlewares/http`: Gin middleware
- `middlewares/grpc`: gRPC interceptors
- `formatter`: structured log models and formatter implementations
- `viperData`: viper-backed configuration cache used by the logger
- `utilities`: small caller-tracing helpers

## Configuration

The module reads configuration from `viper`. It does not load files by itself,
so your application should load `application.yaml`, `application.yml`, JSON, or
environment values before calling `builder.InitLogger(...)` or installing the
middlewares.

```yaml
app:
  name: service-template
  version: 0.0.1

server:
  gin:
    LoggerWithConfig:
      enabled: true
      SkipPaths:
        - /health
      SkipQueryString: false
  grpc:
    LoggerWithConfig:
      enabled: true
      SkipFunction: []

logger:
  dir: logs
  modeTest: false
  level: info
  ignoredHeaders:
    - Authorization
    - Cookie
  formatter: json
  formatDate: "2006-01-02T15:04:05.000"
  rotate:
    enable: true
    maxSize: 10
    maxBackups: 5
    maxAge: 30
    compress: true
```

Main keys:

- `app.name`: service name included in log details and OTEL resource metadata
- `app.version`: service version included in OTEL resource metadata
- `server.gin.LoggerWithConfig.enabled`: enables final Gin request logs
- `server.gin.LoggerWithConfig.SkipPaths`: request paths skipped by Gin logging
- `server.gin.LoggerWithConfig.SkipQueryString`: omits query strings from logged paths
- `server.grpc.LoggerWithConfig.enabled`: enables final gRPC request logs
- `server.grpc.LoggerWithConfig.SkipFunction`: gRPC methods skipped by name (`SayHello`) or full method (`/pkg.Service/SayHello`)
- `logger.dir`: directory where the log file is created by callers that use this key
- `logger.modeTest`: suppresses logger output and trace collection in test mode
- `logger.level`: `debug`, `info`, `warn`, or `error`
- `logger.ignoredHeaders`: headers filtered from structured request details
- `logger.formatter`: `json`, `text`, or a custom Go template
- `logger.formatDate`: timestamp layout
- `logger.rotate.*`: file rotation settings backed by `lumberjack`

`viperData` caches values on first use. In tests that change viper values
inside the same process, call `viperdata.ResetViperDataSingleton()` before
re-reading logger configuration.

## Initialize The Logger

```go
package main

import (
	"context"
	"path/filepath"

	"github.com/PointerByte/GoForge/logger/builder"
	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigName("application")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}

	ctx := context.Background()
	lp, err := builder.InitLogger(ctx, filepath.Join(".", viper.GetString("logger.dir")))
	if err != nil {
		panic(err)
	}
	defer lp.Shutdown(ctx)

	builder.New(ctx).Info("logger initialized")
}
```

`builder.InitLogger` configures the process-wide `slog` default logger. It
writes to stdout and, when `logger.rotate.enable=true`, to a rotated log file.
It also creates an OpenTelemetry logger provider and returns it so the caller
can shut it down gracefully.

## Gin Middleware

```go
package main

import (
	"net/http"

	httpmiddlewares "github.com/PointerByte/GoForge/logger/middlewares/http"
	"github.com/gin-gonic/gin"
)

func main() {
	engine := gin.New()
	engine.Use(
		gin.Recovery(),
		httpmiddlewares.InitLogger(),
		httpmiddlewares.LoggerWithConfig(),
		httpmiddlewares.CaptureBody(),
	)

	engine.GET("/health", func(c *gin.Context) {
		httpmiddlewares.EnableBody(c, true, true)
		httpmiddlewares.PrintInfo(c, "health check")
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
```

Middleware roles:

- `InitLogger()` extracts distributed-tracing headers, creates a request-scoped logger context, and stores request metadata.
- `LoggerWithConfig()` emits the final structured HTTP log entry through Gin's logger hook.
- `CaptureBody()` captures request and response bodies only when body logging is enabled, so they can be included in `details.request` and `details.response` without storing disabled payloads.
- `EnableBody(c, request, response)` opts the final HTTP request log into request and response body emission. Bodies are disabled by default.
- `EnableTraceBody(c, request, response)` opts trace service entries into request and response body emission when `TraceEnd` is called. Trace bodies are disabled by default.

The helper functions `PrintInfo`, `PrintDebug`, `PrintWarn`, and `PrintError`
schedule a request-scoped log message from inside Gin handlers.

## gRPC Interceptors

```go
import loggrpc "github.com/PointerByte/GoForge/logger/middlewares/grpc"

grpcServer := grpc.NewServer(
	grpc.ChainUnaryInterceptor(
		loggrpc.InitLoggerUnaryServerInterceptor(),
		loggrpc.LoggerWithConfigUnaryServerInterceptor(),
		loggrpc.CaptureBodyUnaryServerInterceptor(),
	),
	grpc.ChainStreamInterceptor(
		loggrpc.InitLoggerStreamServerInterceptor(),
		loggrpc.LoggerWithConfigStreamServerInterceptor(),
		loggrpc.CaptureBodyStreamServerInterceptor(),
	),
)
```

The interceptors mirror the Gin middleware behavior for unary and streaming
RPCs: they build the request-scoped logger context, capture request/response
payloads, copy metadata into structured details, and write the final log when
the handler completes.

Request and response bodies are disabled by default. Use
`loggrpc.EnableBody(ctxLogger, true, true)` to include them in the final gRPC
request log, and `loggrpc.EnableTraceBody(ctxLogger, true, true)` to include
trace service bodies.

Final gRPC request logging intentionally ignores `codes.Unauthenticated` and
`codes.PermissionDenied` errors, so JWT authorization failures do not emit
logger middleware logs.

Use `loggrpc.PrintInfo`, `PrintDebug`, `PrintWarn`, or `PrintError` with the
request logger context when a handler needs to choose the final log level and
message explicitly.

When you use the root `config/server/grpc` package, these interceptors are
installed for you.

## Context Logger

Use `builder.New(ctx)` outside Gin or gRPC handlers when you need a contextual
logger directly:

```go
ctxLogger := builder.New(context.Background())

ctxLogger.Info("application started")
ctxLogger.Debug("cache warmed")
ctxLogger.Warn("dependency latency is high")
ctxLogger.Error(errors.New("dependency failed"))
```

## Trace Sections

`TraceInit` and `TraceEnd` add downstream calls or internal subprocesses to the
`services` array in the structured log.

```go
process := &formatter.Service{
	System:  "users-service",
	Process: "list-users",
	Method:  "GET",
	Server:  "https://users.internal",
	Path:    "/users",
}

ctxLogger.TraceInit(process)
defer ctxLogger.TraceEnd(process)

process.Code = 200
```

Common use cases are outbound HTTP/gRPC calls, provider SDK calls, and internal
business steps that should appear under the same trace.

Trace service request and response bodies are disabled by default. In Gin
handlers use `httpmiddlewares.EnableTraceBody(c, true, true)`; in gRPC handlers
use `loggrpc.EnableTraceBody(ctxLogger, true, true)`.

## Formatters

`logger.formatter` supports:

- `json`: structured JSON output
- `text` or `txt`: human-readable text output
- any custom Go template accepted by `formatter.CustomFormatter`

Custom templates can use helper functions such as `json`, `buildDetails`, and
`buildServices`.

## Testing

To silence log output and trace collection in tests:

```go
builder.EnableModeTest()
defer builder.DisableModeTest()
```

From the `logger` module directory:

```bash
go test ./...
go test -cover -covermode=atomic -coverprofile=coverage.out ./...
```
