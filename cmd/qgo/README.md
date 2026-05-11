# qgo

`qgo` is the QuicksGo service scaffolding CLI. It creates starter projects for
Gin HTTP services or gRPC services, writes the initial application
configuration, initializes `go.mod`, and runs `go mod tidy`.

## Install

```bash
go install github.com/PointerByte/QuicksGo/cmd/qgo@latest
```

## Commands

Create a Gin service:

```bash
qgo new gin
```

Create a gRPC service:

```bash
qgo new grpc
```

Both commands support interactive prompts and non-interactive flags.

## Non-Interactive Usage

```bash
qgo new gin \
  --module github.com/acme/orders-api \
  --app-name orders-api \
  --config-format yaml \
  --dir ./orders-api
```

```bash
qgo new grpc \
  --module github.com/acme/payments-rpc \
  --app-name payments-rpc \
  --config-format json \
  --dir ./payments-rpc
```

## Flags

| Flag | Short | Description |
| --- | --- | --- |
| `--module` | `-m` | Go module path used in `go mod init` |
| `--app-name` | `-a` | Value written to `app.name` in the generated config |
| `--config-format` | `-c` | `yaml` or `json`; interactive mode defaults to `yaml` |
| `--dir` | `-d` | Output directory; defaults to `app.name` |

If a required flag is omitted, `qgo` asks for it interactively.

## Validation

- module path accepts letters, numbers, `.`, `_`, `/`, and `-`
- `app.name` accepts letters, numbers, `_`, and `-`
- spaces are rejected in both values
- config format must be `yaml` or `json`
- the output directory must not already exist

## Generated Files

For both service types, `qgo` creates:

- `main.go`
- `application.yaml` or `application.json`
- `go.mod`, created by `go mod init <module>`
- `go.sum`, when dependency resolution needs it

After writing the files, it runs:

```bash
go mod init <module>
go mod tidy
```

`go mod tidy` downloads the QuicksGo dependencies required by the generated
service, so network access may be needed.

## Gin Scaffold

The Gin scaffold creates a `main.go` that:

- calls `serverGin.CreateApp()`
- retrieves the `/api/v1` route group with `serverGin.GetRoute("/api/v1")`
- registers `GET /hello`
- starts the server with `serverGin.Start(srv)`

The generated application config includes Gin server settings, logging,
OpenTelemetry flags, JWT defaults, HTTP/gRPC client TLS settings, and
TLS/mTLS placeholders for server certificates.

Run the generated service from its output directory:

```bash
go run .
```

Default HTTP port:

```text
:8080
```

## gRPC Scaffold

The gRPC scaffold creates a minimal `main.go` that:

- calls `serverGRPC.NewIConfig(nil, nil)`
- starts the server with `srv.Serve()`

The generated application config includes gRPC server settings, logging,
OpenTelemetry flags, JWT defaults, gRPC client TLS settings, and TLS/mTLS
placeholders.

Run the generated service from its output directory:

```bash
go run .
```

Default gRPC port:

```text
:50051
```

## Examples

Create a YAML Gin service in the default output directory:

```bash
qgo new gin -m github.com/acme/orders-api -a orders-api -c yaml
```

Create a JSON gRPC service in an explicit directory:

```bash
qgo new grpc \
  -m github.com/acme/payments-rpc \
  -a payments-rpc \
  -c json \
  -d ./services/payments-rpc
```

If the command succeeds, it prints:

```text
Service created in <absolute-output-directory>
```

## Development

From the `cmd/qgo` module directory:

```bash
go test ./...
go test -cover -covermode=atomic -coverprofile=coverage.out ./...
```
