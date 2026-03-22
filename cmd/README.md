# qgo

`qgo` is the QuicksGo CLI used to scaffold new Gin or gRPC services.

## Install

```bash
go install github.com/PointerByte/QuicksGo/cmd/qgo@latest
```

## Commands

Create a new Gin service:

```bash
qgo new gin
```

Create a new gRPC service:

```bash
qgo new grpc
```

The generator asks for:

- the Go module/package name
- the `app.name` value
- the config format: `yaml` or `json`

Restrictions:

- the module/package name cannot contain spaces or unsupported special characters
- `app.name` cannot contain spaces or special characters

Then it creates:

- `main.go`
- `application.yaml` or `application.json`
- `go.mod`
- a project folder named after `app.name` by default, unless `--dir` is provided

Finally it runs:

```bash
go mod init <your-module>
go mod tidy
```

That installs the required dependencies automatically.
