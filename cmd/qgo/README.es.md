# qgo

`qgo` es la CLI de scaffolding de servicios QuicksGo. Crea proyectos iniciales
para servicios HTTP con Gin o servicios gRPC, escribe la configuracion inicial,
inicializa `go.mod` y ejecuta `go mod tidy`.

## Instalacion

```bash
go install github.com/PointerByte/QuicksGo/cmd/qgo@latest
```

## Comandos

Crear un servicio Gin:

```bash
qgo new gin
```

Crear un servicio gRPC:

```bash
qgo new grpc
```

Ambos comandos soportan prompts interactivos y flags no interactivos.

## Uso No Interactivo

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

| Flag | Short | Descripcion |
| --- | --- | --- |
| `--module` | `-m` | ruta del modulo Go usada en `go mod init` |
| `--app-name` | `-a` | valor escrito en `app.name` dentro de la configuracion generada |
| `--config-format` | `-c` | `yaml` o `json`; en modo interactivo el default es `yaml` |
| `--dir` | `-d` | directorio de salida; por defecto usa `app.name` |

Si omites un flag requerido, `qgo` lo pregunta de forma interactiva.

## Validacion

- la ruta del modulo acepta letras, numeros, `.`, `_`, `/` y `-`
- `app.name` acepta letras, numeros, `_` y `-`
- los espacios se rechazan en ambos valores
- el formato de configuracion debe ser `yaml` o `json`
- el directorio de salida no debe existir previamente

## Archivos Generados

Para ambos tipos de servicio, `qgo` crea:

- `main.go`
- `application.yaml` o `application.json`
- `go.mod`, creado por `go mod init <modulo>`
- `go.sum`, cuando la resolucion de dependencias lo necesita

Despues de escribir los archivos, ejecuta:

```bash
go mod init <modulo>
go mod tidy
```

`go mod tidy` descarga las dependencias QuicksGo necesarias para el servicio
generado, asi que puede requerir acceso a red.

## Scaffold Gin

El scaffold Gin crea un `main.go` que:

- llama `serverGin.CreateApp()`
- obtiene el grupo `/api/v1` con `serverGin.GetRoute("/api/v1")`
- registra `GET /hello`
- inicia el servidor con `serverGin.Start(srv)`

La configuracion generada incluye settings del servidor Gin, logging, flags de
OpenTelemetry, defaults JWT, configuracion TLS para clientes HTTP/gRPC y
placeholders TLS/mTLS para certificados del servidor.

Ejecutar el servicio generado desde su directorio:

```bash
go run .
```

Puerto HTTP por defecto:

```text
:8080
```

## Scaffold gRPC

El scaffold gRPC crea un `main.go` minimo que:

- llama `serverGRPC.NewIConfig(nil, nil)`
- inicia el servidor con `srv.Serve()`

La configuracion generada incluye settings del servidor gRPC, logging, flags de
OpenTelemetry, defaults JWT, configuracion TLS del cliente gRPC y placeholders
TLS/mTLS.

Ejecutar el servicio generado desde su directorio:

```bash
go run .
```

Puerto gRPC por defecto:

```text
:50051
```

## Ejemplos

Crear un servicio Gin YAML en el directorio default:

```bash
qgo new gin -m github.com/acme/orders-api -a orders-api -c yaml
```

Crear un servicio gRPC JSON en un directorio explicito:

```bash
qgo new grpc \
  -m github.com/acme/payments-rpc \
  -a payments-rpc \
  -c json \
  -d ./services/payments-rpc
```

Si el comando termina bien, imprime:

```text
Service created in <directorio-absoluto-de-salida>
```

## Desarrollo

Desde el directorio del modulo `cmd/qgo`:

```bash
go test ./...
go test -cover -covermode=atomic -coverprofile=coverage.out ./...
```
