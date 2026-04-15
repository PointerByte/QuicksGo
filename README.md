# QuicksGo

QuicksGo is a modular framework for bootstrapping Go services with a shared approach to configuration, transport, observability, and security.

It helps you assemble applications faster with:

- Gin HTTP servers
- gRPC servers
- HTTP and gRPC clients
- structured logging
- OpenTelemetry tracing
- JWT and security middleware
- `viper`-based configuration loading
- simple in-process background jobs

## Main modules

- [config](/e:/Proyects/Practices/QuicksGo/config): server and client bootstrap, configuration loading, tracing, and jobs
- [logger](/e:/Proyects/Practices/QuicksGo/logger/README.md): structured logging plus HTTP and gRPC middleware
- [security](/e:/Proyects/Practices/QuicksGo/security/README.md): JWT, security middleware, and cryptographic helpers
- [cmd/qgo](/e:/Proyects/Practices/QuicksGo/cmd/qgo/README.md): CLI for scaffolding new Gin and gRPC services

## How the pieces fit together

A typical QuicksGo application flow looks like this:

1. `config/utilities.LoadEnv` loads `application.yml` or `application.json` into `viper`
2. `config` initializes `logger`
3. `config` initializes OpenTelemetry tracing
4. `config/server/gin` or `config/server/grpc` starts the server
5. `security` consumes the same shared `viper` configuration
6. `config/client/http` and `config/client/grpc` reuse tracing and logging for outbound calls

## Installation

Install the root module:

```bash
go get github.com/PointerByte/QuicksGo
```

Or install only the module you need:

```bash
go get github.com/PointerByte/QuicksGo/config
go get github.com/PointerByte/QuicksGo/logger
go get github.com/PointerByte/QuicksGo/security
```

## Configuration model

The complete framework templates are available at:

- [config/application.yml](/e:/Proyects/Practices/QuicksGo/config/application.yml)
- [config/application.json](/e:/Proyects/Practices/QuicksGo/config/application.json)

Supported load priority:

1. `application.yml`
2. `application.json`
3. `.env`
4. `.env.local`
5. environment variables

YAML is the recommended format for new applications.

### Example YAML

```yaml
app:
  name: quicksgo-server
  version: 0.0.1

server:
  groups:
    - /api/v1
  gin:
    port: ":8080"
    mode: release
    UseH2C: true
    rate:
      limit: 1000
      burst: 2000
  grpc:
    port: ":50051"

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

### Environment variable mapping

Overrides are generated from the key path. Examples:

- `app.name` -> `APP_NAME`
- `server.port` -> `SERVER_PORT`
- `server.gin.port` -> `SERVER_GIN_PORT`
- `server.grpc.port` -> `SERVER_GRPC_PORT`
- `client.grpc.tls.serverName` -> `CLIENT_GRPC_TLS_SERVERNAME`
- `jwt.hmac.secret` -> `JWT_HMAC_SECRET`

## Configuration reference

The example templates include the keys most commonly used across `config`, `logger`, and `security`.

### `app`

- `app.name`: service name used by health endpoints, logger metadata, and OTEL resource naming
- `app.version`: service version reported by health endpoints and OTEL metadata

### `server`

- `server.groups`: Gin route groups created automatically by `config/server/gin`
- `server.modeTest`: helper flag used in tests to simplify runtime behavior

### `server.gin`

- `server.gin.port`: HTTP listen address
- `server.gin.mode`: Gin mode such as `debug`, `release`, or `test`
- `server.gin.UseH2C`: enables HTTP/2 cleartext support
- `server.gin.rate.limit`: built-in rate limiter request rate
- `server.gin.rate.burst`: burst size for the limiter

### `server.gin.LoggerWithConfig`

- `server.gin.LoggerWithConfig.enabled`: enables the structured HTTP request logger
- `server.gin.LoggerWithConfig.SkipPaths`: routes skipped by the logger middleware
- `server.gin.LoggerWithConfig.SkipQueryString`: hides the query string from the logged path

### `server.grpc`

- `server.grpc.port`: gRPC listen address

### `server.grpc.tls`

- `server.grpc.tls.enable`: enables TLS on the gRPC server
- `server.grpc.tls.certFile`: server certificate path
- `server.grpc.tls.keyFile`: server private key path
- `server.grpc.tls.version`: minimum TLS version such as `tlsv12` or `tlsv13`

### `server.grpc.mtls`

- `server.grpc.mtls.enable`: enables mTLS validation on the gRPC server
- `server.grpc.mtls.clientCAFile`: CA file used to validate client certificates
- `server.grpc.mtls.clientAuth`: client certificate policy

Supported `server.grpc.mtls.clientAuth` values:

- `request_client_cert`
- `require_any_client_cert`
- `verify_client_cert_if_given`
- `require_and_verify_client_cert`

### `gin.autotls`

- `gin.autotls.enable`: enables automatic certificate management through `autocert`
- `gin.autotls.domain`: allowed domain for managed certificates
- `gin.autotls.dirCache`: local cache directory for `autocert`
- `gin.autotls.version`: minimum TLS version for auto TLS

### `client.grpc.tls`

- `client.grpc.tls.enable`: enables TLS on the outbound gRPC client
- `client.grpc.tls.caFile`: CA bundle used to validate the remote server certificate
- `client.grpc.tls.serverName`: expected server name during certificate validation
- `client.grpc.tls.version`: minimum TLS version for the client transport
- `client.grpc.tls.insecureSkipVerify`: disables certificate validation and should be used only in controlled development scenarios

### `client.grpc.mtls`

- `client.grpc.mtls.enable`: enables mTLS on the outbound gRPC client
- `client.grpc.mtls.certFile`: client certificate path
- `client.grpc.mtls.keyFile`: client private key path

### `logger`

- `logger.dir`: directory used to create the log file
- `logger.modeTest`: disables logger output during tests
- `logger.level`: minimum log level such as `debug`, `info`, `warn`, or `error`
- `logger.ignoredHeaders`: headers that must not appear in structured logs
- `logger.formatter`: output format such as `json` or `text`
- `logger.formatDate`: timestamp format used by the formatter

### `logger.rotate`

- `logger.rotate.enable`: enables file rotation through `lumberjack`
- `logger.rotate.maxSize`: maximum log file size in MB before rotation
- `logger.rotate.maxBackups`: maximum number of rotated files to keep
- `logger.rotate.maxAge`: maximum age in days for rotated files
- `logger.rotate.compress`: compresses rotated files when enabled

### `traces`

- `traces.SkipPaths`: HTTP paths excluded from Gin OpenTelemetry middleware

### `jwt`

- `jwt.enable`: enables or disables JWT middleware enforcement
- `jwt.transport`: JWT source used by Gin middleware. Supported values: `header` and `cookie`
- `jwt.algorithm`: signing algorithm such as `HS256`, `RS256`, `PS256`, or `EdDSA`

### `jwt.cookie`

- `jwt.cookie.name`: cookie name used when `jwt.transport` is `cookie`

### `jwt.hmac`

- `jwt.hmac.secret`: shared secret used by HMAC-based JWT algorithms

### `jwt.rsa`

- `jwt.rsa.private_key`: RSA private key in string form for signing
- `jwt.rsa.public_key`: RSA public key in string form for verification

### `jwt.eddsa`

- `jwt.eddsa.private_key`: Ed25519 private key in string form for signing
- `jwt.eddsa.public_key`: Ed25519 public key in string form for verification

## Observability

Services bootstrapped with QuicksGo are already prepared for OpenTelemetry-based observability.

That includes:

- traces
- logs
- metrics

QuicksGo is also compatible with the OpenTelemetry Go Auto Instrumentation SDK when your deployment strategy needs automatic instrumentation on top of the framework setup.

## HTTP server

`config/server/gin.CreateApp()`:

- loads configuration
- initializes logger
- initializes OpenTelemetry
- creates the `gin.Engine`
- registers shared middleware
- applies JWT middleware when configured
- creates groups from `server.groups`
- registers `/health` and `/refresh` for each group

Basic usage:

```go
package main

import (
	"log"

	"github.com/gin-gonic/gin"

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

### Gin refresh endpoint

Each route group created from `server.groups` also gets a `GET /refresh` endpoint.

It is intended for:

- reloading cache or in-memory state
- restarting background jobs across instances
- propagating refresh events to peers registered with `SetHostsRefresh(...)`

If you need local refresh logic before fan-out, register callbacks with `SetFunctionsRefresh(...)`.

## gRPC server

`config/server/grpc`:

- loads configuration when `Serve()` runs
- resolves `server.grpc.port` from `viper`
- integrates logger and trace interceptors
- supports TLS and mTLS
- listens to shutdown signals and performs `GracefulStop()`

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

### gRPC refresh endpoint

`config/server/grpc` exposes an internal administrative refresh RPC through `"/quicksgo.admin/Refresh"`.

It is intended for:

- refreshing process-local state on all nodes
- restarting package-level scheduled jobs in a coordinated way
- propagating internal reload events across gRPC instances

## HTTP client

`config/client/http` exposes a generic REST client with request and response tracing.

```go
client := clientHttp.NewGenericRest(10*time.Second, nil)

err := client.GetGeneric(ctx, clientHttp.RequestGeneric{
	System:   "users-service",
	Process:  "list-users",
	Host:     "https://api.example.com",
	Path:     "users",
	Response: &usersResponse,
})
```

## gRPC client

`config/client/grpc` wraps `grpc.ClientConn` and can:

- build protobuf clients through `BuildClient`
- resolve TLS and mTLS from `viper`
- trace metadata, request, and response through `logger`

```go
cli := clientGRPC.NewIClient(nil)

greeter, err := clientGRPC.BuildClient(cli, pb.NewGreeterClient)
if err != nil {
	panic(err)
}
```

## Background jobs

`config/utilities/jobs` provides simple in-process recurring tasks.

Common entry points:

- `jobs.Job(...)`
- `jobs.CronJob(...)`
- `jobs.StartJobs()`
- `jobs.RestartJobs()`
- `jobs.StopAllJobs(...)`
- `jobs.CheckStatusJobs()`

Important behavior:

- jobs start only after `StartJobs()` runs
- `config/server/gin.Start(...)` already calls `jobs.StartJobs()`
- jobs registered after `StartJobs()` starts are launched immediately
- when `server.modeTest=true`, jobs do not run

Example:

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

	srv, err := serverGin.CreateApp()
	if err != nil {
		log.Fatal(err)
	}

	serverGin.Start(srv)
}
```

## Runnable example

The `config` module includes a runnable example in [config/main.go](/e:/Proyects/Practices/QuicksGo/config/main.go).

Run the Gin example:

```powershell
$env:QUICKSGO_EXAMPLE_SERVER="gin"
go run ./config
```

Run the gRPC example:

```powershell
$env:QUICKSGO_EXAMPLE_SERVER="grpc"
go run ./config
```

## Recommended usage

If you are starting a new application with QuicksGo:

1. start from [config/application.yml](/e:/Proyects/Practices/QuicksGo/config/application.yml)
2. load configuration with `config/utilities.LoadEnv`
3. use `config/server/gin` or `config/server/grpc` as your bootstrap layer
4. define your routes or protobuf services
5. use `security` for JWT and endpoint protection
6. use `config/client/http` or `config/client/grpc` for traced outbound calls
7. use `config/utilities/jobs` when you need lightweight recurring background work

You can also scaffold a new service with `qgo`:

```bash
go install github.com/PointerByte/QuicksGo/cmd/qgo@latest
qgo new gin
qgo new grpc
```

## Protobuf

Required commands:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
protoc --go_out=. --go-grpc_out=. config/proto/methods.proto
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

### Clean build, test, and module cache

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

### Show coverage by function

```bash
go tool cover -func="coverage.out"
```
