# Configuration

The application is preferably configured through a YAML file.  
YAML should be considered the default format to follow for new configurations and future maintenance.

JSON remains supported as an optional format for compatibility or integration with existing flows.

This file defines application metadata, server settings, Gin configuration, and the logging system configuration.

## Default recommended format: YAML

```yaml
app:
  name: service-template
  version: 0.0.1

server:
  port: ":10443"
  groups:
    - /service-template/v1

gin:
  LoggerWithConfig:
    enabled: true
    SkipPaths:
      - /health
    SkipQueryString: false

logger:
  level: info
  ignoredHeaders:
    - Authorization
    - Cookie
  rotate:
    enable: true
    maxSize: 10
    maxBackups: 5
    maxAge: 30
    compress: true
  formatter: json
  formatDate: "2006-01-02T15:04:05.000"
```

## Optional format: JSON

If you need to use JSON, the same configuration can be expressed as follows:

```json
{
  "app": {
    "name": "service-template",
    "version": "0.0.1"
  },
  "server": {
    "port": ":10443",
    "groups": ["/service-template/v1"]
  },
  "gin": {
    "LoggerWithConfig": {
      "enabled": true,
      "SkipPaths": ["/health"],
      "SkipQueryString": false
    }
  },
  "logger": {
    "level": "info",
    "ignoredHeaders": ["Authorization", "Cookie"],
    "rotate": {
      "enable": true,
      "maxSize": 10,
      "maxBackups": 5,
      "maxAge": 30,
      "compress": true
    },
    "formatter": "json",
    "formatDate": "2006-01-02T15:04:05.000"
  }
}
```

## How to use this dependency

This package reads its configuration from `viper` and then initializes a custom `slog` logger with file output, console output, OTEL export, Gin request logging, and support for downstream trace entries.

### 1. Install the dependency

```bash
go get github.com/PointerByte/QuicksGo/logger
```

### 2. Load the configuration into Viper

Before calling the package, make sure your application has already loaded the configuration values expected by the logger.

Example using `application.yaml`:

```go
package main

import "github.com/spf13/viper"

func loadConfig() error {
	viper.SetConfigName("application")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	return viper.ReadInConfig()
}
```

### 3. Initialize the logger

`builder.InitLogger` configures the global logger and returns the OTEL logger provider so you can shut it down gracefully.

```go
package main

import (
	"context"
	"path/filepath"

	"github.com/PointerByte/QuicksGo/logger/builder"
)

func initLogger(ctx context.Context) error {
	lp, err := builder.InitLogger(ctx, filepath.Join(".", "logs"))
	if err != nil {
		return err
	}

	defer lp.Shutdown(ctx)
	return nil
}
```

### 4. Use it with Gin

For HTTP services, the recommended middleware order is:

```go
engine := gin.New()
engine.Use(
	gin.Recovery(),
	middlewares.InitLogger(),
	middlewares.LoggerWithConfig(),
	middlewares.CaptureBody(),
)
```

What each middleware does:

- `middlewares.InitLogger()` creates or propagates the request trace context.
- `middlewares.LoggerWithConfig()` writes the final HTTP log entry using the configured formatter.
- `middlewares.CaptureBody()` captures request and response bodies so they can be included in `details.request` and `details.response`.

Imports used by the previous example:

```go
import (
	"github.com/PointerByte/QuicksGo/logger/middlewares"
	"github.com/gin-gonic/gin"
)
```

### 5. Use it with gRPC

For gRPC servers, the package also provides unary and stream interceptors that mirror the HTTP middlewares:

```go
grpcServer := grpc.NewServer(
	grpc.ChainUnaryInterceptor(
		traces.MiddlewareOtelGRPCUnary(),
		middlewares.InitLoggerUnaryServerInterceptor(),
		middlewares.LoggerWithConfigUnaryServerInterceptor(),
		middlewares.CaptureBodyUnaryServerInterceptor(),
	),
	grpc.ChainStreamInterceptor(
		traces.MiddlewareOtelGRPCStream(),
		middlewares.InitLoggerStreamServerInterceptor(),
		middlewares.LoggerWithConfigStreamServerInterceptor(),
		middlewares.CaptureBodyStreamServerInterceptor(),
	),
)
```

Recommended order:

- OTEL first, so distributed tracing is extracted before the logger creates its request-scoped context.
- `InitLogger*` next, so the logger context is available to the remaining interceptors.
- `LoggerWithConfig*` before `CaptureBody*`, just like in HTTP, so the final log sees the captured payloads after the handler returns.

What each gRPC interceptor does:

- `middlewares.InitLoggerUnaryServerInterceptor()` and `middlewares.InitLoggerStreamServerInterceptor()` create the logger context, attach the gRPC method metadata, copy incoming headers, and open the logger span.
- `middlewares.CaptureBodyUnaryServerInterceptor()` stores the unary request and response in the logger context.
- `middlewares.CaptureBodyStreamServerInterceptor()` captures inbound and outbound stream messages and stores them as one value or as a slice when multiple messages are exchanged.
- `middlewares.LoggerWithConfigUnaryServerInterceptor()` and `middlewares.LoggerWithConfigStreamServerInterceptor()` write the final structured log entry and copy captured bodies into `details.request` and `details.response` when body logging is enabled.
- `traces.MiddlewareOtelGRPCUnary()` and `traces.MiddlewareOtelGRPCStream()` create the OpenTelemetry server span for each RPC.

Imports used by the previous example:

```go
import (
	"github.com/PointerByte/QuicksGo/logger/middlewares"
	"github.com/PointerByte/QuicksGo/config/utilities/traces"
	"google.golang.org/grpc"
)
```

### 6. Log inside handlers

For request-scoped logging inside Gin handlers, use the helper functions:

```go
func exampleHandler(c *gin.Context) {
	builder.PrintInfo(c, "request processed")
	c.JSON(200, gin.H{"ok": true})
}
```

Available helpers:

- `builder.PrintInfo`
- `builder.PrintDebug`
- `builder.PrintWarn`
- `builder.PrintError`

### 7. Use the contextual logger directly

If you need to log outside Gin, or you want to keep state in a custom context, create a logger context with `builder.New`.

```go
import (
	"context"
	"errors"
)

ctx := context.Background()
ctxLogger := builder.New(ctx)

ctxLogger.Info("application started")
ctxLogger.Warn("slow dependency detected")
ctxLogger.Error(errors.New("unexpected failure"))
```

### 8. Trace downstream calls or subprocesses

Use `TraceInit` and `TraceEnd` to register satellite services or internal sub-processes in the `services` section of the log.

```go
process := formatter.Service{
	System:  "auth-service",
	Process: "validate-token",
	Method:  "POST",
	Path:    "/auth/validate",
	Server:  "auth.internal",
}

ctxLogger.TraceInit(&process)
defer ctxLogger.TraceEnd(&process)

process.Code = 200
```

Typical use cases:

- downstream HTTP calls
- internal business sub-processes
- integrations that should appear under the same trace

### 9. Testing mode

If you need to silence log output during tests, enable test mode before exercising the logger:

```go
builder.EnableModeTest()
defer builder.DisableModeTest()
```

## End-to-end example

```go
package main

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/PointerByte/QuicksGo/logger/builder"
	"github.com/gin-gonic/gin"
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
	lp, err := builder.InitLogger(ctx, filepath.Join(".", "logs"))
	if err != nil {
		panic(err)
	}
	defer lp.Shutdown(ctx)

	engine := gin.New()
	engine.Use(
		gin.Recovery(),
		builder.MiddlewareInitLogger(),
		builder.MiddlewareLoggerWithConfig(),
		builder.MiddlewareCaptureBody(),
	)

	engine.GET("/health", func(c *gin.Context) {
		builder.PrintInfo(c, "health check")
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	_ = engine.Run(":8080")
}
```

## Usage note

- Use YAML as the primary format for new projects and reference examples.
- Use JSON only when there is a specific compatibility requirement.
- Structurally, both formats represent the same configuration.

---

# app

General service information.

```json
"app": {
  "name": "service-template",
  "version": "0.0.1"
}
```

| Field | Description |
|------|-------------|
| `name` | Service or application name |
| `version` | Current service version |

---

# server

HTTP server configuration.

```json
"server": {
  "port": ":10443",
  "groups": ["/api/v1"]
}
```

| Field | Description |
|------|-------------|
| `port` | Port where the server runs |
| `groups` | List of base prefixes for API routes |

`groups` lets you define multiple route groups to organize service endpoints.

---

# gin

Configuration related to Gin and its middlewares.

```json
"gin": {
  "LoggerWithConfig": {
    "enabled": true,
    "SkipPaths": ["/health"],
    "SkipQueryString": false
  }
}
```

This section allows you to control the behavior of Gin's HTTP logger when `LoggerWithConfig` is used.

## Purpose

The `gin` configuration is used to control how incoming HTTP requests are logged when Gin is acting as the web framework.

This makes it possible to, for example:

- enable or disable Gin HTTP logging
- omit routes that should not be logged
- decide whether the query string should be part of the logged path

---

# server.gin.LoggerWithConfig

Defines the configuration of Gin's `LoggerWithConfig` middleware.

```json
"gin": {
  "LoggerWithConfig": {
    "enabled": true,
    "SkipPaths": ["/health"],
    "SkipQueryString": false
  }
}
```

| Field | Description |
|------|-------------|
| `enabled` | Enables or disables Gin HTTP logging |
| `SkipPaths` | List of routes that must not be logged by Gin |
| `SkipQueryString` | Defines whether the query string should be excluded from the logged path |

## `enabled`

Allows enabling or disabling Gin's HTTP logging middleware.

```json
"enabled": true
```

### Behavior

- `true`: Gin HTTP logging is enabled
- `false`: Gin HTTP logging is not applied

### Recommendation

Keep it set to `true` when you want traceability for incoming HTTP requests handled by the service.

It can be disabled if HTTP logging is already covered by another middleware or if you do not want to log incoming traffic from Gin.

---

## `SkipPaths`

Controls which request paths Gin's HTTP logger must skip.

```json
"SkipPaths": ["/health"]
```

### Purpose

This is used to avoid log noise when there are endpoints that are called very frequently and normally do not add useful observability value.

Typical cases:

- health checks
- readiness probes
- liveness probes
- internal monitoring endpoints

### Example

```json
"SkipPaths": [
  "/health",
  "/ready",
  "/live"
]
```

With this configuration, requests to those paths should not appear in Gin HTTP logs.

### Recommendation

Add operational or infrastructure endpoints here if they can generate too much noise, especially when they are called periodically by load balancers, Kubernetes, or monitoring tools.

---

## `SkipQueryString`

Controls whether Gin's HTTP logger includes the query string in the logged path.

```json
"SkipQueryString": false
```

### Behavior

- `false`: the logged path keeps the query string when present
- `true`: the logged path omits the query string and keeps only the base route

### Example

If a request arrives like this:

```text
GET /customers?id=123&type=premium
```

Then:

- with `SkipQueryString: false`, the log may keep the `?id=123&type=premium` part
- with `SkipQueryString: true`, the log records only the base path `/customers`

### When to enable it

Using `true` is recommended when:

- the query string may contain sensitive data
- you want to avoid high-cardinality logs
- you want a cleaner and more stable path for searching and aggregation

It can remain `false` when query parameters are useful for troubleshooting and do not introduce sensitive data or excessive log cardinality.

---

## Recommended example

```json
"gin": {
  "LoggerWithConfig": {
    "enabled": true,
    "SkipPaths": ["/health", "/ready", "/live"],
    "SkipQueryString": true
  }
}
```

This configuration is usually appropriate when the goal is to:

- reduce log noise
- avoid logging repetitive operational paths
- keep routes clean without variable parameters

---

# logger

Logging system configuration.

```json
"logger": {
  "level": "info",
  "ignoredHeaders": ["Authorization", "Cookie"],
  "rotate": {
    "enable": true,
    "maxSize": 10,
    "maxBackups": 5,
    "maxAge": 30,
    "compress": true
  },
  "formatter": "json",
  "formatDate": "2006-01-02T15:04:05.000"
}
```

| Field | Description |
|------|-------------|
| `level` | Minimum log level that will be recorded |
| `ignoredHeaders` | List of HTTP headers that must not be included in the log |
| `rotate` | Log rotation configuration |
| `formatter` | Log output format |
| `formatDate` | Date format used in logs |

---

# logger.level

Defines the minimum log level that will be recorded.

```json
"level": "info"
```

## Available levels

| Level | Description |
|------|-------------|
| `debug` | Detailed information for debugging |
| `info` | General system information |
| `error` | Errors that affect operation |

---

# logger.ignoredHeaders

Defines which HTTP headers must be excluded from the structured log, specifically from the `details.headers` field.

## Purpose

`ignoredHeaders` is used to prevent sensitive or unnecessary information from being recorded in logs, for example:

- authorization tokens
- session cookies
- API keys
- credentials or other private headers

## Example

```json
"logger": {
  "ignoredHeaders": ["Authorization", "Cookie"]
}
```

If the request includes headers such as:

```http
Authorization: Bearer abc123
Cookie: session=xyz789
Content-Type: application/json
X-Trace-Id: 12345
```

the expected result in the log is to keep only the non-ignored headers, for example:

```json
"details": {
  "headers": {
    "Content-Type": ["application/json"],
    "X-Trace-Id": ["12345"]
  }
}
```

## Recommendation

Add here any header that may contain sensitive information, for example:

```json
"ignoredHeaders": [
  "Authorization",
  "Cookie",
  "Set-Cookie",
  "X-Api-Key"
]
```

---

# logger.rotate

Automatic log file rotation configuration.

```json
"rotate": {
  "enable": true,
  "maxSize": 10,
  "maxBackups": 5,
  "maxAge": 30,
  "compress": true
}
```

| Field | Description |
|------|-------------|
| `enable` | Enables log rotation |
| `maxSize` | Maximum file size in MB before rotation |
| `maxBackups` | Maximum number of old files to keep |
| `maxAge` | Maximum number of days to keep old logs |
| `compress` | Compress old files into `.gz` |

---

# logger.formatter

Defines how the log is written at the end.

The formatter supports three approaches:

1. structured JSON output
2. readable text output
3. custom output through templates

## Supported values

| Value | Behavior |
|---|---|
| `json` | Generates the log as JSON using the default internal template |
| `text` | Generates readable text output |
| `txt` | Alias of `text` |
| `""` | If empty, uses `text` format |
| any other value | Interpreted as a complete custom template |

This means there is no special value called `template`.  
Any string different from `json`, `text`, `txt`, or empty will be treated as a full output template.

---

## `json` format

If configured as:

```json
"formatter": "json"
```

the output will be a single-line JSON log.

### `json` mode characteristics

- uses a default internal template
- prints the JSON on a single line
- omits null fields
- omits empty fields
- omits optional fields with no value
- keeps a consistent structure for observability

### Example

```json
{"timestamp":"2026-03-13T01:10:23.123","traceID":"main-trace-001","level":"INFO","message":"Loan simulation completed","details":{"system":"loan-service","client":"mobile-app","protocol":"HTTP","method":"POST","path":"/loan/simulate","headers":{"Content-Type":["application/json"]},"request":{"amount":10000,"term":12},"response":{"approved":true}},"services":[{"traceID":"sat-001","system":"auth-service","process":"validate-token","server":"auth.internal","protocol":"HTTP","method":"POST","path":"/auth/validate","code":200,"status":"SUCCESS","latency":12},{"traceID":"sat-002","system":"score-engine","process":"calculate-score","server":"score.internal","protocol":"HTTP","method":"POST","path":"/score/calculate","code":200,"status":"SUCCESS","latency":28}],"method":"SimulateLoan","line":87,"totalTime":64}
```

---

## `text` format

If configured as:

```json
"formatter": "text"
```

the output will be a readable single-line text log.

### Example

```text
[2026-03-13T01:10:23.123] [INFO] [8f3a5d9c-9f2a-4e1d-b3a7-7f23d9a1e4aa] ProcessPayment:142 - Request processed successfully totalTime=155ms
```

---

## Template-based customization

In addition to `json` and `text`, the logger allows full output customization using templates.

If `formatter` contains any value different from:

- `json`
- `text`
- `txt`
- empty

then that value is interpreted as the complete template that will define the final output.

## Supported customization level

This level of customization allows changing not only the log content, but also its full structure.

With a custom template you can:

- change field names
- omit entire sections
- group data differently
- combine multiple fields into one
- emit single-line JSON
- emit free-form text
- mix text with JSON fragments
- reduce final log size
- adapt the output to legacy integrations or specific platforms

---

## Important: the template fully replaces the output

When you use a custom template, that template completely defines the final output.

This means the following form is not used:

```json
"formatter": "json { ... }"
```

That syntax is not valid.

## Correct usage

You must use one of these two options:

### Option 1: use the default JSON

```json
"formatter": "json"
```

### Option 2: use a full custom template

```json
"formatter": "{\"timestamp\":{{json .Timestamp}},\"traceID\":{{json .TraceID}},\"message\":{{json .Message}}}"
```

In this second option, the full `formatter` value is already the template.

---

## Advanced JSON customization

If you want to change the JSON structure, you must use a full JSON template inside `formatter`.

This allows you to transform the output structure without depending on the default one.

### Example: combine `method` and `path` into `details.service`

If you want to print this:

```json
{
  "timestamp": "2026-03-13T01:10:23.123",
  "traceID": "main-trace-001",
  "level": "INFO",
  "message": "Loan simulation completed",
  "details": {
    "system": "loan-service",
    "client": "mobile-app",
    "protocol": "HTTP",
    "service": "POST /loan/simulate",
    "headers": {
      "Content-Type": ["application/json"]
    },
    "request": {
      "amount": 10000,
      "term": 12
    },
    "response": {
      "approved": true
    }
  },
  "services": [
    {
      "traceID": "sat-001",
      "system": "auth-service",
      "process": "validate-token",
      "protocol": "HTTP",
      "method": "POST",
      "server": "auth.internal",
      "code": 200,
      "path": "/auth/validate",
      "status": "SUCCESS",
      "latency": 12
    }
  ],
  "method": "SimulateLoan",
  "line": 87,
  "totalTime": 64
}
```

then `formatter` must be a full template like this:

```json
"formatter": "{\"timestamp\":{{json .Timestamp}},\"traceID\":{{json .TraceID}},\"level\":{{json .Level}},\"message\":{{json .Message}},\"details\":{\"system\":{{json .Details.System}},\"client\":{{json .Details.Client}},\"protocol\":{{json .Details.Protocol}},\"service\":{{json (printf \"%s %s\" .Details.Method .Details.Path | trim)}},\"headers\":{{json .Details.Headers}},\"request\":{{json .Details.Request}},\"response\":{{json .Details.Response}}},\"services\":{{json (buildServices .Services)}},\"method\":{{json .Method}},\"line\":{{json .Line}},\"totalTime\":{{json .Time}}}"
```

### Expected result

With that template, `details` no longer prints:

```json
"method": "POST",
"path": "/loan/simulate"
```

and instead prints:

```json
"service": "POST /loan/simulate"
```

---

## What you can customize

With templates you can decide exactly what to show in the log, for example:

- only the message
- message plus method and line
- request and response
- main trace
- downstream calls or satellites from `services`
- a completely different JSON from the default template

## Available data for customization

You can build custom output from:

- `Timestamp`
- `TraceID`
- `Level`
- `Message`
- `Method`
- `Line`
- `Time`
- `Details`
- `Services`

Inside `Details`, the most useful fields are usually:

- `System`
- `Client`
- `Protocol`
- `Method`
- `Path`
- `Headers`
- `Request`
- `Response`

Inside `Services`, the most useful fields are usually:

- `IdTrace`
- `System`
- `Process`
- `Server`
- `Protocol`
- `Method`
- `Path`
- `Code`
- `Request`
- `Response`
- `Status`
- `Latency`

> Note: in JSON output for a downstream service, the field appears as `traceID`, but in template customization it is referenced as `IdTrace`.

---

## Customization examples

### 1) Simple message

```json
"formatter": "{{.Message}}"
```

Output:

```text
Request processed successfully
```

### 2) Message with method and line

```json
"formatter": "{{.Message}} | {{.Method}}:{{.Line}}"
```

Output:

```text
Request processed successfully | ProcessPayment:142
```

### 3) Basic access log

```json
"formatter": "[{{.Timestamp}}] {{.Level}} {{.Details.Method}} {{.Details.Path}} trace={{.TraceID}} msg={{.Message}}"
```

Output:

```text
[2026-03-13T01:10:23.123] INFO POST /payments trace=8f3a5d9c-9f2a-4e1d-b3a7-7f23d9a1e4aa msg=Request processed successfully
```

### 4) Request and response in compact output

```json
"formatter": "{{.Message}} | req={{json .Details.Request}} | resp={{json .Details.Response}}"
```

Output:

```text
Request processed successfully | req={"amount":100} | resp={"status":"ok"}
```

### 5) Full log serialized from a template

```json
"formatter": "{{json .}}"
```

Approximate output:

```json
{"timestamp":"2026-03-13T01:10:23.123","traceID":"8f3a5d9c-9f2a-4e1d-b3a7-7f23d9a1e4aa","level":"INFO","message":"Request processed successfully","details":{},"services":[],"method":"ProcessPayment","line":142,"totalTime":155}
```

### 6) Print downstream calls or satellites

```json
"formatter": "{{.Message}}{{range .Services}} | svc={{.System}} process={{.Process}} method={{.Method}} path={{.Path}} code={{.Code}} status={{.Status}} latency={{.Latency}}ms{{end}}"
```

Approximate output:

```text
Request processed successfully | svc=auth-service process=validate-token method=POST path=/auth/validate code=200 status=SUCCESS latency=18ms | svc=customer-core process=get-profile method=GET path=/customers/profile code=200 status=SUCCESS latency=32ms
```

### 7) Print satellites with individual trace

```json
"formatter": "{{range .Services}}[trace={{.IdTrace}}] {{.System}} {{.Process}} status={{.Status}} latency={{.Latency}}ms {{end}}"
```

Approximate output:

```text
[trace=sat-001] auth-service validate-token status=SUCCESS latency=12ms [trace=sat-002] score-engine calculate-score status=SUCCESS latency=28ms
```

### 8) Compact custom JSON

```json
"formatter": "{\"timestamp\":{{json .Timestamp}},\"traceID\":{{json .TraceID}},\"message\":{{json .Message}},\"details\":{\"system\":{{json .Details.System}},\"service\":{{json (printf \"%s %s\" .Details.Method .Details.Path | trim)}}}}"
```

Approximate output:

```json
{"timestamp":"2026-03-13T01:10:23.123","traceID":"main-trace-001","message":"Loan simulation completed","details":{"system":"loan-service","service":"POST /loan/simulate"}}
```

---

## Usage recommendations

### Use `json` when:

- you want structured observability
- you need field-based searches
- the log destination is Kibana, Loki, CloudWatch, or Elastic
- you want full `details` and `services`
- you want compact single-line output
- you want null and empty values omitted automatically

### Use `text` when:

- you want easy-to-read console output
- you are working in local development
- you need fast and compact output

### Use customization when:

- you want to adapt the log to a legacy format
- you only need to print certain fields
- you want to mix free text with JSON blocks
- you want to include downstream calls or satellites from `Services` in a single line
- you want to change the default JSON structure

---

## Important considerations

- `json` is the recommended option for structured observability
- `text` is the most convenient option for local debugging
- customization is the best option when you need hybrid or more compact output
- if you need field-based querying, prefer `json`
- if you need immediate readability in the console, prefer `text`
- if you need to change the JSON structure, you must use a full JSON template different from the default one

---

# logger.formatDate

Defines the date format used in logs.

```json
"formatDate": "2006-01-02T15:04:05.000"
```

---

# Summary

| Section | Purpose |
|------|------|
| `app` | Service information |
| `server` | HTTP server configuration |
| `gin` | Gin middleware and HTTP logging configuration |
| `server.gin.LoggerWithConfig` | Gin `LoggerWithConfig` middleware configuration |
| `logger` | Logging system configuration |
| `logger.level` | Minimum log level |
| `logger.ignoredHeaders` | Headers that must not be logged |
| `logger.rotate` | Automatic log rotation |
| `logger.formatter` | Log output format and customization in JSON, text, and templates |
| `logger.formatDate` | Date format |

---

# Useful commands

### Update dependencies

```bash
go get -u=patch ./...
```

### Clear build, unit test, and gomod cache

```bash
go clean -cache -testcache -modcache
```

### Run unit tests

```bash
go test -cover -covermode=atomic -coverprofile="coverage.out" ./...
```

### Generate unit test HTML coverage report

```bash
go tool cover -html="coverage.out" -o "coverage.html"
```

### Show unit test coverage from `.out`

```bash
go tool cover -func="coverage.out"
```
