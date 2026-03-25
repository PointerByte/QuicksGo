# QuicksGo Config

Configuration and infrastructure library for Go services built around:

- Gin HTTP servers
- gRPC servers
- HTTP and gRPC clients
- `viper`-based config loading
- OpenTelemetry tracing
- integration with `logger` and `security`

## Installation

```bash
go get github.com/PointerByte/QuicksGo/config
```

## Packages

- `server/gin`: HTTP server bootstrap with Gin, shared middleware, base routes, and graceful shutdown
- `server/grpc`: gRPC server bootstrap, interceptors, TLS/mTLS, and signal-based shutdown
- `client/http`: generic HTTP client with tracing and response decoding
- `client/grpc`: gRPC client wrapper with tracing, TLS, and mTLS
- `utilities`: shared helpers, including `application.yml` / `application.json` loading
- `utilities/traces`: OTEL initialization plus HTTP and gRPC middleware/interceptors
- `utilities/jobs`: simple background jobs
- `proto`: example protobuf contract used by tests and runnable examples

## Configuration

This dependency loads configuration through `utilities.LoadEnv`, using the following priority:

1. `application.yml`
2. `application.json`
3. `.env`
4. `.env.local`
5. environment variables mapped automatically from the loaded `viper` structure

If `application.yml` exists, it is preferred over `application.json`.

### Recommended format: YAML

```yaml
app:
  name: quicksgo-server
  version: 0.0.1

server:
  port: ":8080"
  grpc:
    port: ":50051"
  groups:
    - /api/v1
  gin:
    mode: release
    UseH2C: true
    rate:
      limit: 1000
      burst: 2000

logger:
  dir: logs
  level: info

jwt:
  enable: true
  transport: cookie
  cookie:
    name: session_token
  algorithm: HS256
  hmac:
    secret: change-me
```

### Optional format: JSON

```json
{
  "app": {
    "name": "quicksgo-server",
    "version": "0.0.1"
  },
  "server": {
    "port": ":8080",
    "grpc": {
      "port": ":50051"
    },
    "groups": ["/api/v1"],
    "gin": {
      "mode": "release",
      "UseH2C": true,
      "rate": {
        "limit": 1000,
        "burst": 2000
      }
    }
  },
  "logger": {
    "dir": "logs",
    "level": "info"
  },
  "jwt": {
    "enable": true,
    "transport": "cookie",
    "cookie": {
      "name": "session_token"
    },
    "algorithm": "HS256",
    "hmac": {
      "secret": "change-me"
    }
  }
}
```

### Environment variables

Overrides are generated from each property path. Examples:

- `app.name` -> `APP_NAME`
- `server.port` -> `SERVER_PORT`
- `server.grpc.port` -> `SERVER_GRPC_PORT`
- `server.gin.rate.limit` -> `SERVER_GIN_RATE_LIMIT`

### Configuration reference

The example `application.yml` and `application.json` include the most relevant keys used today by `config`, `logger`, and `security`.

#### `app`

- `app.name`: service name used by health endpoints, logger metadata, and OTEL resource naming.
- `app.version`: service version reported by health endpoints and OTEL resource metadata.

#### `server`

- `server.groups`: Gin route groups created automatically by `server/gin.CreateApp()`.
- `server.modeTest`: helper flag used in tests to disable or simplify runtime behavior.

#### `server.gin`

- `server.gin.port`: HTTP listen address used by the Gin server.
- `server.gin.mode`: Gin runtime mode, typically `debug`, `release`, or `test`.
- `server.gin.UseH2C`: enables HTTP/2 cleartext support on the Gin server handler.
- `server.gin.rate.limit`: request rate allowed by the built-in limiter middleware.
- `server.gin.rate.burst`: burst size used together with the rate limit.

#### `server.gin.LoggerWithConfig`

- `server.gin.LoggerWithConfig.enabled`: enables or disables the structured HTTP request logger.
- `server.gin.LoggerWithConfig.SkipPaths`: routes that should not be logged by the logger middleware.
- `server.gin.LoggerWithConfig.SkipQueryString`: hides the query string from the logged path when enabled.

#### `server.grpc`

- `server.grpc.port`: gRPC listen address used when no explicit address or listener is injected.

#### `server.grpc.tls`

- `server.grpc.tls.enable`: enables TLS for the gRPC server.
- `server.grpc.tls.certFile`: server certificate path.
- `server.grpc.tls.keyFile`: server private key path.
- `server.grpc.tls.version`: minimum TLS version, for example `tlsv12` or `tlsv13`.

#### `server.grpc.mtls`

- `server.grpc.mtls.enable`: enables mutual TLS validation on the gRPC server.
- `server.grpc.mtls.clientCAFile`: CA file used to validate client certificates.
- `server.grpc.mtls.clientAuth`: client certificate policy. Supported values:
- `request_client_cert`: requests a client certificate, but allows the connection to continue when the client does not provide one.
- `require_any_client_cert`: requires the client to provide a certificate, but does not enforce full trust validation by itself.
- `verify_client_cert_if_given`: makes the certificate optional, but validates it when the client sends one.
- `require_and_verify_client_cert`: requires a client certificate and validates it against the configured client CA. This is the recommended option for strict mTLS.

#### `gin.autotls`

- `gin.autotls.enable`: enables automatic certificate management through `autocert`.
- `gin.autotls.domain`: allowed domain for auto-managed certificates.
- `gin.autotls.dirCache`: local cache directory for `autocert`.
- `gin.autotls.version`: minimum TLS version used by the auto TLS config.

#### `client.grpc.tls`

- `client.grpc.tls.enable`: enables TLS for the outbound gRPC client.
- `client.grpc.tls.caFile`: CA bundle used to validate the remote server certificate.
- `client.grpc.tls.serverName`: expected server name used during certificate validation.
- `client.grpc.tls.version`: minimum TLS version for the client transport.
- `client.grpc.tls.insecureSkipVerify`: disables certificate validation. Useful only in controlled development scenarios.

#### `client.grpc.mtls`

- `client.grpc.mtls.enable`: enables mutual TLS on the outbound gRPC client.
- `client.grpc.mtls.certFile`: client certificate path.
- `client.grpc.mtls.keyFile`: client private key path.

#### `logger`

- `logger.dir`: directory used by `builder.InitLogger` to create the log file.
- `logger.modeTest`: disables logger output during tests when enabled.
- `logger.level`: minimum log level, usually `debug`, `info`, `warn`, or `error`.
- `logger.ignoredHeaders`: headers that must not be included in structured logs.
- `logger.formatter`: output format, such as `json` or `text`, depending on logger configuration.
- `logger.formatDate`: timestamp format used by the formatter.

#### `logger.rotate`

- `logger.rotate.enable`: enables file rotation through `lumberjack`.
- `logger.rotate.maxSize`: maximum log file size in MB before rotation.
- `logger.rotate.maxBackups`: maximum number of rotated files to keep.
- `logger.rotate.maxAge`: maximum age in days for rotated files.
- `logger.rotate.compress`: compresses rotated files when enabled.

#### `traces`

- `traces.SkipPaths`: HTTP paths excluded from Gin OpenTelemetry middleware.

#### `jwt`

- `jwt.enable`: enables or disables JWT middleware enforcement.
- `jwt.transport`: selects where `server/gin.CreateApp()` reads the JWT from. Supported values: `header` and `cookie`.
- `jwt.algorithm`: JWT signing algorithm used by `security`, for example `HS256`, `RS256`, `PS256`, or `EdDSA`.

#### `jwt.cookie`

- `jwt.cookie.name`: cookie name used when `jwt.transport` is `cookie`. If omitted, `security` falls back to `access_token`.

#### `jwt.hmac`

- `jwt.hmac.secret`: shared secret used by HMAC-based JWT algorithms.

#### `jwt.rsa`

- `jwt.rsa.private_key`: RSA private key in string form for signing.
- `jwt.rsa.public_key`: RSA public key in string form for verification.

#### `jwt.eddsa`

- `jwt.eddsa.private_key`: Ed25519 private key in string form for signing.
- `jwt.eddsa.public_key`: Ed25519 public key in string form for verification.

## Gin HTTP server

`server/gin.CreateApp()`:

- loads configuration through `utilities.LoadEnv`
- initializes `logger`
- initializes OpenTelemetry
- creates the `gin.Engine`
- registers shared middleware
- applies `RequireJWT` when `jwt.transport=header` or `RequireJWTCookie` when `jwt.transport=cookie`
- creates route groups from `server.groups`
- registers `/health` and `/refresh` under each group

Basic usage:

```go
package main

import (
	"log"

	serverGin "github.com/PointerByte/QuicksGo/config/server/gin"
)

func main() {
	srv, err := serverGin.CreateApp()
	if err != nil {
		log.Fatal(err)
	}

	api := serverGin.GetRoute("/api/v1")
	api.GET("/hello", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	serverGin.Start(srv)
}
```

Main keys:

- `server.port`
- `server.groups`
- `server.gin.mode`
- `server.gin.UseH2C`
- `server.gin.rate.limit`
- `server.gin.rate.burst`
- `jwt.enable`
- `jwt.transport`
- `jwt.cookie.name`

### Gin refresh endpoint

Each route group created from `server.groups` also gets a `GET /refresh`
endpoint. For example, if the group is `/api/v1`, the refresh endpoint becomes
`/api/v1/refresh`.

What it does:

- calls `jobs.RestartJobs()` through the internal `restartJobs` function variable
- forwards the refresh request to the hosts registered with `SetHostsRefresh(...)`
- preserves inbound headers while adding `broadcast-refresh=true`
- avoids infinite loops by returning immediately when the request already
  carries that broadcast marker

Typical use:

- reload cache or in-memory state after configuration changes
- reschedule package-level background jobs across multiple instances
- keep a cluster of Gin services synchronized through a simple fan-out request

If you need local refresh logic before fan-out, register callbacks with
`SetFunctionsRefresh(...)`.

## gRPC server

`server/grpc`:

- loads configuration with `utilities.LoadEnv(".")` when `Serve()` runs
- resolves `server.grpc.port` from `viper`
- integrates `logger` and `traces` interceptors
- supports TLS and mTLS
- listens to `Ctrl+C` / `SIGTERM` and performs `GracefulStop()`

Basic usage:

```go
package main

import (
	"context"
	"log"

	pb "github.com/PointerByte/QuicksGo/config/proto"
	serverGRPC "github.com/PointerByte/QuicksGo/config/server/grpc"
	"google.golang.org/grpc"
)

type greeterServer struct {
	pb.UnimplementedGreeterServer
}

func (s greeterServer) SayHello(_ context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
	return &pb.HelloReply{Message: "hello " + req.GetName()}, nil
}

func (s greeterServer) CreateChat(stream grpc.ClientStreamingServer[pb.ChatMessage, pb.ChatSummary]) error {
	return nil
}

func (s greeterServer) StreamAlerts(stream grpc.BidiStreamingServer[pb.AlertMessage, pb.AlertMessage]) error {
	return nil
}

func main() {
	srv := serverGRPC.NewIConfig(nil, nil)

	if err := srv.Register(func(r grpc.ServiceRegistrar) {
		pb.RegisterGreeterServer(r, greeterServer{})
	}); err != nil {
		log.Fatal(err)
	}

	log.Fatal(srv.Serve())
}
```

Main keys:

- `server.grpc.port`
- `server.grpc.tls.enable`
- `server.grpc.tls.certFile`
- `server.grpc.tls.keyFile`
- `server.grpc.tls.version`
- `server.grpc.mtls.enable`
- `server.grpc.mtls.clientCAFile`
- `server.grpc.mtls.clientAuth`

### gRPC refresh endpoint

`server/grpc` exposes an administrative refresh RPC through the internal method
name `"/quicksgo.admin/Refresh"`.

What it does:

- calls `jobs.RestartJobs()` through the internal `restartJobs` function variable
- executes callbacks registered with `SetFunctionsRefresh(...)`
- propagates the same RPC to the hosts registered with `SetHostsRefresh(...)`
- uses metadata `broadcast-refresh=true` to avoid fan-out loops between nodes

Typical use:

- refresh process-local state on all nodes
- restart package-level scheduled jobs in a coordinated way
- propagate a manual or operational reload event across gRPC instances

The request and response body for this administrative RPC are empty, and the
flow is intended for internal service-to-service coordination.

## HTTP client

`client/http` exposes a generic REST client with request/response tracing.

Basic usage:

```go
client := clientHttp.NewGenericRest(nil, 10*time.Second, nil)

err := client.GetGeneric(ctx, clientHttp.RequestGeneric{
	System:   "users-service",
	Process:  "list-users",
	Host:     "https://api.example.com",
	Path:     "users",
	Response: &usersResponse,
})
```

## gRPC client

`client/grpc` wraps `grpc.ClientConn` and can:

- build protobuf clients through `BuildClient`
- resolve TLS/mTLS from `viper`
- trace metadata, request, and response through `logger`

Basic usage:

```go
cli := clientGRPC.NewIClient(nil, nil)

greeter, err := clientGRPC.BuildClient(cli, pb.NewGreeterClient)
if err != nil {
	panic(err)
}
```

Main keys:

- `client.grpc.tls.enable`
- `client.grpc.tls.caFile`
- `client.grpc.tls.serverName`
- `client.grpc.tls.version`
- `client.grpc.tls.insecureSkipVerify`
- `client.grpc.mtls.enable`
- `client.grpc.mtls.certFile`
- `client.grpc.mtls.keyFile`

## Background jobs

`utilities/jobs` provides simple in-process background execution for recurring tasks.

There are two common ways to use it:

- package-level helpers like `jobs.Job(...)`, `jobs.CronJob(...)`, and `jobs.StartJobs()`
- an isolated instance created with `jobs.NewJobs()` when you want separate lifecycle control

Important behavior:

- jobs do not start when they are registered; they start when `StartJobs()` runs
- `server/gin.Start(...)` already calls `jobs.StartJobs()` internally
- if you register jobs after `StartJobs()`, they start immediately
- when `server.modeTest=true`, `StartJobs()` does not run jobs

### Package-level lifecycle helpers

These helpers operate on the process-wide scheduler used by the package:

#### `StartJobs()`

Starts the package-level scheduler loop.

What it does:

- starts the registered jobs in the global scheduler
- keeps an internal watcher alive while jobs are marked as running
- is the entry point used by `server/gin.Start(...)`

Notes:

- jobs must already be registered with `Job(...)` or `CronJob(...)`
- if `server.modeTest=true`, jobs are not started

#### `RestartJobs()`

Requests a restart of the global scheduler.

What it does:

- sends an internal restart signal
- causes the scheduler to stop current jobs without clearing their definitions
- starts them again from the current state

Useful when:

- configuration changed and you want the current set of jobs to restart
- you need to reschedule active jobs without rebuilding the process

#### `StopAllJobs(clearJobs bool)`

Stops all jobs registered globally with `NewJobs()`.

Behavior:

- if `clearJobs=false`, jobs are stopped but remain registered, so they can be started again
- if `clearJobs=true`, jobs are stopped and their definitions are removed from each registered instance

Typical use:

- `StopAllJobs(false)` for temporary stop / restart flows
- `StopAllJobs(true)` for tests, shutdown, or full reset

#### `CheckStatusJobs() bool`

Returns whether the package currently considers the global jobs system active.

In practice:

- `true` means jobs were started and not fully stopped yet
- `false` means the global scheduler is stopped

This is mainly useful for diagnostics, tests, or operational checks.

### `Job(fn func(), interval time.Duration, timeout *time.Duration)`

Use `Job` for fixed intervals such as "every 30 seconds" or "every 5 minutes".

Parameters:

- `fn`: function executed on every cycle
- `interval`: execution frequency; if `interval <= 0` the job is ignored
- `timeout`: optional total lifetime for that job; if `nil`, the job keeps running until shutdown or `StopAllJobs`

Behavior:

- the first execution happens immediately when the job starts
- later executions use the provided `interval`
- if `timeout != nil` and `*timeout > 0`, the job stops automatically when the timeout expires

Example without timeout:

```go
import (
	"time"

	"github.com/PointerByte/QuicksGo/config/utilities/jobs"
)

func registerJobs() {
	jobs.Job(func() {
		refreshCache()
	}, 30*time.Second, nil)
}
```

Example with timeout:

```go
func registerJobs() {
	timeout := 10 * time.Minute

	jobs.Job(func() {
		pollTemporarySource()
	}, 15*time.Second, &timeout)
}
```

### `CronJob(fn func(), trigger CronTrigger, interval time.Duration)`

Use `CronJob` when the first execution must be aligned to a specific hour/minute/second.

Parameters:

- `fn`: function to execute
- `trigger`: daily start time using `Hour`, `Minute`, and `Second`
- `interval`: controls what happens after the first aligned execution

Behavior:

- if `interval <= 0`, the job runs once per day at the given `trigger`
- if `interval > 0`, the first run waits until the next matching `trigger`, and after that it repeats every `interval`

Example: run every day at 09:00:00

```go
func registerJobs() {
	jobs.CronJob(func() {
		buildDailyReport()
	}, jobs.CronTrigger{
		Hour:   9,
		Minute: 0,
		Second: 0,
	}, 0)
}
```

Example: first run at 08:30:00, then every 5 minutes

```go
func registerJobs() {
	jobs.CronJob(func() {
		syncMorningWindow()
	}, jobs.CronTrigger{
		Hour:   8,
		Minute: 30,
		Second: 0,
	}, 5*time.Minute)
}
```

### Complete package-level example

```go
func registerJobs() {
	timeout := 30 * time.Minute

	jobs.Job(func() {
		refreshCache()
	}, time.Minute, &timeout)

	jobs.CronJob(func() {
		buildDailyReport()
	}, jobs.CronTrigger{
		Hour:   2,
		Minute: 0,
		Second: 0,
	}, 0)
}

func main() {
	registerJobs()

	jobs.StartJobs()

	if jobs.CheckStatusJobs() {
		log.Println("jobs are running")
	}

	// Example operational restart:
	jobs.RestartJobs()

	// Example final shutdown:
	jobs.StopAllJobs(true)
}
```

### Recommended integration pattern

When you use the Gin server, a common pattern is:

```go
func main() {
	registerJobs()

	srv, err := serverGin.CreateApp()
	if err != nil {
		log.Fatal(err)
	}

	serverGin.Start(srv) // this starts registered jobs too
}
```

If you want manual control outside the Gin bootstrap, call `jobs.StartJobs()` explicitly:

```go
func main() {
	jobs.Job(func() {
		cleanup()
	}, time.Minute, nil)

	jobs.StartJobs()

	select {}
}
```

## Runnable example

The project includes a runnable example in [main.go](/e:/Proyects/Practices/QuicksGo/config/main.go).

Run the Gin example:

```powershell
$env:QUICKSGO_EXAMPLE_SERVER="gin"
go run .
```

Run the gRPC example:

```powershell
$env:QUICKSGO_EXAMPLE_SERVER="grpc"
go run .
```

## Protobuf

Required commands:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
protoc --go_out=. --go-grpc_out=. proto/methods.proto
```

## Tests

```bash
go test ./...
```

## Useful commands

### Update dependencies

```bash
go get -u ./...
```

### Clear build, test, and module cache

```bash
go clean -cache -testcache -modcache
```

### Run tests with coverage

```bash
go test -cover -covermode=atomic -coverprofile="coverage.out" ./...
```

### Generate HTML coverage report

```bash
go tool cover -html="coverage.out" -o "coverage.html"
```

### Show per-function coverage

```bash
go tool cover -func="coverage.out"
```
