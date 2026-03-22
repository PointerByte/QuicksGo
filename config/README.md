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

- `server_Gin`: HTTP server bootstrap with Gin, shared middleware, base routes, and graceful shutdown
- `server_gRPC`: gRPC server bootstrap, interceptors, TLS/mTLS, and signal-based shutdown
- `clientHttp`: generic HTTP client with tracing and response decoding
- `client_gRPC`: gRPC client wrapper with tracing, TLS, and mTLS
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

- `server.groups`: Gin route groups created automatically by `server_Gin.CreateApp()`.
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
- `server.grpc.mtls.clientAuth`: client certificate policy. Supported values include `request_client_cert`, `require_any_client_cert`, `verify_client_cert_if_given`, and `require_and_verify_client_cert`.

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
- `jwt.transport`: selects where `server_Gin.CreateApp()` reads the JWT from. Supported values: `header` and `cookie`.
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

`server_Gin.CreateApp()`:

- loads configuration through `utilities.LoadEnv`
- initializes `logger`
- initializes OpenTelemetry
- creates the `gin.Engine`
- registers shared middleware
- applies `RequireJWT` when `jwt.transport=header` or `RequireJWTCookie` when `jwt.transport=cookie`
- creates route groups from `server.groups`
- registers `/health` under each group

Basic usage:

```go
package main

import (
	"log"

	serverGin "github.com/PointerByte/QuicksGo/config/server_Gin"
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

## gRPC server

`server_gRPC`:

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
	serverGRPC "github.com/PointerByte/QuicksGo/config/server_gRPC"
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
	srv := serverGRPC.NewIUnitary(nil, nil)

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

## HTTP client

`clientHttp` exposes a generic REST client with request/response tracing.

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

`client_gRPC` wraps `grpc.ClientConn` and can:

- build protobuf clients through `BuildClient`
- resolve TLS/mTLS from `viper`
- trace metadata, request, and response through `logger`

Basic usage:

```go
cli := client_gRPC.NewIClient(nil, nil)

greeter, err := client_gRPC.BuildClient(cli, pb.NewGreeterClient)
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

## Runnable example

The project includes a runnable example in [main.go](/e:/Proyects/Practices/QuicksGoV2t/config/main.go).

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
