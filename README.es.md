# QuicksGo

QuicksGo es un toolkit modular para arrancar aplicaciones de servicios en Go
con convenciones compartidas para configuracion, transporte HTTP y gRPC,
logging, OpenTelemetry, seguridad JWT, jobs en background y utilidades de
runtime.

El repositorio esta organizado como un workspace de Go. El modulo raiz contiene
los paquetes de bootstrap en `config` y `tools`; `logger`, `security`,
`encrypt` y las CLIs son modulos separados que tambien pueden consumirse de
forma independiente.

La documentacion en ingles esta disponible en [README.md](./README.md).

## Que Incluye

- bootstrap de servidor HTTP con Gin y middlewares comunes
- bootstrap de servidor gRPC con interceptores unary y stream
- clientes HTTP y gRPC con hooks de tracing
- logging estructurado mediante el modulo `logger`
- trazas, metricas y helpers de instrumentacion con OpenTelemetry
- middleware JWT mediante el modulo `security`
- criptografia local y backends orientados a KMS mediante `encrypt`
- jobs de intervalo fijo y utilidades simples de workers
- helpers de ECS y Kubernetes para descubrir hosts de refresh
- CLIs para generar servicios y certificados

## Modulos

- [modulo raiz](./go.mod): `github.com/PointerByte/QuicksGo`
- [logger](./logger/README.es.md): logging estructurado y middlewares HTTP/gRPC
- [security](./security/README.es.md): servicios JWT y middleware de seguridad para Gin
- [encrypt](./encrypt/README.es.md): cifrado simetrico, hashes, RSA, firmas y backends orientados a KMS
- [cmd/qgo](./cmd/qgo/README.es.md): CLI para generar servicios Gin y gRPC
- [cmd/go-openssl](./cmd/go-openssl/README.es.md): CLI para generar y leer certificados y llaves PEM

## Instalacion

Instalar el modulo raiz:

```bash
go get github.com/PointerByte/QuicksGo
```

O instalar solo los modulos que necesites:

```bash
go get github.com/PointerByte/QuicksGo/logger
go get github.com/PointerByte/QuicksGo/security
go get github.com/PointerByte/QuicksGo/encrypt
```

Instalar las CLIs:

```bash
go install github.com/PointerByte/QuicksGo/cmd/qgo@latest
go install github.com/PointerByte/QuicksGo/cmd/go-openssl@latest
```

## Configuracion

`config/utilities.LoadEnv(prefixPath)` carga configuracion en `viper` desde el
directorio indicado. Busca estos archivos en orden:

1. `application.yml`
2. `application.yaml`
3. `application.json`

Despues de leer el archivo de aplicacion, mezcla `.env` y `.env.local` desde el
directorio de trabajo actual y habilita variables de entorno. Los overrides por
entorno se generan desde la ruta de claves ya existente en la configuracion.

Ejemplos:

- `app.name` -> `APP_NAME`
- `server.gin.port` -> `SERVER_GIN_PORT`
- `server.gin.groups` -> `SERVER_GIN_GROUPS`
- `server.grpc.port` -> `SERVER_GRPC_PORT`
- `client.http.timeout` -> `CLIENT_HTTP_TIMEOUT`
- `client.grpc.tls.serverName` -> `CLIENT_GRPC_TLS_SERVERNAME`
- `jwt.hmac.secret` -> `JWT_HMAC_SECRET`

YAML es el formato recomendado para aplicaciones nuevas.

### YAML Minimo

```yaml
app:
  name: quicksgo-service
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
  algorithm: HS256
  hmac:
    secret: change-me
```

## Claves Principales

### Aplicacion

- `app.name`: nombre del servicio usado por health endpoints y metadata OpenTelemetry
- `app.version`: version del servicio reportada en health endpoints y telemetria
- `server.modeTest`: deshabilita comportamientos de runtime como ejecucion de jobs durante pruebas

### Servidor Gin

- `server.gin.port`: direccion de escucha HTTP
- `server.gin.mode`: modo de Gin, por ejemplo `debug`, `release` o `test`
- `server.gin.groups`: grupos de rutas creados por `config/server/gin`
- `server.gin.UseH2C`: habilita soporte HTTP/2 cleartext
- `server.gin.rate.limit`: tasa del limiter incorporado; `0` lo deshabilita
- `server.gin.rate.burst`: burst del limiter
- `server.gin.LoggerWithConfig.enabled`: habilita logs estructurados de requests HTTP
- `server.gin.LoggerWithConfig.SkipPaths`: rutas omitidas por el logger de requests
- `server.gin.LoggerWithConfig.SkipQueryString`: oculta query strings en paths logueados

Gin tambien soporta `server.gin.autotls.*`, `server.gin.tls.*` y
`server.gin.mtls.*` para TLS automatico, TLS explicito y mTLS.

Versiones TLS soportadas: `tlsv10`, `tlsv11`, `tlsv12` y `tlsv13`.
Valores de client-auth soportados: `no_client_cert`, `request_client_cert`,
`require_any_client_cert`, `verify_client_cert_if_given` y
`require_and_verify_client_cert`.

### Servidor gRPC

- `server.grpc.port`: direccion de escucha gRPC
- `server.grpc.tls.enable`: habilita TLS en el servidor gRPC
- `server.grpc.tls.certFile`: ruta del certificado del servidor
- `server.grpc.tls.keyFile`: ruta de la llave privada del servidor
- `server.grpc.tls.version`: version minima TLS
- `server.grpc.mtls.enable`: habilita validacion mTLS
- `server.grpc.mtls.clientCAFile`: archivo CA para validar certificados cliente
- `server.grpc.mtls.clientAuth`: politica de certificados cliente

### Clientes

- `client.http.timeout`: timeout por defecto para clientes HTTP configurados
- `client.http.tls.*`: configuracion TLS HTTP saliente
- `client.http.mtls.*`: certificado cliente mTLS HTTP saliente
- `client.grpc.tls.*`: configuracion TLS gRPC saliente
- `client.grpc.mtls.*`: certificado cliente mTLS gRPC saliente

### Logger, Traces y JWT

- `logger.dir`: directorio de archivos de log
- `logger.level`: nivel minimo como `debug`, `info`, `warn` o `error`
- `logger.ignoredHeaders`: headers removidos de logs estructurados
- `logger.formatter`: formato de salida, normalmente `json` o `text`
- `logger.rotate.*`: configuracion de rotacion de archivos
- `traces.enable`: habilita inicializacion de OpenTelemetry
- `traces.SkipPaths`: rutas HTTP omitidas por el middleware OpenTelemetry de Gin
- `jwt.enable`: habilita validacion JWT en middleware
- `jwt.transport`: origen del token, normalmente `header` o `cookie`
- `jwt.cookie.name`: nombre de cookie cuando `jwt.transport` es `cookie`
- `jwt.algorithm`: algoritmo de firma como `HS256`, `RS256`, `PS256` o `EdDSA`
- `jwt.hmac.secret`, `jwt.rsa.*`, `jwt.eddsa.*`: configuracion de llaves de firma

## Servidor HTTP

`config/server/gin.CreateApp()` carga configuracion, inicializa logger y
OpenTelemetry, crea el `gin.Engine` compartido, registra middlewares comunes,
crea grupos desde `server.gin.groups` y registra `/health` y `/refresh` en cada
grupo.

```go
package main

import (
	"log"

	serverGin "github.com/PointerByte/QuicksGo/config/server/gin"
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

`GET /refresh` reinicia jobs registrados a nivel de paquete, ejecuta callbacks
registrados con `SetFunctionsRefresh(...)` y puede propagarse a peers
registrados con `SetHostsRefresh(...)`.

## Servidor gRPC

`config/server/grpc` carga configuracion en `Serve()`, resuelve
`server.grpc.port`, agrega interceptores de logging y OpenTelemetry, soporta
TLS y mTLS, y hace apagado graceful ante senales del proceso.

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

## Clientes

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

Usa `NewGenericRestFromConfig()` o las claves TLS del cliente gRPC cuando
quieras construir transportes desde `viper`.

## Trabajo En Background

`tools/jobs` ofrece jobs en proceso con intervalo fijo. Los jobs arrancan
cuando se ejecuta `jobs.StartJobs()`; `config/server/gin.Start(...)` lo llama
automaticamente. Cuando `server.modeTest=true`, los jobs no arrancan.

```go
func registerJobs() {
	timeout := 30 * time.Minute

	jobs.Job(func() {
		refreshCache()
	}, time.Minute, &timeout)
}
```

`tools/workers` ofrece un loop simple de workers acotados mediante
`SetWorkersLimit`, `RunWorkers`, `AddTask`, `StopWorkers` y `RestartWorkers`.

## Ejemplos Ejecutables

El modulo raiz incluye ejemplos ejecutables en [main.go](./main.go).
Usa un archivo de aplicacion como el YAML minimo anterior antes de arrancar el
ejemplo Gin; espera `/api/v1` en `server.gin.groups`.

Ejecutar el ejemplo Gin:

```bash
QUICKSGO_EXAMPLE_SERVER=gin go run .
```

Ejecutar el ejemplo gRPC:

```bash
QUICKSGO_EXAMPLE_SERVER=grpc go run .
```

## Desarrollo

El workspace usa Go `1.25.0`.

Ejecutar pruebas del modulo raiz:

```bash
go test ./...
```

Ejecutar pruebas de un modulo del workspace:

```bash
cd logger
go test ./...
```

Generar archivos protobuf despues de editar `config/proto/methods.proto`:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
protoc --go_out=. --go-grpc_out=. config/proto/methods.proto
```

Coverage para el modulo actual:

```bash
go test -cover -covermode=atomic -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html
```
