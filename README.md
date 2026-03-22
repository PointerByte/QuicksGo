# QuicksGo

QuicksGo is a modular Go framework for building services with:

- Gin HTTP servers
- gRPC servers
- HTTP and gRPC clients
- structured logging
- OpenTelemetry tracing
- JWT and security middleware
- `viper`-based configuration loading

## Main modules

- [config](/e:/Proyects/Practices/QuicksGoV2t/config/README.md): server/client bootstrap plus configuration and tracing utilities
- [logger](/e:/Proyects/Practices/QuicksGoV2t/logger/README.md): structured logging, HTTP/gRPC middleware, and satellite traces
- [security](/e:/Proyects/Practices/QuicksGoV2t/security/README.md): JWT, security middleware, and cryptographic helpers

## How the dependencies fit together

A typical QuicksGo application flow looks like this:

1. `config` loads `application.yml` or `application.json` into `viper`
2. `config` initializes `logger`
3. `config` initializes OTEL tracing
4. `server_Gin` or `server_gRPC` starts the server
5. `security` consumes the same `viper` configuration
6. `clientHttp` and `client_gRPC` reuse tracing and logging for outbound calls

## Configuration template

The complete framework template is available at:

- [application.yml](/e:/Proyects/Practices/QuicksGoV2t/config/application.yml)
- [application.json](/e:/Proyects/Practices/QuicksGoV2t/config/application.json)

It includes configuration for:

- `app.*`
- `server.gin.*`
- `server.gin.LoggerWithConfig.*`
- `server.grpc.*`
- `gin.autotls.*`
- `client.grpc.*`
- `logger.*`
- `traces.SkipPaths`
- `jwt.*`

## Recommended format

Even though a JSON template is included, YAML is the recommended format for new applications.

The current `config` loader uses this priority:

1. `application.yml`
2. `application.json`
3. `.env`
4. `.env.local`
5. environment variables

If you use the framework YAML template, use it as the base for your `application.yml`.

## Environment variables

QuicksGo can override file-based configuration using names derived from the full key path.

Examples:

- `app.name` -> `APP_NAME`
- `server.gin.port` -> `SERVER_GIN_PORT`
- `server.grpc.port` -> `SERVER_GRPC_PORT`
- `client.grpc.tls.serverName` -> `CLIENT_GRPC_TLS_SERVERNAME`
- `jwt.hmac.secret` -> `JWT_HMAC_SECRET`

## HTTP server

To start a Gin server with QuicksGo, you typically use `config/server_Gin`:

```go
srv, err := serverGin.CreateApp()
if err != nil {
	panic(err)
}

api := serverGin.GetRoute("/api/v1")
api.GET("/hello", func(c *gin.Context) {
	c.JSON(200, gin.H{"message": "ok"})
})

serverGin.Start(srv)
```

## gRPC server

To start a gRPC server with QuicksGo, you typically use `config/server_gRPC`:

```go
srv := serverGRPC.NewIUnitary(nil, nil)

if err := srv.Register(func(r grpc.ServiceRegistrar) {
	pb.RegisterGreeterServer(r, greeterServer{})
}); err != nil {
	panic(err)
}

panic(srv.Serve())
```

## Runnable example

The `config` module includes a runnable example in [main.go](/e:/Proyects/Practices/QuicksGoV2t/config/main.go).

Gin:

```powershell
cd e:\Proyects\Practices\QuicksGoV2t\config
$env:QUICKSGO_EXAMPLE_SERVER="gin"
go run .
```

gRPC:

```powershell
cd e:\Proyects\Practices\QuicksGoV2t\config
$env:QUICKSGO_EXAMPLE_SERVER="grpc"
go run .
```

## Recommended usage

If you are starting a new application with QuicksGo:

1. start from [application.yml](/e:/Proyects/Practices/QuicksGoV2t/config/application.yml)
2. load configuration with `config/utilities.LoadEnv`
3. use `server_Gin` or `server_gRPC` as your bootstrap layer
4. define your routes or protobuf services
5. use `security` for JWT and endpoint protection
6. use `clientHttp` or `client_gRPC` for traced outbound calls

## Useful commands

Run tests:

```bash
go test ./...
```

Coverage:

```bash
go test -cover -covermode=atomic -coverprofile="coverage.out" ./...
go tool cover -html="coverage.out" -o "coverage.html"
```
