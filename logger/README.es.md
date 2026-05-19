# GoForge Logger

`logger` provee la capa de logging estructurado de GoForge. Configura un
logger global basado en `slog`, formatea entradas como JSON, texto o template
custom, exporta logs mediante OpenTelemetry e incluye middleware Gin e
interceptores gRPC para logs asociados al request.

## Instalacion

```bash
go get github.com/PointerByte/GoForge/logger
```

## Paquetes

- `builder`: inicializacion del logger, logging con contexto y secciones de traza
- `common`: llaves de contexto compartidas por middlewares HTTP y gRPC
- `middlewares/http`: middleware Gin
- `middlewares/grpc`: interceptores gRPC
- `formatter`: modelos de log estructurado e implementaciones de formatter
- `viperData`: cache de configuracion respaldada por viper
- `utilities`: helpers pequenos para detectar caller

## Configuracion

El modulo lee configuracion desde `viper`. No carga archivos por si solo, asi
que tu aplicacion debe cargar `application.yaml`, `application.yml`, JSON o
variables de entorno antes de llamar `builder.InitLogger(...)` o instalar
middlewares.

```yaml
app:
  name: service-template
  version: 0.0.1

server:
  gin:
    LoggerWithConfig:
      enabled: true
      SkipPaths:
        - /health
      SkipQueryString: false
  grpc:
    LoggerWithConfig:
      enabled: true
      SkipFunction: []

logger:
  dir: logs
  modeTest: false
  level: info
  ignoredHeaders:
    - Authorization
    - Cookie
  formatter: json
  formatDate: "2006-01-02T15:04:05.000"
  rotate:
    enable: true
    maxSize: 10
    maxBackups: 5
    maxAge: 30
    compress: true
```

Claves principales:

- `app.name`: nombre del servicio incluido en details y metadata OTEL
- `app.version`: version del servicio incluida en metadata OTEL
- `server.gin.LoggerWithConfig.enabled`: habilita logs finales de requests Gin
- `server.gin.LoggerWithConfig.SkipPaths`: rutas omitidas por el logging Gin
- `server.gin.LoggerWithConfig.SkipQueryString`: omite query strings del path logueado
- `server.grpc.LoggerWithConfig.enabled`: habilita logs finales de requests gRPC
- `server.grpc.LoggerWithConfig.SkipFunction`: metodos gRPC omitidos por nombre (`SayHello`) o metodo completo (`/pkg.Service/SayHello`)
- `logger.dir`: directorio donde se crea el archivo de log cuando el caller usa esta clave
- `logger.modeTest`: suprime salida de logs y coleccion de trazas en modo test
- `logger.level`: `debug`, `info`, `warn` o `error`
- `logger.ignoredHeaders`: headers filtrados de los details estructurados
- `logger.formatter`: `json`, `text` o un template Go custom
- `logger.formatDate`: layout de timestamp
- `logger.rotate.*`: configuracion de rotacion de archivos con `lumberjack`

`viperData` cachea valores en el primer uso. En tests que cambian valores de
viper dentro del mismo proceso, llama `viperdata.ResetViperDataSingleton()`
antes de volver a leer configuracion del logger.

## Inicializar El Logger

```go
package main

import (
	"context"
	"path/filepath"

	"github.com/PointerByte/GoForge/logger/builder"
	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigName("application")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}

	ctx := context.Background()
	lp, err := builder.InitLogger(ctx, filepath.Join(".", viper.GetString("logger.dir")))
	if err != nil {
		panic(err)
	}
	defer lp.Shutdown(ctx)

	builder.New(ctx).Info("logger initialized")
}
```

`builder.InitLogger` configura el logger `slog` default del proceso. Escribe en
stdout y, cuando `logger.rotate.enable=true`, tambien en un archivo rotado.
Tambien crea un logger provider de OpenTelemetry y lo devuelve para que el
caller pueda apagarlo de forma ordenada.

## Middleware Gin

```go
package main

import (
	"net/http"

	httpmiddlewares "github.com/PointerByte/GoForge/logger/middlewares/http"
	"github.com/gin-gonic/gin"
)

func main() {
	engine := gin.New()
	engine.Use(
		gin.Recovery(),
		httpmiddlewares.InitLogger(),
		httpmiddlewares.LoggerWithConfig(),
		httpmiddlewares.CaptureBody(),
	)

	engine.GET("/health", func(c *gin.Context) {
		httpmiddlewares.EnableBody(c, true, true)
		httpmiddlewares.PrintInfo(c, "health check")
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
```

Rol de cada middleware:

- `InitLogger()` extrae headers de tracing distribuido, crea el contexto logger del request y guarda metadata del request.
- `LoggerWithConfig()` emite el log HTTP estructurado final mediante el hook logger de Gin.
- `CaptureBody()` captura request y response solo cuando el logging de bodies esta habilitado, para incluirlos en `details.request` y `details.response` sin guardar payloads deshabilitados.
- `EnableBody(c, request, response)` habilita la emision de request y response body en el log HTTP final. Los bodies estan deshabilitados por default.
- `EnableTraceBody(c, request, response)` habilita la emision de request y response body en las entradas de servicios de traza cuando se llama `TraceEnd`. Los bodies de traza estan deshabilitados por default.

Los helpers `PrintInfo`, `PrintDebug`, `PrintWarn` y `PrintError` programan un
mensaje de log asociado al request desde handlers Gin.

## Interceptores gRPC

```go
import loggrpc "github.com/PointerByte/GoForge/logger/middlewares/grpc"

grpcServer := grpc.NewServer(
	grpc.ChainUnaryInterceptor(
		loggrpc.InitLoggerUnaryServerInterceptor(),
		loggrpc.LoggerWithConfigUnaryServerInterceptor(),
		loggrpc.CaptureBodyUnaryServerInterceptor(),
	),
	grpc.ChainStreamInterceptor(
		loggrpc.InitLoggerStreamServerInterceptor(),
		loggrpc.LoggerWithConfigStreamServerInterceptor(),
		loggrpc.CaptureBodyStreamServerInterceptor(),
	),
)
```

Los interceptores replican el comportamiento de Gin para RPCs unary y stream:
crean el contexto logger del request, capturan payloads, copian metadata en los
details estructurados y escriben el log final cuando termina el handler.

Los request y response bodies estan deshabilitados por default. Usa
`loggrpc.EnableBody(ctxLogger, true, true)` para incluirlos en el log gRPC
final, y `loggrpc.EnableTraceBody(ctxLogger, true, true)` para incluir bodies
en servicios de traza.

El logging gRPC final ignora intencionalmente errores `codes.Unauthenticated` y
`codes.PermissionDenied`, por lo que las fallas de autorizacion JWT no emiten
logs del middleware logger.

Usa `loggrpc.PrintInfo`, `PrintDebug`, `PrintWarn` o `PrintError` con el
contexto logger del request cuando un handler necesite elegir explicitamente el
nivel y mensaje del log final.

Cuando usas el paquete raiz `config/server/grpc`, estos interceptores se
instalan automaticamente.

## Logger Con Contexto

Usa `builder.New(ctx)` fuera de handlers Gin o gRPC cuando necesites un logger
contextual directamente:

```go
ctxLogger := builder.New(context.Background())

ctxLogger.Info("application started")
ctxLogger.Debug("cache warmed")
ctxLogger.Warn("dependency latency is high")
ctxLogger.Error(errors.New("dependency failed"))
```

## Secciones De Traza

`TraceInit` y `TraceEnd` agregan llamadas downstream o subprocesos internos al
array `services` del log estructurado.

```go
process := &formatter.Service{
	System:  "users-service",
	Process: "list-users",
	Method:  "GET",
	Server:  "https://users.internal",
	Path:    "/users",
}

ctxLogger.TraceInit(process)
defer ctxLogger.TraceEnd(process)

process.Code = 200
```

Casos comunes: llamadas HTTP/gRPC salientes, llamadas a SDKs de proveedores y
pasos internos de negocio que deben aparecer bajo la misma traza.

Los request y response bodies de servicios de traza estan deshabilitados por
default. En handlers Gin usa `httpmiddlewares.EnableTraceBody(c, true, true)`;
en handlers gRPC usa `loggrpc.EnableTraceBody(ctxLogger, true, true)`.

## Formatters

`logger.formatter` soporta:

- `json`: salida JSON estructurada
- `text` o `txt`: salida de texto legible
- cualquier template Go custom aceptado por `formatter.CustomFormatter`

Los templates custom pueden usar helpers como `json`, `buildDetails` y
`buildServices`.

## Pruebas

Para silenciar salida de logs y coleccion de trazas en pruebas:

```go
builder.EnableModeTest()
defer builder.DisableModeTest()
```

Desde el directorio del modulo `logger`:

```bash
go test ./...
go test -cover -covermode=atomic -coverprofile=coverage.out ./...
```
