# QuicksGo

QuicksGo es un framework modular para construir servicios Go con:

- servidor HTTP con Gin
- servidor gRPC
- clientes HTTP y gRPC
- logging estructurado
- tracing con OpenTelemetry
- seguridad con JWT y middlewares
- carga de configuración con `viper`

## Módulos principales

- [config](/e:/Proyects/Practices/QuicksGoV2t/config/README.es.md): bootstrap de servidores, clientes y utilidades de configuración/tracing
- [logger](/e:/Proyects/Practices/QuicksGoV2t/logger/README.es.md): logging estructurado, middlewares HTTP/gRPC y trazas satélite
- [security](/e:/Proyects/Practices/QuicksGoV2t/security/README.es.md): JWT, middlewares de seguridad y utilidades criptográficas
- [cmd/qgo](/e:/Proyects/Practices/QuicksGo/cmd/README.es.md): CLI para generar servicios nuevos con Gin y gRPC

## Cómo encajan las dependencias

Flujo típico de una aplicación QuicksGo:

1. `config` carga `application.yml` o `application.json` con `viper`
2. `config` inicializa `logger`
3. `config` inicializa tracing OTEL
4. `server/gin` o `server/grpc` arrancan el servidor
5. `security` usa la misma configuración ya cargada en `viper`
6. `client/http` y `client/grpc` reutilizan tracing y logging para llamadas salientes

## Plantilla de configuración

La plantilla completa del framework quedó en:

- [application.yml](/e:/Proyects/Practices/QuicksGoV2t/config/application.yml)
- [application.json](/e:/Proyects/Practices/QuicksGoV2t/config/application.json)

Incluye configuración para:

- `app.*`
- `server.gin.*`
- `server.gin.LoggerWithConfig.*`
- `server.grpc.*`
- `gin.autotls.*`
- `client.grpc.*`
- `logger.*`
- `traces.SkipPaths`
- `jwt.*`

## Formato recomendado

Aunque también existe template en JSON, el formato recomendado para nuevas aplicaciones es YAML.

El loader actual de `config` prioriza:

1. `application.yml`
2. `application.json`
3. `.env`
4. `.env.local`
5. variables de entorno

Si vas a usar el template YAML del framework, úsalo como base para tu `application.yml`.

## Variables de entorno

QuicksGo puede sobreescribir configuración cargada desde archivo usando nombres derivados de la ruta de cada clave.

Ejemplos:

- `app.name` -> `APP_NAME`
- `server.gin.port` -> `SERVER_GIN_PORT`
- `server.grpc.port` -> `SERVER_GRPC_PORT`
- `client.grpc.tls.serverName` -> `CLIENT_GRPC_TLS_SERVERNAME`
- `jwt.hmac.secret` -> `JWT_HMAC_SECRET`

## Observabilidad

Los servicios generados y levantados con QuicksGo ya vienen preparados para
observabilidad usando el SDK de OpenTelemetry.

Eso incluye soporte para:

- trazas
- logs
- metricas

QuicksGo tambien es compatible con Go Auto Instrumentation SDK de
OpenTelemetry cuando tu despliegue requiere instrumentacion automatica encima
de la configuracion que ya provee el framework.


## Servidor HTTP

Para arrancar un servidor Gin con QuicksGo, normalmente usas `config/server/gin`:

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

## Servidor gRPC

Para arrancar un servidor gRPC con QuicksGo, normalmente usas `config/server/grpc`:

```go
srv := serverGRPC.NewIConfig(nil, nil)

if err := srv.Register(func(r grpc.ServiceRegistrar) {
	pb.RegisterGreeterServer(r, greeterServer{})
}); err != nil {
	panic(err)
}

panic(srv.Serve())
```

## Recomendación de uso

Si vas a empezar una aplicación nueva con QuicksGo:

1. toma como base [application.yml](/e:/Proyects/Practices/QuicksGoV2t/config/application.yml)
2. carga configuración con `config/utilities.LoadEnv`
3. usa `server/gin` o `server/grpc` como bootstrap
4. define tus rutas o servicios protobuf
5. usa `security` para JWT y protección de endpoints
6. usa `client/http` o `client/grpc` para llamadas salientes con tracing

Tambien puedes generar un servicio nuevo con `qgo`:

```bash
go install github.com/PointerByte/QuicksGo/cmd/qgo@latest
qgo new gin
qgo new grpc
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

