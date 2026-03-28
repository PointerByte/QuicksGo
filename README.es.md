# QuicksGo

QuicksGo es un framework modular para arrancar servicios Go con un enfoque compartido de configuracion, transporte, observabilidad y seguridad.

Ayuda a construir aplicaciones mas rapido con:

- servidor HTTP con Gin
- servidor gRPC
- clientes HTTP y gRPC
- logging estructurado
- tracing con OpenTelemetry
- middlewares JWT y de seguridad
- carga de configuracion con `viper`
- jobs simples dentro del proceso

## Modulos principales

- [config](/e:/Proyects/Practices/QuicksGo/config): bootstrap de servidores y clientes, carga de configuracion, tracing y jobs
- [logger](/e:/Proyects/Practices/QuicksGo/logger/README.es.md): logging estructurado y middlewares HTTP/gRPC
- [security](/e:/Proyects/Practices/QuicksGo/security/README.es.md): JWT, middlewares de seguridad y utilidades criptograficas
- [cmd/qgo](/e:/Proyects/Practices/QuicksGo/cmd/qgo/README.es.md): CLI para generar servicios nuevos con Gin y gRPC

## Como encajan las piezas

Flujo tipico de una aplicacion QuicksGo:

1. `config/utilities.LoadEnv` carga `application.yml` o `application.json` en `viper`
2. `config` inicializa `logger`
3. `config` inicializa tracing con OpenTelemetry
4. `config/server/gin` o `config/server/grpc` arrancan el servidor
5. `security` reutiliza la misma configuracion compartida en `viper`
6. `config/client/http` y `config/client/grpc` reutilizan tracing y logging para llamadas salientes

## Instalacion

Instalar el modulo raiz:

```bash
go get github.com/PointerByte/QuicksGo
```

O instalar solo el modulo que necesites:

```bash
go get github.com/PointerByte/QuicksGo/config
go get github.com/PointerByte/QuicksGo/logger
go get github.com/PointerByte/QuicksGo/security
```

## Modelo de configuracion

Las plantillas completas del framework estan en:

- [config/application.yml](/e:/Proyects/Practices/QuicksGo/config/application.yml)
- [config/application.json](/e:/Proyects/Practices/QuicksGo/config/application.json)

Prioridad de carga soportada:

1. `application.yml`
2. `application.json`
3. `.env`
4. `.env.local`
5. variables de entorno

YAML es el formato recomendado para aplicaciones nuevas.

### Ejemplo YAML

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

### Mapeo de variables de entorno

Los overrides se generan desde la ruta de cada clave. Ejemplos:

- `app.name` -> `APP_NAME`
- `server.port` -> `SERVER_PORT`
- `server.gin.port` -> `SERVER_GIN_PORT`
- `server.grpc.port` -> `SERVER_GRPC_PORT`
- `client.grpc.tls.serverName` -> `CLIENT_GRPC_TLS_SERVERNAME`
- `jwt.hmac.secret` -> `JWT_HMAC_SECRET`

## Referencia de configuracion

Las plantillas de ejemplo incluyen las claves mas usadas hoy por `config`, `logger` y `security`.

### `app`

- `app.name`: nombre del servicio usado por health endpoints, metadata del logger y nombre del recurso OTEL
- `app.version`: version del servicio reportada en health endpoints y metadata OTEL

### `server`

- `server.groups`: grupos de rutas Gin creados automaticamente por `config/server/gin`
- `server.modeTest`: bandera auxiliar usada en pruebas para simplificar el runtime

### `server.gin`

- `server.gin.port`: direccion de escucha HTTP
- `server.gin.mode`: modo de Gin como `debug`, `release` o `test`
- `server.gin.UseH2C`: habilita soporte HTTP/2 cleartext
- `server.gin.rate.limit`: limite de requests del rate limiter incorporado
- `server.gin.rate.burst`: burst del rate limiter

### `server.gin.LoggerWithConfig`

- `server.gin.LoggerWithConfig.enabled`: habilita el logger estructurado de requests HTTP
- `server.gin.LoggerWithConfig.SkipPaths`: rutas excluidas del middleware de logging
- `server.gin.LoggerWithConfig.SkipQueryString`: oculta el query string del path logueado

### `server.grpc`

- `server.grpc.port`: direccion de escucha gRPC

### `server.grpc.tls`

- `server.grpc.tls.enable`: habilita TLS en el servidor gRPC
- `server.grpc.tls.certFile`: ruta del certificado del servidor
- `server.grpc.tls.keyFile`: ruta de la llave privada del servidor
- `server.grpc.tls.version`: version minima TLS como `tlsv12` o `tlsv13`

### `server.grpc.mtls`

- `server.grpc.mtls.enable`: habilita validacion mTLS en el servidor gRPC
- `server.grpc.mtls.clientCAFile`: archivo CA para validar certificados cliente
- `server.grpc.mtls.clientAuth`: politica de certificados cliente

Valores soportados para `server.grpc.mtls.clientAuth`:

- `request_client_cert`
- `require_any_client_cert`
- `verify_client_cert_if_given`
- `require_and_verify_client_cert`

### `gin.autotls`

- `gin.autotls.enable`: habilita gestion automatica de certificados con `autocert`
- `gin.autotls.domain`: dominio permitido para certificados administrados
- `gin.autotls.dirCache`: directorio local de cache para `autocert`
- `gin.autotls.version`: version minima TLS para auto TLS

### `client.grpc.tls`

- `client.grpc.tls.enable`: habilita TLS en el cliente gRPC saliente
- `client.grpc.tls.caFile`: bundle CA usado para validar el certificado remoto
- `client.grpc.tls.serverName`: nombre esperado del servidor durante la validacion del certificado
- `client.grpc.tls.version`: version minima TLS para el transporte cliente
- `client.grpc.tls.insecureSkipVerify`: desactiva la validacion de certificados y solo deberia usarse en desarrollo controlado

### `client.grpc.mtls`

- `client.grpc.mtls.enable`: habilita mTLS en el cliente gRPC saliente
- `client.grpc.mtls.certFile`: ruta del certificado cliente
- `client.grpc.mtls.keyFile`: ruta de la llave privada cliente

### `logger`

- `logger.dir`: directorio usado para crear el archivo de log
- `logger.modeTest`: desactiva la salida del logger durante tests
- `logger.level`: nivel minimo de log como `debug`, `info`, `warn` o `error`
- `logger.ignoredHeaders`: headers que no deben aparecer en logs estructurados
- `logger.formatter`: formato de salida como `json` o `text`
- `logger.formatDate`: formato de fecha usado por el formatter

### `logger.rotate`

- `logger.rotate.enable`: habilita rotacion de archivos con `lumberjack`
- `logger.rotate.maxSize`: tamano maximo en MB antes de rotar
- `logger.rotate.maxBackups`: numero maximo de archivos rotados a conservar
- `logger.rotate.maxAge`: edad maxima en dias de los archivos rotados
- `logger.rotate.compress`: comprime archivos rotados cuando esta habilitado

### `traces`

- `traces.SkipPaths`: paths HTTP excluidos del middleware OpenTelemetry de Gin

### `jwt`

- `jwt.enable`: habilita o deshabilita la validacion JWT en middleware
- `jwt.transport`: origen del JWT usado por Gin. Valores soportados: `header` y `cookie`
- `jwt.algorithm`: algoritmo de firma como `HS256`, `RS256`, `PS256` o `EdDSA`

### `jwt.cookie`

- `jwt.cookie.name`: nombre de la cookie usada cuando `jwt.transport` es `cookie`

### `jwt.hmac`

- `jwt.hmac.secret`: secreto compartido usado por algoritmos JWT basados en HMAC

### `jwt.rsa`

- `jwt.rsa.private_key`: llave privada RSA en texto para firmar
- `jwt.rsa.public_key`: llave publica RSA en texto para validar

### `jwt.eddsa`

- `jwt.eddsa.private_key`: llave privada Ed25519 en texto para firmar
- `jwt.eddsa.public_key`: llave publica Ed25519 en texto para validar

## Observabilidad

Los servicios arrancados con QuicksGo ya quedan preparados para observabilidad basada en OpenTelemetry.

Eso incluye:

- trazas
- logs
- metricas

QuicksGo tambien es compatible con OpenTelemetry Go Auto Instrumentation cuando tu despliegue necesita instrumentacion automatica encima de la configuracion del framework.

## Servidor HTTP

`config/server/gin.CreateApp()`:

- carga configuracion
- inicializa logger
- inicializa OpenTelemetry
- crea el `gin.Engine`
- registra middlewares compartidos
- aplica middleware JWT cuando esta configurado
- crea grupos desde `server.groups`
- registra `/health` y `/refresh` por cada grupo

Uso basico:

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

### Endpoint refresh en Gin

Cada grupo de rutas creado desde `server.groups` tambien recibe un endpoint `GET /refresh`.

Esta pensado para:

- recargar cache o estado en memoria
- reiniciar jobs de background entre instancias
- propagar eventos de refresh a peers registrados con `SetHostsRefresh(...)`

Si necesitas logica local antes del fan-out, registra callbacks con `SetFunctionsRefresh(...)`.

## Servidor gRPC

`config/server/grpc`:

- carga configuracion cuando se ejecuta `Serve()`
- resuelve `server.grpc.port` desde `viper`
- integra interceptores de logger y tracing
- soporta TLS y mTLS
- escucha senales de apagado y ejecuta `GracefulStop()`

Uso basico:

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

### Endpoint refresh en gRPC

`config/server/grpc` expone un RPC administrativo interno de refresh en `"/quicksgo.admin/Refresh"`.

Esta pensado para:

- refrescar estado local en todos los nodos
- reiniciar jobs programados de forma coordinada
- propagar eventos internos de recarga entre instancias gRPC

## Cliente HTTP

`config/client/http` expone un cliente REST generico con trazabilidad de request y response.

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

## Cliente gRPC

`config/client/grpc` envuelve `grpc.ClientConn` y puede:

- construir clientes protobuf con `BuildClient`
- resolver TLS y mTLS desde `viper`
- trazar metadata, request y response con `logger`

```go
cli := clientGRPC.NewIClient(nil)

greeter, err := clientGRPC.BuildClient(cli, pb.NewGreeterClient)
if err != nil {
	panic(err)
}
```

## Jobs en background

`config/utilities/jobs` ofrece tareas recurrentes simples dentro del mismo proceso.

Entradas comunes:

- `jobs.Job(...)`
- `jobs.CronJob(...)`
- `jobs.StartJobs()`
- `jobs.RestartJobs()`
- `jobs.StopAllJobs(...)`
- `jobs.CheckStatusJobs()`

Comportamiento importante:

- los jobs arrancan solo despues de `StartJobs()`
- `config/server/gin.Start(...)` ya llama `jobs.StartJobs()`
- los jobs registrados despues de `StartJobs()` se lanzan de inmediato
- cuando `server.modeTest=true`, los jobs no corren

Ejemplo:

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

## Ejemplo ejecutable

El modulo `config` incluye un ejemplo ejecutable en [config/main.go](/e:/Proyects/Practices/QuicksGo/config/main.go).

Ejecutar el ejemplo Gin:

```powershell
$env:QUICKSGO_EXAMPLE_SERVER="gin"
go run ./config
```

Ejecutar el ejemplo gRPC:

```powershell
$env:QUICKSGO_EXAMPLE_SERVER="grpc"
go run ./config
```

## Uso recomendado

Si vas a arrancar una aplicacion nueva con QuicksGo:

1. usa como base [config/application.yml](/e:/Proyects/Practices/QuicksGo/config/application.yml)
2. carga configuracion con `config/utilities.LoadEnv`
3. usa `config/server/gin` o `config/server/grpc` como bootstrap
4. define tus rutas o servicios protobuf
5. usa `security` para JWT y proteccion de endpoints
6. usa `config/client/http` o `config/client/grpc` para llamadas salientes con tracing
7. usa `config/utilities/jobs` si necesitas jobs recurrentes livianos

Tambien puedes generar un servicio nuevo con `qgo`:

```bash
go install github.com/PointerByte/QuicksGo/cmd/qgo@latest
qgo new gin
qgo new grpc
```

## Protobuf

Comandos requeridos:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
protoc --go_out=. --go-grpc_out=. config/proto/methods.proto
```

## Pruebas

```bash
go test ./...
```

## Comandos utiles

### Actualizar dependencias

```bash
go get -u ./...
```

### Limpiar cache de build, tests y modulos

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

### Mostrar coverage por funcion

```bash
go tool cover -func="coverage.out"
```
