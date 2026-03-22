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

## Cómo encajan las dependencias

Flujo típico de una aplicación QuicksGo:

1. `config` carga `application.yml` o `application.json` con `viper`
2. `config` inicializa `logger`
3. `config` inicializa tracing OTEL
4. `server_Gin` o `server_gRPC` arrancan el servidor
5. `security` usa la misma configuración ya cargada en `viper`
6. `clientHttp` y `client_gRPC` reutilizan tracing y logging para llamadas salientes

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

## Servidor HTTP

Para arrancar un servidor Gin con QuicksGo, normalmente usas `config/server_Gin`:

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

Para arrancar un servidor gRPC con QuicksGo, normalmente usas `config/server_gRPC`:

```go
srv := serverGRPC.NewIUnitary(nil, nil)

if err := srv.Register(func(r grpc.ServiceRegistrar) {
	pb.RegisterGreeterServer(r, greeterServer{})
}); err != nil {
	panic(err)
}

panic(srv.Serve())
```

## Ejemplo ejecutable

El módulo `config` incluye un ejemplo runnable en [main.go](/e:/Proyects/Practices/QuicksGoV2t/config/main.go).

Gin:

```powershell
cd e:\Proyects\Practices\QuicksGoV2t\config
$env:QUICKSGO_EXAMPLE_SERVER="gin"
go run .
```

gRPC:

```powershell
cd e:\Proyects\Practices\QuicksGoV2t\config
$env:QUICKSGO_EXAMPLE_SERVER="grpc"
go run .
```

## Recomendación de uso

Si vas a empezar una aplicación nueva con QuicksGo:

1. toma como base [application.yml](/e:/Proyects/Practices/QuicksGoV2t/config/application.yml)
2. carga configuración con `config/utilities.LoadEnv`
3. usa `server_Gin` o `server_gRPC` como bootstrap
4. define tus rutas o servicios protobuf
5. usa `security` para JWT y protección de endpoints
6. usa `clientHttp` o `client_gRPC` para llamadas salientes con tracing

## Comandos útiles

Ejecutar tests:

```bash
go test ./...
```

Coverage:

```bash
go test -cover -covermode=atomic -coverprofile="coverage.out" ./...
go tool cover -html="coverage.out" -o "coverage.html"
```
