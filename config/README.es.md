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

- `server/gin`: bootstrap del servidor HTTP con Gin, middlewares, rutas base y shutdown
- `server/grpc`: bootstrap del servidor gRPC, interceptores, TLS/mTLS y shutdown por señal
- `client/http`: cliente HTTP genérico con tracing y deserialización
- `client/grpc`: cliente gRPC con tracing, TLS y mTLS
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

jwt:
  enable: true
  transport: cookie
  cookie:
    name: session_token
  algorithm: HS256
  hmac:
    secret: change-me
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

- `server.groups`: grupos de rutas Gin que `server/gin.CreateApp()` crea automáticamente.
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
- `jwt.transport`: define desde dónde `server/gin.CreateApp()` lee el JWT. Valores soportados: `header` y `cookie`.
- `jwt.algorithm`: algoritmo de firma JWT usado por `security`, por ejemplo `HS256`, `RS256`, `PS256` o `EdDSA`.

#### `jwt.cookie`

- `jwt.cookie.name`: nombre de la cookie usada cuando `jwt.transport` vale `cookie`. Si no se define, `security` usa `access_token`.

#### `jwt.hmac`

- `jwt.hmac.secret`: secreto compartido usado por algoritmos JWT basados en HMAC.

#### `jwt.rsa`

- `jwt.rsa.private_key`: llave privada RSA en texto para firmar.
- `jwt.rsa.public_key`: llave pública RSA en texto para validar.

#### `jwt.eddsa`

- `jwt.eddsa.private_key`: llave privada Ed25519 en texto para firmar.
- `jwt.eddsa.public_key`: llave pública Ed25519 en texto para validar.

## Servidor HTTP con Gin

`server/gin.CreateApp()`:

- carga configuración con `utilities.LoadEnv`
- inicializa `logger`
- inicializa OpenTelemetry
- crea el `gin.Engine`
- registra middlewares compartidos
- aplica `RequireJWT` cuando `jwt.transport=header` o `RequireJWTCookie` cuando `jwt.transport=cookie`
- crea grupos desde `server.groups`
- registra `/health` y `/refresh` en cada grupo

Uso básico:

```go
package main

import (
	"log"

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

Claves principales:

- `server.port`
- `server.groups`
- `server.gin.mode`
- `server.gin.UseH2C`
- `server.gin.rate.limit`
- `server.gin.rate.burst`
- `jwt.enable`
- `jwt.transport`
- `jwt.cookie.name`

### Endpoint refresh en Gin

Cada grupo de rutas creado desde `server.groups` también recibe un endpoint
`GET /refresh`. Por ejemplo, si el grupo es `/api/v1`, el endpoint queda como
`/api/v1/refresh`.

Qué hace:

- llama `jobs.RestartJobs()` a través de la variable función interna `restartJobs`
- propaga la petición de refresh a los hosts registrados con `SetHostsRefresh(...)`
- conserva los headers entrantes y agrega `broadcast-refresh=true`
- evita loops infinitos respondiendo de inmediato cuando la petición ya trae
  ese marcador de broadcast

Uso típico:

- recargar cache o estado en memoria después de cambios de configuración
- reprogramar jobs de background a nivel paquete en múltiples instancias
- mantener sincronizado un cluster de servicios Gin mediante fan-out simple

Si necesitas lógica local antes de la propagación, registra callbacks con
`SetFunctionsRefresh(...)`.

## Servidor gRPC

`server/grpc`:

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

Claves principales:

- `server.grpc.port`
- `server.grpc.tls.enable`
- `server.grpc.tls.certFile`
- `server.grpc.tls.keyFile`
- `server.grpc.tls.version`
- `server.grpc.mtls.enable`
- `server.grpc.mtls.clientCAFile`
- `server.grpc.mtls.clientAuth`

### Endpoint refresh en gRPC

`server/grpc` expone un RPC administrativo de refresh mediante el nombre interno
de método `"/quicksgo.admin/Refresh"`.

Qué hace:

- llama `jobs.RestartJobs()` a través de la variable función interna `restartJobs`
- ejecuta los callbacks registrados con `SetFunctionsRefresh(...)`
- propaga el mismo RPC a los hosts registrados con `SetHostsRefresh(...)`
- usa metadata `broadcast-refresh=true` para evitar loops de propagación entre nodos

Uso típico:

- refrescar estado local del proceso en todos los nodos
- reiniciar de forma coordinada los jobs programados a nivel paquete
- propagar un evento manual u operativo de recarga entre instancias gRPC

El request y el response de este RPC administrativo están vacíos, y el flujo
está pensado para coordinación interna entre servicios.

## Cliente HTTP

`client/http` expone un cliente REST genérico con trazabilidad de request/response.

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

`client/grpc` envuelve `grpc.ClientConn` y puede:

- crear clientes protobuf con `BuildClient`
- resolver TLS/mTLS desde `viper`
- trazar metadata, request y response con `logger`

Uso básico:

```go
cli := clientGRPC.NewIClient(nil, nil)

greeter, err := clientGRPC.BuildClient(cli, pb.NewGreeterClient)
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

## Jobs en background

`utilities/jobs` permite ejecutar tareas recurrentes dentro del mismo proceso.

Hay dos formas comunes de usarlo:

- helpers de paquete como `jobs.Job(...)`, `jobs.CronJob(...)` y `jobs.StartJobs()`
- una instancia aislada creada con `jobs.NewJobs()` cuando se quiere controlar su ciclo de vida por separado

Comportamiento importante:

- registrar un job no lo inicia automáticamente; comienza cuando se ejecuta `StartJobs()`
- `server/gin.Start(...)` ya invoca `jobs.StartJobs()` internamente
- si registras jobs después de `StartJobs()`, comienzan inmediatamente
- cuando `server.modeTest=true`, `StartJobs()` no ejecuta jobs

### Helpers de ciclo de vida a nivel paquete

Estos helpers operan sobre el scheduler global del paquete:

#### `StartJobs()`

Inicia el ciclo del scheduler global.

Qué hace:

- arranca los jobs registrados en el scheduler global
- mantiene un watcher interno activo mientras el sistema de jobs siga marcado como iniciado
- es el punto de entrada que usa `server/gin.Start(...)`

Notas:

- los jobs deben haberse registrado antes con `Job(...)` o `CronJob(...)`
- si `server.modeTest=true`, los jobs no se inician

#### `RestartJobs()`

Solicita un reinicio del scheduler global.

Qué hace:

- envía una señal interna de reinicio
- hace que el scheduler detenga los jobs actuales sin borrar sus definiciones
- los vuelve a iniciar desde el estado actual

Útil cuando:

- cambió la configuración y quieres reiniciar el conjunto actual de jobs
- necesitas reprogramar jobs activos sin reiniciar el proceso

#### `StopAllJobs(clearJobs bool)`

Detiene todos los jobs registrados globalmente con `NewJobs()`.

Comportamiento:

- si `clearJobs=false`, los jobs se detienen pero siguen registrados, por lo que pueden iniciarse otra vez
- si `clearJobs=true`, los jobs se detienen y además se eliminan sus definiciones de cada instancia registrada

Uso típico:

- `StopAllJobs(false)` para flujos temporales de stop / restart
- `StopAllJobs(true)` para pruebas, shutdown o reinicio completo

#### `CheckStatusJobs() bool`

Devuelve si el paquete considera que el sistema global de jobs sigue activo.

En la práctica:

- `true` significa que los jobs fueron iniciados y no se han detenido completamente
- `false` significa que el scheduler global está detenido

Esto es especialmente útil para diagnóstico, pruebas o checks operativos.

### `Job(fn func(), interval time.Duration, timeout *time.Duration)`

Usa `Job` para intervalos fijos como "cada 30 segundos" o "cada 5 minutos".

Parámetros:

- `fn`: función que se ejecuta en cada ciclo
- `interval`: frecuencia de ejecución; si `interval <= 0`, el job se ignora
- `timeout`: tiempo total de vida del job; si es `nil`, el job sigue hasta shutdown o `StopAllJobs`

Comportamiento:

- la primera ejecución ocurre inmediatamente cuando el job inicia
- las siguientes usan el `interval` configurado
- si `timeout != nil` y `*timeout > 0`, el job se detiene automáticamente cuando expira ese tiempo

Ejemplo sin timeout:

```go
import (
	"time"

	"github.com/PointerByte/QuicksGo/config/utilities/jobs"
)

func registerJobs() {
	jobs.Job(func() {
		refreshCache()
	}, 30*time.Second, nil)
}
```

Ejemplo con timeout:

```go
func registerJobs() {
	timeout := 10 * time.Minute

	jobs.Job(func() {
		pollTemporarySource()
	}, 15*time.Second, &timeout)
}
```

### `CronJob(fn func(), trigger CronTrigger, interval time.Duration)`

Usa `CronJob` cuando la primera ejecución debe alinearse a una hora/minuto/segundo específicos.

Parámetros:

- `fn`: función a ejecutar
- `trigger`: hora diaria de inicio usando `Hour`, `Minute` y `Second`
- `interval`: define qué pasa después de la primera ejecución alineada

Comportamiento:

- si `interval <= 0`, el job corre una vez al día en el `trigger` indicado
- si `interval > 0`, la primera ejecución espera al siguiente `trigger` válido y después repite cada `interval`

Ejemplo: ejecutar todos los días a las 09:00:00

```go
func registerJobs() {
	jobs.CronJob(func() {
		buildDailyReport()
	}, jobs.CronTrigger{
		Hour:   9,
		Minute: 0,
		Second: 0,
	}, 0)
}
```

Ejemplo: primera ejecución a las 08:30:00 y luego cada 5 minutos

```go
func registerJobs() {
	jobs.CronJob(func() {
		syncMorningWindow()
	}, jobs.CronTrigger{
		Hour:   8,
		Minute: 30,
		Second: 0,
	}, 5*time.Minute)
}
```

### Ejemplo completo con helpers de paquete

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

	jobs.StartJobs()

	if jobs.CheckStatusJobs() {
		log.Println("jobs running")
	}

	// Ejemplo de reinicio operativo:
	jobs.RestartJobs()

	// Ejemplo de apagado final:
	jobs.StopAllJobs(true)
}
```

### Patrón de integración recomendado

Si usas el servidor Gin, un patrón común es:

```go
func main() {
	registerJobs()

	srv, err := serverGin.CreateApp()
	if err != nil {
		log.Fatal(err)
	}

	serverGin.Start(srv) // aquí también se inician los jobs registrados
}
```

Si necesitas control manual fuera del bootstrap de Gin, llama `jobs.StartJobs()` explícitamente:

```go
func main() {
	jobs.Job(func() {
		cleanup()
	}, time.Minute, nil)

	jobs.StartJobs()

	select {}
}
```

## Ejemplo ejecutable

El proyecto incluye un ejemplo en [main.go](/e:/Proyects/Practices/QuicksGo/config/main.go).

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
