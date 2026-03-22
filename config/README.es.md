# QuicksGo Config

Biblioteca de configuración e infraestructura para servicios Go basada en:

- servidor HTTP con Gin
- servidor gRPC
- clientes HTTP y gRPC
- carga de configuración con `viper`
- tracing con OpenTelemetry
- integración con `logger` y `security`

## Instalación

```bash
go get github.com/PointerByte/QuicksGo/config
```

## Paquetes

- `server_Gin`: bootstrap del servidor HTTP con Gin, middlewares, rutas base y shutdown
- `server_gRPC`: bootstrap del servidor gRPC, interceptores, TLS/mTLS y shutdown por señal
- `clientHttp`: cliente HTTP genérico con tracing y deserialización
- `client_gRPC`: cliente gRPC con tracing, TLS y mTLS
- `utilities`: helpers compartidos, incluida la carga de `application.yml`/`application.json`
- `utilities/traces`: inicialización OTEL y middlewares/interceptores HTTP y gRPC
- `utilities/jobs`: jobs simples en background
- `proto`: contrato protobuf de ejemplo para pruebas y ejemplos ejecutables

## Configuración

La dependencia carga su configuración con `utilities.LoadEnv`, que sigue esta prioridad:

1. `application.yml`
2. `application.json`
3. `.env`
4. `.env.local`
5. variables de entorno con mapeo automático desde la estructura cargada en `viper`

Eso significa que si existe `application.yml`, ese archivo se usa primero y tiene prioridad sobre `application.json`.

### Formato recomendado: YAML

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
```

### Formato opcional: JSON

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
  }
}
```

### Variables de entorno

Los overrides se generan a partir de la ruta de cada propiedad. Ejemplos:

- `app.name` -> `APP_NAME`
- `server.port` -> `SERVER_PORT`
- `server.grpc.port` -> `SERVER_GRPC_PORT`
- `server.gin.rate.limit` -> `SERVER_GIN_RATE_LIMIT`

### Referencia de configuración

Los ejemplos `application.yml` y `application.json` incluyen las llaves más importantes que hoy consumen `config`, `logger` y `security`.

#### `app`

- `app.name`: nombre del servicio usado por el health endpoint, el logger y el nombre del recurso OTEL.
- `app.version`: versión del servicio reportada en health y en metadata de OpenTelemetry.

#### `server`

- `server.groups`: grupos de rutas Gin que `server_Gin.CreateApp()` crea automáticamente.
- `server.modeTest`: bandera auxiliar usada en pruebas para simplificar comportamiento en runtime.

#### `server.gin`

- `server.gin.port`: dirección de escucha del servidor HTTP.
- `server.gin.mode`: modo de Gin, normalmente `debug`, `release` o `test`.
- `server.gin.UseH2C`: habilita HTTP/2 cleartext en el handler de Gin.
- `server.gin.rate.limit`: límite de requests permitido por el middleware de rate limit.
- `server.gin.rate.burst`: burst permitido para ese rate limit.

#### `server.gin.LoggerWithConfig`

- `server.gin.LoggerWithConfig.enabled`: activa o desactiva el logger estructurado de requests HTTP.
- `server.gin.LoggerWithConfig.SkipPaths`: rutas que no deben registrarse en logs.
- `server.gin.LoggerWithConfig.SkipQueryString`: oculta el query string del path logueado cuando está activo.

#### `server.grpc`

- `server.grpc.port`: dirección de escucha del servidor gRPC cuando no se inyecta un listener ni una address manual.

#### `server.grpc.tls`

- `server.grpc.tls.enable`: habilita TLS en el servidor gRPC.
- `server.grpc.tls.certFile`: ruta del certificado del servidor.
- `server.grpc.tls.keyFile`: ruta de la llave privada del servidor.
- `server.grpc.tls.version`: versión mínima TLS, por ejemplo `tlsv12` o `tlsv13`.

#### `server.grpc.mtls`

- `server.grpc.mtls.enable`: habilita validación mTLS en el servidor gRPC.
- `server.grpc.mtls.clientCAFile`: archivo CA usado para validar certificados cliente.
- `server.grpc.mtls.clientAuth`: política de certificados cliente. Valores soportados: `request_client_cert`, `require_any_client_cert`, `verify_client_cert_if_given` y `require_and_verify_client_cert`.

#### `gin.autotls`

- `gin.autotls.enable`: habilita gestión automática de certificados mediante `autocert`.
- `gin.autotls.domain`: dominio permitido para certificados automáticos.
- `gin.autotls.dirCache`: directorio local de cache para `autocert`.
- `gin.autotls.version`: versión mínima TLS usada por la configuración auto TLS.

#### `client.grpc.tls`

- `client.grpc.tls.enable`: habilita TLS en el cliente gRPC saliente.
- `client.grpc.tls.caFile`: bundle CA usado para validar el certificado del servidor remoto.
- `client.grpc.tls.serverName`: nombre esperado del servidor durante la validación del certificado.
- `client.grpc.tls.version`: versión mínima TLS para el transporte cliente.
- `client.grpc.tls.insecureSkipVerify`: desactiva validación de certificados. Útil solo en escenarios controlados de desarrollo.

#### `client.grpc.mtls`

- `client.grpc.mtls.enable`: habilita mTLS en el cliente gRPC saliente.
- `client.grpc.mtls.certFile`: ruta del certificado cliente.
- `client.grpc.mtls.keyFile`: ruta de la llave privada cliente.

#### `logger`

- `logger.dir`: directorio usado por `builder.InitLogger` para crear el archivo de log.
- `logger.modeTest`: desactiva salida del logger durante tests cuando está habilitado.
- `logger.level`: nivel mínimo de log, normalmente `debug`, `info`, `warn` o `error`.
- `logger.ignoredHeaders`: headers que no deben aparecer en logs estructurados.
- `logger.formatter`: formato de salida, por ejemplo `json` o `text`, según la configuración del logger.
- `logger.formatDate`: formato de fecha usado por el formatter.

#### `logger.rotate`

- `logger.rotate.enable`: activa rotación de archivos con `lumberjack`.
- `logger.rotate.maxSize`: tamaño máximo en MB antes de rotar.
- `logger.rotate.maxBackups`: número máximo de archivos rotados a conservar.
- `logger.rotate.maxAge`: edad máxima en días de los archivos rotados.
- `logger.rotate.compress`: comprime archivos rotados cuando está activo.

#### `traces`

- `traces.SkipPaths`: paths HTTP excluidos del middleware OpenTelemetry de Gin.

#### `jwt`

- `jwt.enable`: activa o desactiva la validación JWT en middleware.
- `jwt.algorithm`: algoritmo de firma JWT usado por `security`, por ejemplo `HS256`, `RS256`, `PS256` o `EdDSA`.

#### `jwt.hmac`

- `jwt.hmac.secret`: secreto compartido usado por algoritmos JWT basados en HMAC.

#### `jwt.rsa`

- `jwt.rsa.private_key`: llave privada RSA en texto para firmar.
- `jwt.rsa.public_key`: llave pública RSA en texto para validar.

#### `jwt.eddsa`

- `jwt.eddsa.private_key`: llave privada Ed25519 en texto para firmar.
- `jwt.eddsa.public_key`: llave pública Ed25519 en texto para validar.

## Servidor HTTP con Gin

`server_Gin.CreateApp()`:

- carga configuración con `utilities.LoadEnv`
- inicializa `logger`
- inicializa OpenTelemetry
- crea el `gin.Engine`
- registra middlewares compartidos
- crea grupos desde `server.groups`
- registra `/health` en cada grupo

Uso básico:

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

Claves principales:

- `server.port`
- `server.groups`
- `server.gin.mode`
- `server.gin.UseH2C`
- `server.gin.rate.limit`
- `server.gin.rate.burst`

## Servidor gRPC

`server_gRPC`:

- carga configuración con `utilities.LoadEnv(".")` al ejecutar `Serve()`
- resuelve `server.grpc.port` desde `viper`
- integra interceptores de `logger` y `traces`
- soporta TLS y mTLS
- escucha `Ctrl+C` / `SIGTERM` para hacer `GracefulStop()`

Uso básico:

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

Claves principales:

- `server.grpc.port`
- `server.grpc.tls.enable`
- `server.grpc.tls.certFile`
- `server.grpc.tls.keyFile`
- `server.grpc.tls.version`
- `server.grpc.mtls.enable`
- `server.grpc.mtls.clientCAFile`
- `server.grpc.mtls.clientAuth`

## Cliente HTTP

`clientHttp` expone un cliente REST genérico con trazabilidad de request/response.

Uso básico:

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

## Cliente gRPC

`client_gRPC` envuelve `grpc.ClientConn` y puede:

- crear clientes protobuf con `BuildClient`
- resolver TLS/mTLS desde `viper`
- trazar metadata, request y response con `logger`

Uso básico:

```go
cli := client_gRPC.NewIClient(nil, nil)

greeter, err := client_gRPC.BuildClient(cli, pb.NewGreeterClient)
if err != nil {
	panic(err)
}
```

Claves principales:

- `client.grpc.tls.enable`
- `client.grpc.tls.caFile`
- `client.grpc.tls.serverName`
- `client.grpc.tls.version`
- `client.grpc.tls.insecureSkipVerify`
- `client.grpc.mtls.enable`
- `client.grpc.mtls.certFile`
- `client.grpc.mtls.keyFile`

## Ejemplo ejecutable

El proyecto incluye un ejemplo en [main.go](/e:/Proyects/Practices/QuicksGoV2t/config/main.go).

Ejecutar ejemplo Gin:

```powershell
$env:QUICKSGO_EXAMPLE_SERVER="gin"
go run .
```

Ejecutar ejemplo gRPC:

```powershell
$env:QUICKSGO_EXAMPLE_SERVER="grpc"
go run .
```

## Protobuf

Comandos necesarios:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
protoc --go_out=. --go-grpc_out=. proto/methods.proto
```

## Pruebas

```bash
go test ./...
```

## Comandos útiles

### Actualizar dependencias

```bash
go get -u ./...
```

### Limpiar cache de build, tests y módulos

```bash
go clean -cache -testcache -modcache
```

### Ejecutar pruebas con coverage

```bash
go test -cover -covermode=atomic -coverprofile="coverage.out" ./...
```

### Generar reporte HTML de coverage

```bash
go tool cover -html="coverage.out" -o "coverage.html"
```

### Mostrar coverage por función

```bash
go tool cover -func="coverage.out"
```
