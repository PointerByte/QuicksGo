# QuicksGo Security

`security` provee servicios JWT y middlewares para Gin orientados a autenticacion basada en tokens. Esta disenado para trabajar con configuracion cargada en `viper` y usa `github.com/PointerByte/QuicksGo/encrypt` como dependencia criptografica independiente.

## Instalacion

```bash
go get github.com/PointerByte/QuicksGo/security
```

Si tambien necesitas acceso directo a primitivas criptograficas, agrega:

```bash
go get github.com/PointerByte/QuicksGo/encrypt
```

## Que incluye este modulo

- Creacion, validacion y lectura de claims JWT
- Autenticacion JWT basada en cookies
- Middlewares para Gin con JWT y cookies
- Middleware de headers de seguridad
- Aplicacion de ejemplo con flujos HMAC y RSA

## Estructura de paquetes

- `auth/jwt`: servicios JWT y configuracion de estrategias de firma
- `auth/cookies`: servicio JWT desde cookies
- `middlewares`: helpers de middleware para Gin

## Relacion con `encrypt`

`encrypt` ahora es un modulo separado. `security` depende de el internamente para operaciones criptograficas, pero la ruta publica ya no es `github.com/PointerByte/QuicksGo/security/encrypt`.

Usa estas rutas de modulo:

- `github.com/PointerByte/QuicksGo/security`
- `github.com/PointerByte/QuicksGo/encrypt`

## Configuracion con Viper

Los servicios JWT y cookie-auth resuelven configuracion mediante `viper`.

Este modulo no carga automaticamente `application.yaml`, `application.yml` ni `application.json`. Tu aplicacion debe cargar la configuracion en `viper` antes de crear un servicio configurado o usar `RequireJWT` / `RequireJWTCookie`.

Ejemplo cargando YAML:

```go
import "github.com/spf13/viper"

func loadConfig() error {
	viper.SetConfigName("application")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	return viper.ReadInConfig()
}
```

Ejemplo cargando JSON:

```go
import "github.com/spf13/viper"

func loadConfig() error {
	viper.SetConfigName("application")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")
	return viper.ReadInConfig()
}
```

Claves principales:

- `jwt.enable`
- `jwt.algorithm`
- `jwt.cookie.name`
- `jwt.hmac.secret`
- `jwt.rsa.private_key`
- `jwt.rsa.public_key`
- `jwt.eddsa.private_key`
- `jwt.eddsa.public_key`

Archivos de ejemplo incluidos en este modulo:

- [application.yaml](./application.yaml)
- [application.json](./application.json)

## Uso de JWT

### Crear un servicio configurado

```go
import (
	jwtservice "github.com/PointerByte/QuicksGo/security/auth/jwt"
	"github.com/spf13/viper"
)

viper.Set("jwt.algorithm", "HS256")
viper.Set("jwt.hmac.secret", "my-secret")

service, err := jwtservice.NewConfiguredService(jwtservice.ConfigServiceInput{})
if err != nil {
	panic(err)
}
```

### Crear un token

```go
token, err := service.Create(map[string]any{
	"user_id": "42",
	"role":    "admin",
})
if err != nil {
	panic(err)
}
```

### Leer claims tipados

```go
var claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

if err := service.Read(token, &claims); err != nil {
	panic(err)
}
```

### Algoritmos soportados

- `HS256`
- `RS256`
- `PS256`
- `EdDSA`

## Middleware JWT para Gin

`RequireJWT` construye el servicio JWT internamente usando `viper`.

```go
import (
	"context"

	jwtservice "github.com/PointerByte/QuicksGo/security/auth/jwt"
	"github.com/PointerByte/QuicksGo/security/middlewares"
)

router.Use(middlewares.RequireJWT(
	middlewares.WithJWTClaimsFactory(func() any { return &MyClaims{} }),
	middlewares.WithJWTValidator(func(ctx context.Context, token jwtservice.Token) error {
		return nil
	}),
))
```

### Leer claims desde el contexto de Gin

```go
type MyClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

claimsValue, _ := c.Get(middlewares.JWTClaimsContextKey.String())
claims := claimsValue.(*MyClaims)
```

## Auth por cookies

El paquete `auth/cookies` reutiliza el servicio JWT y lee el token desde una cookie HTTP.

### Crear un servicio configurado por cookies

```go
import (
	cookiesauth "github.com/PointerByte/QuicksGo/security/auth/cookies"
	"github.com/spf13/viper"
)

viper.Set("jwt.algorithm", "HS256")
viper.Set("jwt.hmac.secret", "my-secret")
viper.Set("jwt.cookie.name", "session_token")

service, err := cookiesauth.NewConfiguredService(cookiesauth.ConfigServiceInput{})
if err != nil {
	panic(err)
}
```

### Leer claims desde la cookie del request

```go
var claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

if err := service.Read(r, &claims); err != nil {
	panic(err)
}
```

## Middleware de cookies para Gin

```go
router.Use(middlewares.RequireJWTCookie(
	middlewares.WithJWTCookieClaimsFactory(func() any { return &MyClaims{} }),
))
```

Por defecto lee la cookie configurada en `jwt.cookie.name`, o `access_token` cuando esa clave no existe.

## Uso directo de `encrypt` junto con `security`

Si tu aplicacion usa `security` y tambien necesita operaciones criptograficas explicitas, importa `encrypt` directamente:

```go
import (
	"context"

	"github.com/PointerByte/QuicksGo/encrypt"
	"github.com/PointerByte/QuicksGo/encrypt/local"
)

ctx := context.Background()
repository := encrypt.NewRepository(local.NewRepository())

_, _ = ctx, repository
```

Consulta el README del modulo `encrypt` para detalles especificos de cada backend.

## Ejemplo ejecutable

Este modulo incluye un ejemplo ejecutable en [main.go](./main.go).

Ejecutalo desde el directorio `security`:

```bash
go run .
```

Rutas de ejemplo:

- `GET /health`
- `POST /hmac/login`
- `GET /hmac/api/me`
- `GET /hmac/api/admin`
- `POST /rsa/login`
- `GET /rsa/api/me`
- `GET /rsa/api/admin`

## Pruebas

Desde el directorio del modulo `security`:

```bash
go test ./...
```

Con cobertura:

```bash
go test -cover -covermode=atomic -coverprofile="coverage.out" ./...
```

## Comandos utiles

Actualizar dependencias:

```bash
go get -u ./...
```

Limpiar cache de compilacion, pruebas y modulos:

```bash
go clean -cache -testcache -modcache
```
