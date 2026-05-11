# GoForge

GoForge is a modular Go toolkit for bootstrapping service-oriented
applications with shared conventions for configuration, HTTP and gRPC
transport, logging, OpenTelemetry, JWT security, background jobs, and small
runtime utilities.

The repository is organized as a Go workspace. The root module contains the
service bootstrap packages under `config` and `tools`; `logger`, `security`,
`encrypt`, and the CLIs are separate modules that can also be consumed on their
own.

Spanish documentation is available in [README.es.md](./README.es.md).

## What It Includes

- Gin HTTP server bootstrap with common middleware
- gRPC server bootstrap with unary and stream interceptors
- HTTP and gRPC clients with tracing hooks
- structured logging through the `logger` module
- OpenTelemetry traces, metrics, and instrumentation helpers
- JWT middleware through the `security` module
- local and cloud-backed cryptographic helpers through `encrypt`
- fixed-interval background jobs and simple worker utilities
- ECS and Kubernetes helpers for refresh fan-out host discovery
- CLIs for service scaffolding and certificate generation

## Modules

- [root module](./go.mod): `github.com/PointerByte/GoForge`
- [logger](./logger/README.md): structured logging plus HTTP and gRPC middleware
- [security](./security/README.md): JWT services and Gin security middleware
- [encrypt](./encrypt/README.md): symmetric crypto, hashing, RSA, signatures, and KMS-oriented backends
- [cmd/qgo](./cmd/qgo/README.md): CLI for scaffolding Gin and gRPC services
- [cmd/go-openssl](./cmd/go-openssl/README.md): CLI for generating and reading PEM certificates and keys

## Installation

Install the root module:

```bash
go get github.com/PointerByte/GoForge
```

Or install only the modules you need:

```bash
go get github.com/PointerByte/GoForge/logger
go get github.com/PointerByte/GoForge/security
go get github.com/PointerByte/GoForge/encrypt
```

Install the CLIs:

```bash
go install github.com/PointerByte/GoForge/cmd/qgo@latest
go install github.com/PointerByte/GoForge/cmd/go-openssl@latest
```

## Configuration

`config/utilities.LoadEnv(prefixPath)` loads configuration into `viper` from
the provided directory. It checks these files in order:

1. `application.yml`
2. `application.yaml`
3. `application.json`

After the application file is read, it merges `.env` and `.env.local` from the
current working directory and enables environment variables. Environment
overrides are generated from the existing configuration key path.

Examples:

- `app.name` -> `APP_NAME`
- `server.gin.port` -> `SERVER_GIN_PORT`
- `server.gin.groups` -> `SERVER_GIN_GROUPS`
- `server.grpc.port` -> `SERVER_GRPC_PORT`
- `client.http.timeout` -> `CLIENT_HTTP_TIMEOUT`
- `client.grpc.tls.serverName` -> `CLIENT_GRPC_TLS_SERVERNAME`
- `jwt.hmac.secret` -> `JWT_HMAC_SECRET`

YAML is the recommended format for new applications.

### Minimal YAML

```yaml
app:
  name: GoForge-service
  version: 0.0.1

server:
  modeTest: false
  gin:
    port: ":8080"
    mode: release
    groups:
      - /api/v1
    UseH2C: true
    rate:
      limit: 1000
      burst: 2000
    LoggerWithConfig:
      enabled: true
      SkipPaths:
        - /api/v1/health
      SkipQueryString: false
  grpc:
    port: ":50051"

client:
  http:
    timeout: 5s

logger:
  dir: logs
  level: info
  formatter: json
  formatDate: "2006-01-02T15:04:05.000"
  ignoredHeaders:
    - Authorization
    - Cookie

traces:
  enable: false
  SkipPaths:
    - /api/v1/health

jwt:
  enable: false
  transport: header
  eddsa:
    private_key: ./certs/jwt/ed25519-key.pem
    public_key: ./certs/jwt/ed25519-public.pem
```

## Main Configuration Keys

### Application

- `app.name`: service name used by health endpoints and OpenTelemetry resource metadata
- `app.version`: service version reported by health endpoints and telemetry metadata
- `server.modeTest`: disables runtime behaviors such as background job execution during tests

### Gin Server

- `server.gin.port`: HTTP listen address
- `server.gin.mode`: Gin mode, for example `debug`, `release`, or `test`
- `server.gin.groups`: route groups created by `config/server/gin`
- `server.gin.UseH2C`: enables HTTP/2 cleartext support
- `server.gin.rate.limit`: request rate for the built-in limiter; `0` disables it
- `server.gin.rate.burst`: burst size for the limiter
- `server.gin.LoggerWithConfig.enabled`: enables structured HTTP request logs
- `server.gin.LoggerWithConfig.SkipPaths`: paths skipped by the request logger
- `server.gin.LoggerWithConfig.SkipQueryString`: hides query strings in logged paths

Gin also supports `server.gin.autotls.*`, `server.gin.tls.*`, and
`server.gin.mtls.*` settings for automatic TLS, explicit TLS, and mTLS.

Supported TLS versions are `tlsv10`, `tlsv11`, `tlsv12`, and `tlsv13`.
Supported client-auth values include `no_client_cert`, `request_client_cert`,
`require_any_client_cert`, `verify_client_cert_if_given`, and
`require_and_verify_client_cert`.

### gRPC Server

- `server.grpc.port`: gRPC listen address
- `server.grpc.tls.enable`: enables TLS on the gRPC server
- `server.grpc.tls.certFile`: server certificate path
- `server.grpc.tls.keyFile`: server private key path
- `server.grpc.tls.version`: minimum TLS version
- `server.grpc.mtls.enable`: enables mTLS validation
- `server.grpc.mtls.clientCAFile`: CA file used to validate client certificates
- `server.grpc.mtls.clientAuth`: client certificate policy

### Clients

- `client.http.timeout`: default timeout for configured HTTP clients
- `client.http.tls.*`: outbound HTTP TLS settings
- `client.http.mtls.*`: outbound HTTP mTLS client certificate settings
- `client.grpc.tls.*`: outbound gRPC TLS settings
- `client.grpc.mtls.*`: outbound gRPC mTLS client certificate settings

### Logger, Traces, and JWT

- `logger.dir`: directory for log files
- `logger.level`: minimum log level such as `debug`, `info`, `warn`, or `error`
- `logger.ignoredHeaders`: headers removed from structured logs
- `logger.formatter`: output format, usually `json` or `text`
- `logger.rotate.*`: file rotation settings
- `traces.enable`: enables OpenTelemetry initialization
- `traces.SkipPaths`: HTTP paths skipped by Gin OpenTelemetry middleware
- `jwt.enable`: enables JWT middleware enforcement
- `jwt.transport`: token source, usually `header` or `cookie`
- `jwt.cookie.name`: cookie name when `jwt.transport` is `cookie`
- `jwt.algorithm`: signing algorithm such as `HS256`, `RS256`, `PS256`, or `EdDSA`; optional when only one strategy is configured
- `jwt.hmac.secret`, `jwt.rsa.*`, `jwt.eddsa.*`: signing configuration; configure one strategy per service or set `jwt.algorithm` when multiple strategies exist

## HTTP Server

`config/server/gin.CreateApp()` loads configuration, initializes the logger and
OpenTelemetry, creates the shared `gin.Engine`, registers common middleware,
creates route groups from `server.gin.groups`, and registers `/health` and
`/refresh` under each group.

```go
package main

import (
	"log"

	serverGin "github.com/PointerByte/GoForge/config/server/gin"
	"github.com/gin-gonic/gin"
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

`GET /refresh` restarts registered package-level jobs, runs callbacks
registered with `SetFunctionsRefresh(...)`, and can fan out to peers registered
with `SetHostsRefresh(...)`.

## gRPC Server

`config/server/grpc` loads configuration in `Serve()`, resolves
`server.grpc.port`, attaches logging and OpenTelemetry interceptors, supports
TLS and mTLS, and performs graceful shutdown on process signals.

```go
package main

import (
	"context"
	"log"

	pb "github.com/PointerByte/GoForge/config/proto"
	serverGRPC "github.com/PointerByte/GoForge/config/server/grpc"
	"google.golang.org/grpc"
)

type greeterServer struct {
	pb.UnimplementedGreeterServer
}

func (greeterServer) SayHello(_ context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
	return &pb.HelloReply{Message: "hello " + req.GetName()}, nil
}

func (greeterServer) CreateChat(stream grpc.ClientStreamingServer[pb.ChatMessage, pb.ChatSummary]) error {
	return nil
}

func (greeterServer) StreamAlerts(stream grpc.BidiStreamingServer[pb.AlertMessage, pb.AlertMessage]) error {
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

## Clients

HTTP:

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

gRPC:

```go
cli := clientGRPC.NewIClient(nil)
cli.SetAddress("localhost:50051")

greeter, err := clientGRPC.BuildClient(cli, pb.NewGreeterClient)
if err != nil {
	panic(err)
}
```

Use `NewGenericRestFromConfig()` or the gRPC client TLS settings when you want
client transports to be built from `viper`.

## Background Work

`tools/jobs` provides fixed-interval in-process jobs. Jobs begin when
`jobs.StartJobs()` runs; `config/server/gin.Start(...)` calls it automatically.
When `server.modeTest=true`, jobs are not started.

```go
func registerJobs() {
	timeout := 30 * time.Minute

	jobs.Job(func() {
		refreshCache()
	}, time.Minute, &timeout)
}
```

`tools/workers` provides a small bounded worker loop through `SetWorkersLimit`,
`RunWorkers`, `AddTask`, `StopWorkers`, and `RestartWorkers`.

## Runtime Examples

The root module includes runnable examples in [main.go](./main.go).
Use an application file like the minimal YAML above before starting the Gin
example; it expects `/api/v1` in `server.gin.groups`.

Run the Gin example:

```bash
GoForge_EXAMPLE_SERVER=gin go run .
```

Run the gRPC example:

```bash
GoForge_EXAMPLE_SERVER=grpc go run .
```

## Development

The workspace uses Go `1.25.0`.

Run tests for the root module:

```bash
go test ./...
```

Run tests for a workspace module:

```bash
cd logger
go test ./...
```

Generate protobuf files after editing `config/proto/methods.proto`:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
protoc --go_out=. --go-grpc_out=. config/proto/methods.proto
```

Coverage for the current module:

```bash
go test -cover -covermode=atomic -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html
```
