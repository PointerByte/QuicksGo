# QuicksGo

QuicksGo is a framework for bootstrapping Go services with a shared approach to configuration, transport, observability, and security. It helps you assemble HTTP and gRPC applications faster by combining server/client setup, structured logging, OpenTelemetry tracing, JWT protection, and configuration loading around the same conventions.

QuicksGo is a modular Go framework for building services with:

- Gin HTTP servers
- gRPC servers
- HTTP and gRPC clients
- structured logging
- OpenTelemetry tracing
- JWT and security middleware
- `viper`-based configuration loading

## Main modules

- [config](/e:/Proyects/Practices/QuicksGo/config/README.md): server/client bootstrap plus configuration and tracing utilities
- [logger](/e:/Proyects/Practices/QuicksGo/logger/README.md): structured logging, HTTP/gRPC middleware, and satellite traces
- [security](/e:/Proyects/Practices/QuicksGo/security/README.md): JWT, security middleware, and cryptographic helpers
- [cmd/qgo](/e:/Proyects/Practices/QuicksGo/cmd/README.md): CLI generator for new Gin and gRPC services

## How the dependencies fit together

A typical QuicksGo application flow looks like this:

1. `config` loads `application.yml` or `application.json` into `viper`
2. `config` initializes `logger`
3. `config` initializes OTEL tracing
4. `server/gin` or `server/grpc` starts the server
5. `security` consumes the same `viper` configuration
6. `client/http` and `client/grpc` reuse tracing and logging for outbound calls

## Configuration template

The complete framework template is available at:

- [application.yml](/e:/Proyects/Practices/QuicksGo/config/application.yml)
- [application.json](/e:/Proyects/Practices/QuicksGo/config/application.json)

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

## Observability

Services generated and bootstrapped with QuicksGo are already prepared for
observability with the OpenTelemetry SDK.

That includes support for:

- traces
- logs
- metrics

QuicksGo is also compatible with the OpenTelemetry Go Auto Instrumentation SDK
when your deployment strategy requires automatic instrumentation on top of the
framework-provided setup.

## HTTP server

To start a Gin server with QuicksGo, you typically use `config/server/gin`:

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

To start a gRPC server with QuicksGo, you typically use `config/server/grpc`:

```go
srv := serverGRPC.NewIConfig(nil, nil)

if err := srv.Register(func(r grpc.ServiceRegistrar) {
	pb.RegisterGreeterServer(r, greeterServer{})
}); err != nil {
	panic(err)
}

panic(srv.Serve())
```

## Recommended usage

If you are starting a new application with QuicksGo:

1. start from [application.yml](/e:/Proyects/Practices/QuicksGo/config/application.yml)
2. load configuration with `config/utilities.LoadEnv`
3. use `server/gin` or `server/grpc` as your bootstrap layer
4. define your routes or protobuf services
5. use `security` for JWT and endpoint protection
6. use `client/http` or `client/grpc` for traced outbound calls

You can also scaffold a new service with `qgo`:

```bash
go install github.com/PointerByte/QuicksGo/cmd/qgo@latest
qgo new gin
qgo new grpc
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
