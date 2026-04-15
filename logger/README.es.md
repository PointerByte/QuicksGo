# Configuration

La aplicación se configura preferentemente mediante un archivo YAML.  
YAML debe considerarse el formato por defecto a seguir para nuevas configuraciones y mantenimiento futuro.

JSON sigue siendo soportado como formato opcional para compatibilidad o integración con flujos existentes.

Este archivo define información de la aplicación, configuración del servidor, configuración de Gin y del sistema de logs.

## Formato recomendado por defecto: YAML

```yaml
app:
  name: service-template
  version: 0.0.1

server:
  port: ":10443"
  groups:
    - /service-template/v1

gin:
  LoggerWithConfig:
    enabled: true
    SkipPaths:
      - /health
    SkipQueryString: false

logger:
  level: info
  ignoredHeaders:
    - Authorization
    - Cookie
  rotate:
    enable: true
    maxSize: 10
    maxBackups: 5
    maxAge: 30
    compress: true
  formatter: json
  formatDate: "2006-01-02T15:04:05.000"
```

## Formato opcional: JSON

Si necesitas usar JSON, la misma configuración puede expresarse así:

```json
{
  "app": {
    "name": "service-template",
    "version": "0.0.1"
  },
  "server": {
    "port": ":10443",
    "groups": ["/service-template/v1"]
  },
  "gin": {
    "LoggerWithConfig": {
      "enabled": true,
      "SkipPaths": ["/health"],
      "SkipQueryString": false
    }
  },
  "logger": {
    "level": "info",
    "ignoredHeaders": ["Authorization", "Cookie"],
    "rotate": {
      "enable": true,
      "maxSize": 10,
      "maxBackups": 5,
      "maxAge": 30,
      "compress": true
    },
    "formatter": "json",
    "formatDate": "2006-01-02T15:04:05.000"
  }
}
```

## Como usar esta dependencia

Este paquete toma su configuracion desde `viper` y luego inicializa un logger custom basado en `slog` con salida a archivo, salida a consola, exportacion OTEL, logging HTTP para Gin y soporte para trazas de servicios satelite.

### 1. Instalar la dependencia

```bash
go get github.com/PointerByte/QuicksGo/logger
```

### 2. Cargar la configuracion en Viper

Antes de invocar el paquete, tu aplicacion debe haber cargado en `viper` los valores que el logger espera.

Ejemplo usando `application.yaml`:

```go
package main

import "github.com/spf13/viper"

func loadConfig() error {
	viper.SetConfigName("application")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	return viper.ReadInConfig()
}
```

### 3. Inicializar el logger

`builder.InitLogger` configura el logger global y devuelve el provider de OTEL para que puedas apagarlo de forma ordenada.

```go
package main

import (
	"context"
	"path/filepath"

	"github.com/PointerByte/QuicksGo/logger/builder"
)

func initLogger(ctx context.Context) error {
	lp, err := builder.InitLogger(ctx, filepath.Join(".", "logs"))
	if err != nil {
		return err
	}

	defer lp.Shutdown(ctx)
	return nil
}
```

### 4. Usarlo con Gin

Para servicios HTTP, el orden recomendado de middlewares es:

```go
engine := gin.New()
engine.Use(
	gin.Recovery(),
	middlewares.InitLogger(),
	middlewares.LoggerWithConfig(),
	middlewares.CaptureBody(),
)
```

Que hace cada middleware:

- `middlewares.InitLogger()` crea o propaga el contexto de traza del request.
- `middlewares.LoggerWithConfig()` escribe la entrada final del log HTTP usando el formatter configurado.
- `middlewares.CaptureBody()` captura request y response para incluirlos en `details.request` y `details.response`.

Imports usados por el ejemplo anterior:

```go
import (
	"github.com/PointerByte/QuicksGo/logger/middlewares"
	"github.com/gin-gonic/gin"
)
```

### 5. Usarlo con gRPC

Para servidores gRPC, el paquete tambien expone interceptores unary y stream que replican el comportamiento de los middlewares HTTP:

```go
grpcServer := grpc.NewServer(
	grpc.ChainUnaryInterceptor(
		traces.MiddlewareOtelGRPCUnary(),
		middlewares.InitLoggerUnaryServerInterceptor(),
		middlewares.LoggerWithConfigUnaryServerInterceptor(),
		middlewares.CaptureBodyUnaryServerInterceptor(),
	),
	grpc.ChainStreamInterceptor(
		traces.MiddlewareOtelGRPCStream(),
		middlewares.InitLoggerStreamServerInterceptor(),
		middlewares.LoggerWithConfigStreamServerInterceptor(),
		middlewares.CaptureBodyStreamServerInterceptor(),
	),
)
```

Orden recomendado:

- OTEL primero, para extraer la traza distribuida antes de que el logger cree su contexto por request.
- `InitLogger*` despues, para que el contexto del logger quede disponible para el resto de interceptores.
- `LoggerWithConfig*` antes de `CaptureBody*`, igual que en HTTP, para que el log final vea los payloads capturados cuando el handler termina.

Que hace cada interceptor gRPC:

- `middlewares.InitLoggerUnaryServerInterceptor()` y `middlewares.InitLoggerStreamServerInterceptor()` crean el contexto del logger, adjuntan metadata del metodo gRPC, copian headers entrantes y abren el span del logger.
- `middlewares.CaptureBodyUnaryServerInterceptor()` guarda el request y response unary en el contexto del logger.
- `middlewares.CaptureBodyStreamServerInterceptor()` captura mensajes de entrada y salida del stream, y los guarda como un solo valor o como slice cuando hay multiples mensajes.
- `middlewares.LoggerWithConfigUnaryServerInterceptor()` y `middlewares.LoggerWithConfigStreamServerInterceptor()` escriben la entrada final del log estructurado y copian los bodies capturados hacia `details.request` y `details.response` cuando el body logging esta habilitado.
- `traces.MiddlewareOtelGRPCUnary()` y `traces.MiddlewareOtelGRPCStream()` crean el span server de OpenTelemetry para cada RPC.

Imports usados por el ejemplo anterior:

```go
import (
	"github.com/PointerByte/QuicksGo/logger/middlewares"
	"github.com/PointerByte/QuicksGo/config/utilities/traces"
	"google.golang.org/grpc"
)
```

### 6. Loggear dentro de handlers

Para logs asociados al request dentro de handlers Gin, usa las funciones helper:

```go
func exampleHandler(c *gin.Context) {
	builder.PrintInfo(c, "request procesado")
	c.JSON(200, gin.H{"ok": true})
}
```

Helpers disponibles:

- `builder.PrintInfo`
- `builder.PrintDebug`
- `builder.PrintWarn`
- `builder.PrintError`

### 7. Usar el logger contextual directamente

Si necesitas loggear fuera de Gin, o quieres conservar estado en un contexto propio, crea un contexto logger con `builder.New`.

```go
import (
	"context"
	"errors"
)

ctx := context.Background()
ctxLogger := builder.New(ctx)

ctxLogger.Info("aplicacion iniciada")
ctxLogger.Warn("dependencia lenta detectada")
ctxLogger.Error(errors.New("falla inesperada"))
```

### 8. Trazar consumos o subprocesos

Usa `TraceInit` y `TraceEnd` para registrar servicios satelite o subprocesos internos dentro de la seccion `services` del log.

```go
process := formatter.Service{
	System:  "auth-service",
	Process: "validate-token",
	Method:  "POST",
	Path:    "/auth/validate",
	Server:  "auth.internal",
}

ctxLogger.TraceInit(&process)
defer ctxLogger.TraceEnd(&process)

process.Code = 200
```

Casos comunes:

- llamadas HTTP salientes
- subprocesos internos de negocio
- integraciones que deben quedar dentro de la misma traza

### 9. Modo test

Si necesitas silenciar la salida de logs durante pruebas, activa el modo test antes de ejecutar el logger:

```go
builder.EnableModeTest()
defer builder.DisableModeTest()
```

## Ejemplo end-to-end

```go
package main

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/PointerByte/QuicksGo/logger/builder"
	"github.com/gin-gonic/gin"
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
	lp, err := builder.InitLogger(ctx, filepath.Join(".", "logs"))
	if err != nil {
		panic(err)
	}
	defer lp.Shutdown(ctx)

	engine := gin.New()
	engine.Use(
		gin.Recovery(),
		builder.MiddlewareInitLogger(),
		builder.MiddlewareLoggerWithConfig(),
		builder.MiddlewareCaptureBody(),
	)

	engine.GET("/health", func(c *gin.Context) {
		builder.PrintInfo(c, "health check")
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	_ = engine.Run(":8080")
}
```

## Nota de uso

- Usa YAML como formato principal para nuevos proyectos y ejemplos de referencia.
- Usa JSON solo cuando exista una necesidad específica de compatibilidad.
- A nivel de estructura, ambos formatos representan la misma configuración.

---

# app

Información general del servicio.

```json
"app": {
  "name": "service-template",
  "version": "0.0.1"
}
```

| Campo | Descripción |
|------|-------------|
| `name` | Nombre del servicio o aplicación |
| `version` | Versión actual del servicio |

---

# server

Configuración del servidor HTTP.

```json
"server": {
  "port": ":10443",
  "groups": ["/api/v1"]
}
```

| Campo | Descripción |
|------|-------------|
| `port` | Puerto donde corre el servidor |
| `groups` | Lista de prefijos base para las rutas de la API |

`groups` permite definir múltiples grupos de rutas para organizar los endpoints del servicio.

---

# gin

Configuración relacionada con Gin y sus middlewares.

```json
"gin": {
  "LoggerWithConfig": {
    "enabled": true,
    "SkipPaths": ["/health"],
    "SkipQueryString": false
  }
}
```

Esta sección permite controlar el comportamiento del logger HTTP de Gin cuando se utiliza `LoggerWithConfig`.

## Objetivo

La configuración de `gin` sirve para controlar cómo se registran las peticiones HTTP que entran al servicio cuando Gin está actuando como framework web.

Esto permite, por ejemplo:

- habilitar o deshabilitar el logger HTTP de Gin
- omitir rutas que no interesa registrar
- decidir si el query string debe formar parte del path registrado

---

# server.gin.LoggerWithConfig

Define la configuración del middleware `LoggerWithConfig` de Gin.

```json
"gin": {
  "LoggerWithConfig": {
    "enabled": true,
    "SkipPaths": ["/health"],
    "SkipQueryString": false
  }
}
```

| Campo | Descripción |
|------|-------------|
| `enabled` | Habilita o deshabilita el logger HTTP de Gin |
| `SkipPaths` | Lista de rutas que no deben registrarse en el logger de Gin |
| `SkipQueryString` | Define si el query string debe excluirse del path registrado |

## `enabled`

Permite activar o desactivar el middleware de logging HTTP de Gin.

```json
"enabled": true
```

### Comportamiento

- `true`: el logger HTTP de Gin se encuentra habilitado
- `false`: el logger HTTP de Gin no se aplica

### Recomendación

Mantenerlo en `true` cuando se quiera tener trazabilidad de las peticiones HTTP del servicio.

Puede desactivarse si el logging HTTP ya está cubierto por otro middleware o si no se desea registrar tráfico de entrada desde Gin.

---

## `SkipPaths`

Controla qué rutas debe omitir el logger HTTP de Gin.

```json
"SkipPaths": ["/health"]
```

### Objetivo

Se usa para evitar ruido en el log cuando existen endpoints que se consultan con mucha frecuencia y que normalmente no aportan valor de observabilidad detallada.

Casos típicos:

- health checks
- readiness probes
- liveness probes
- endpoints internos de monitoreo

### Ejemplo

```json
"SkipPaths": [
  "/health",
  "/ready",
  "/live"
]
```

Con esta configuración, las peticiones a esas rutas no deberían aparecer en el log HTTP de Gin.

### Recomendación

Agregar aquí endpoints operativos o de infraestructura que puedan generar mucho ruido, especialmente cuando son invocados periódicamente por balanceadores, Kubernetes o herramientas de monitoreo.

---

## `SkipQueryString`

Controla si el logger HTTP de Gin incluye el query string dentro del path registrado.

```json
"SkipQueryString": false
```

### Comportamiento

- `false`: el path registrado conserva el query string cuando existe
- `true`: el path registrado omite el query string y deja solo la ruta base

### Ejemplo

Si llega una petición como:

```text
GET /customers?id=123&type=premium
```

Entonces:

- con `SkipQueryString: false`, el log puede conservar la parte `?id=123&type=premium`
- con `SkipQueryString: true`, el log registra solo la ruta base `/customers`

### Cuándo conviene activarlo

Se recomienda usar `true` cuando:

- el query string puede contener datos sensibles
- no se desea generar alta cardinalidad en los logs
- se quiere un path más limpio y estable para búsqueda y agregación

Se puede dejar en `false` cuando el query string forma parte importante del análisis funcional y no representa un riesgo de seguridad o ruido excesivo.

---

## Ejemplo recomendado

```json
"gin": {
  "LoggerWithConfig": {
    "enabled": true,
    "SkipPaths": ["/health", "/ready", "/live"],
    "SkipQueryString": true
  }
}
```

Esta configuración suele ser adecuada cuando se busca:

- reducir ruido en logs
- evitar registrar paths operativos repetitivos
- mantener rutas limpias sin parámetros variables

---

# logger

Configuración del sistema de logs.

```json
"logger": {
  "level": "info",
  "ignoredHeaders": ["Authorization", "Cookie"],
  "rotate": {
    "enable": true,
    "maxSize": 10,
    "maxBackups": 5,
    "maxAge": 30,
    "compress": true
  },
  "formatter": "json",
  "formatDate": "2006-01-02T15:04:05.000"
}
```

| Campo | Descripción |
|------|-------------|
| `level` | Nivel mínimo de logs que se registrarán |
| `ignoredHeaders` | Lista de headers HTTP que no deben incluirse en el log |
| `rotate` | Configuración de rotación de logs |
| `formatter` | Formato de salida del log |
| `formatDate` | Formato de fecha usado en los logs |

---

# logger.level

Define el nivel mínimo de logs que serán registrados.

```json
"level": "info"
```

## Niveles disponibles

| Nivel | Descripción |
|------|-------------|
| `debug` | Información detallada para debugging |
| `info` | Información general del sistema |
| `error` | Errores que afectan la operación |

---

# logger.ignoredHeaders

Define qué headers HTTP deben excluirse del log estructurado, específicamente del campo `details.headers`.

## Objetivo

`ignoredHeaders` se utiliza para evitar que información sensible o innecesaria quede registrada en los logs, por ejemplo:

- tokens de autorización
- cookies de sesión
- llaves de API
- credenciales u otros headers privados

## Ejemplo

```json
"logger": {
  "ignoredHeaders": ["Authorization", "Cookie"]
}
```

Si el request incluye headers como:

```http
Authorization: Bearer abc123
Cookie: session=xyz789
Content-Type: application/json
X-Trace-Id: 12345
```

el resultado esperado en el log sería conservar únicamente los headers no ignorados, por ejemplo:

```json
"details": {
  "headers": {
    "Content-Type": ["application/json"],
    "X-Trace-Id": ["12345"]
  }
}
```

## Recomendación

Agregar aquí cualquier header que pueda contener información sensible, por ejemplo:

```json
"ignoredHeaders": [
  "Authorization",
  "Cookie",
  "Set-Cookie",
  "X-Api-Key"
]
```

---

# logger.rotate

Configuración de rotación automática de archivos de log.

```json
"rotate": {
  "enable": true,
  "maxSize": 10,
  "maxBackups": 5,
  "maxAge": 30,
  "compress": true
}
```

| Campo | Descripción |
|------|-------------|
| `enable` | Habilita la rotación de logs |
| `maxSize` | Tamaño máximo del archivo en MB antes de rotar |
| `maxBackups` | Número máximo de archivos antiguos a mantener |
| `maxAge` | Número máximo de días que se guardan los logs |
| `compress` | Comprime archivos antiguos en `.gz` |

---

# logger.formatter

Define cómo se escribe el log al final.

El formatter soporta tres enfoques:

1. salida JSON estructurada
2. salida de texto legible
3. salida personalizada mediante templates

## Valores soportados

| Valor | Comportamiento |
|---|---|
| `json` | Genera el log en JSON usando el template interno por defecto |
| `text` | Genera una salida legible en texto |
| `txt` | Alias de `text` |
| `""` | Si está vacío, usa formato `text` |
| cualquier otro valor | Se interpreta como un template personalizado completo |

Esto significa que no existe un valor especial llamado `template`.  
Cualquier cadena distinta de `json`, `text`, `txt` o vacío será tratada como una plantilla de salida completa.

---

## Formato `json`

Si se configura:

```json
"formatter": "json"
```

la salida será un log JSON en una sola línea.

### Características del modo `json`

- usa un template interno por defecto
- imprime el JSON en una sola línea
- omite campos nulos
- omite campos vacíos
- omite campos opcionales que no tengan valor
- mantiene una estructura consistente para observabilidad

### Ejemplo

```json
{"timestamp":"2026-03-13T01:10:23.123","traceID":"main-trace-001","level":"INFO","message":"Loan simulation completed","details":{"system":"loan-service","client":"mobile-app","protocol":"HTTP","method":"POST","path":"/loan/simulate","headers":{"Content-Type":["application/json"]},"request":{"amount":10000,"term":12},"response":{"approved":true}},"services":[{"traceID":"sat-001","system":"auth-service","process":"validate-token","server":"auth.internal","protocol":"HTTP","method":"POST","path":"/auth/validate","code":200,"status":"SUCCESS","latency":12},{"traceID":"sat-002","system":"score-engine","process":"calculate-score","server":"score.internal","protocol":"HTTP","method":"POST","path":"/score/calculate","code":200,"status":"SUCCESS","latency":28}],"method":"SimulateLoan","line":87,"totalTime":64}
```

---

## Formato `text`

Si se configura:

```json
"formatter": "text"
```

la salida será una línea de texto legible.

### Ejemplo

```text
[2026-03-13T01:10:23.123] [INFO] [8f3a5d9c-9f2a-4e1d-b3a7-7f23d9a1e4aa] ProcessPayment:142 - Request processed successfully totalTime=155ms
```

---

## Personalización con templates

Además de `json` y `text`, el logger permite personalizar completamente la salida usando templates.

Si `formatter` contiene cualquier valor distinto de:

- `json`
- `text`
- `txt`
- vacío

entonces ese valor se interpreta como el template completo que definirá la salida final.

## Nivel de personalización soportado

Este nivel de personalización permite cambiar no solo el contenido del log, sino también su estructura completa.

Con un template personalizado puedes:

- cambiar nombres de campos
- omitir secciones completas
- agrupar datos de otra manera
- combinar varios campos en uno solo
- emitir JSON de una sola línea
- emitir texto libre
- mezclar texto con fragmentos JSON
- reducir el tamaño del log final
- adaptar la salida a integraciones legacy o plataformas específicas

---

## Importante: el template reemplaza por completo la salida

Cuando usas un template personalizado, ese template define completamente la salida final.

Eso significa que no se usa esta forma:

```json
"formatter": "json { ... }"
```

Esa sintaxis no es válida.

## Forma correcta

Debes usar una de estas dos opciones:

### Opción 1: usar el JSON por defecto

```json
"formatter": "json"
```

### Opción 2: usar un template completo personalizado

```json
"formatter": "{\"timestamp\":{{json .Timestamp}},\"traceID\":{{json .TraceID}},\"message\":{{json .Message}}}"
```

En esta segunda opción, el valor completo de `formatter` ya es el template.

---

## Personalización avanzada de JSON

Si quieres cambiar la estructura del JSON, debes usar un template JSON completo dentro de `formatter`.

Esto permite transformar la forma de salida sin depender de la estructura por defecto.

### Ejemplo: combinar `method` y `path` en `details.service`

Si quieres imprimir esto:

```json
{
  "timestamp": "2026-03-13T01:10:23.123",
  "traceID": "main-trace-001",
  "level": "INFO",
  "message": "Loan simulation completed",
  "details": {
    "system": "loan-service",
    "client": "mobile-app",
    "protocol": "HTTP",
    "service": "POST /loan/simulate",
    "headers": {
      "Content-Type": ["application/json"]
    },
    "request": {
      "amount": 10000,
      "term": 12
    },
    "response": {
      "approved": true
    }
  },
  "services": [
    {
      "traceID": "sat-001",
      "system": "auth-service",
      "process": "validate-token",
      "protocol": "HTTP",
      "method": "POST",
      "server": "auth.internal",
      "code": 200,
      "path": "/auth/validate",
      "status": "SUCCESS",
      "latency": 12
    }
  ],
  "method": "SimulateLoan",
  "line": 87,
  "totalTime": 64
}
```

entonces el `formatter` debe ser un template completo como este:

```json
"formatter": "{\"timestamp\":{{json .Timestamp}},\"traceID\":{{json .TraceID}},\"level\":{{json .Level}},\"message\":{{json .Message}},\"details\":{\"system\":{{json .Details.System}},\"client\":{{json .Details.Client}},\"protocol\":{{json .Details.Protocol}},\"service\":{{json (printf \"%s %s\" .Details.Method .Details.Path | trim)}},\"headers\":{{json .Details.Headers}},\"request\":{{json .Details.Request}},\"response\":{{json .Details.Response}}},\"services\":{{json (buildServices .Services)}},\"method\":{{json .Method}},\"line\":{{json .Line}},\"totalTime\":{{json .Time}}}"
```

### Resultado esperado

Con ese template, `details` ya no imprime:

```json
"method": "POST",
"path": "/loan/simulate"
```

y en su lugar imprime:

```json
"service": "POST /loan/simulate"
```

---

## Qué puedes personalizar

Con templates se puede decidir exactamente qué mostrar del log, por ejemplo:

- solo el mensaje
- mensaje más método y línea
- request y response
- traza principal
- consumos o satélites de `services`
- un JSON completamente distinto al template por defecto

## Información disponible para personalizar

Se puede construir una salida personalizada a partir de:

- `Timestamp`
- `TraceID`
- `Level`
- `Message`
- `Method`
- `Line`
- `Time`
- `Details`
- `Services`

Dentro de `Details` normalmente interesa:

- `System`
- `Client`
- `Protocol`
- `Method`
- `Path`
- `Headers`
- `Request`
- `Response`

Dentro de `Services` normalmente interesa:

- `IdTrace`
- `System`
- `Process`
- `Server`
- `Protocol`
- `Method`
- `Path`
- `Code`
- `Request`
- `Response`
- `Status`
- `Latency`

> Nota: en la salida JSON del consumo el campo se ve como `traceID`, pero en la personalización de templates se referencia como `IdTrace`.

---

## Ejemplos de personalización

### 1) Mensaje simple

```json
"formatter": "{{.Message}}"
```

Salida:

```text
Request processed successfully
```

### 2) Mensaje con método y línea

```json
"formatter": "{{.Message}} | {{.Method}}:{{.Line}}"
```

Salida:

```text
Request processed successfully | ProcessPayment:142
```

### 3) Access log básico

```json
"formatter": "[{{.Timestamp}}] {{.Level}} {{.Details.Method}} {{.Details.Path}} trace={{.TraceID}} msg={{.Message}}"
```

Salida:

```text
[2026-03-13T01:10:23.123] INFO POST /payments trace=8f3a5d9c-9f2a-4e1d-b3a7-7f23d9a1e4aa msg=Request processed successfully
```

### 4) Request y response en una salida compacta

```json
"formatter": "{{.Message}} | req={{json .Details.Request}} | resp={{json .Details.Response}}"
```

Salida:

```text
Request processed successfully | req={"amount":100} | resp={"status":"ok"}
```

### 5) Log completo serializado desde un template

```json
"formatter": "{{json .}}"
```

Salida aproximada:

```json
{"timestamp":"2026-03-13T01:10:23.123","traceID":"8f3a5d9c-9f2a-4e1d-b3a7-7f23d9a1e4aa","level":"INFO","message":"Request processed successfully","details":{},"services":[],"method":"ProcessPayment","line":142,"totalTime":155}
```

### 6) Imprimir satélites o consumos

```json
"formatter": "{{.Message}}{{range .Services}} | svc={{.System}} process={{.Process}} method={{.Method}} path={{.Path}} code={{.Code}} status={{.Status}} latency={{.Latency}}ms{{end}}"
```

Salida aproximada:

```text
Request processed successfully | svc=auth-service process=validate-token method=POST path=/auth/validate code=200 status=SUCCESS latency=18ms | svc=customer-core process=get-profile method=GET path=/customers/profile code=200 status=SUCCESS latency=32ms
```

### 7) Imprimir satélites con trace individual

```json
"formatter": "{{range .Services}}[trace={{.IdTrace}}] {{.System}} {{.Process}} status={{.Status}} latency={{.Latency}}ms {{end}}"
```

Salida aproximada:

```text
[trace=sat-001] auth-service validate-token status=SUCCESS latency=12ms [trace=sat-002] score-engine calculate-score status=SUCCESS latency=28ms
```

### 8) JSON compacto personalizado

```json
"formatter": "{\"timestamp\":{{json .Timestamp}},\"traceID\":{{json .TraceID}},\"message\":{{json .Message}},\"details\":{\"system\":{{json .Details.System}},\"service\":{{json (printf \"%s %s\" .Details.Method .Details.Path | trim)}}}}"
```

Salida aproximada:

```json
{"timestamp":"2026-03-13T01:10:23.123","traceID":"main-trace-001","message":"Loan simulation completed","details":{"system":"loan-service","service":"POST /loan/simulate"}}
```

---

## Recomendaciones de uso

### Usa `json` cuando:

- quieras observabilidad estructurada
- necesites búsquedas por campo
- el destino del log sea Kibana, Loki, CloudWatch o Elastic
- quieras ver también `details` y `services` completos
- quieras una salida compacta en una sola línea
- quieras omitir automáticamente nulos y vacíos

### Usa `text` cuando:

- quieras leer el log fácilmente en consola
- estés en desarrollo local
- necesites una salida rápida y compacta

### Usa personalización cuando:

- quieras adaptar el log a un formato legado
- solo necesites imprimir ciertos campos
- quieras mezclar texto libre con bloques JSON
- quieras incluir los consumos o satélites de `Services` en una sola línea
- quieras cambiar la estructura del JSON por defecto

---

## Consideraciones importantes

- `json` es la opción recomendada para observabilidad estructurada
- `text` es la opción más cómoda para debugging local
- la personalización es la mejor opción cuando necesitas una salida híbrida o más compacta
- si necesitas explotar el log por campos, conviene preferir `json`
- si necesitas legibilidad inmediata en consola, conviene preferir `text`
- si necesitas cambiar la estructura del JSON, debes usar un template JSON completo diferente al predeterminado

---

# logger.formatDate

Define el formato de fecha utilizado en los logs.

```json
"formatDate": "2006-01-02T15:04:05.000"
```

---

# Resumen

| Sección | Propósito |
|------|------|
| `app` | Información del servicio |
| `server` | Configuración del servidor HTTP |
| `gin` | Configuración de middlewares y logging HTTP de Gin |
| `server.gin.LoggerWithConfig` | Configuración del middleware LoggerWithConfig de Gin |
| `logger` | Configuración del sistema de logs |
| `logger.level` | Nivel mínimo de logs |
| `logger.ignoredHeaders` | Headers que no deben registrarse |
| `logger.rotate` | Rotación automática de logs |
| `logger.formatter` | Formato de salida del log y personalización en JSON, texto y templates |
| `logger.formatDate` | Formato de fecha |

---

# Comandos útiles

### Actualización de dependencias

```bash
go get -u ./...
```

### Liberar caché de compilación, test unitarios o gomods

```bash
go clean -cache -testcache -modcache
```

### Ejecutar test unitarios

```bash
go test -cover -covermode=atomic -coverprofile="coverage.out" ./...
```

### Generar coverage html de test unitarios

```bash
go tool cover -html="coverage.out" -o "coverage.html"
```

### Generar coverage `.out` de test unitarios

```bash
go tool cover -func="coverage.out"
```

