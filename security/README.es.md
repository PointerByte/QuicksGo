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

Los inputs configurados reciben los nombres de claves de `viper`, no los
valores de secretos o llaves directamente. Por ejemplo, `HMACSecretKey` es un
`*string` que apunta a la clave de `viper` donde vive el secreto HS256. Si
quieres pasar el secreto directamente, crea el servicio con
`jwtservice.New(jwtservice.WithHMACSHA256("secret"))`.
El mismo patron de puntero a clave aplica a `RSAPrivateKeyKey`,
`RSAPublicKeyKey`, `EdDSAPrivateKeyKey` y `EdDSAPublicKeyKey`.

```go
func stringPtr(value string) *string {
	return &value
}
```

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

hmacSecretKey := jwtservice.DefaultHMACSecretKey

service, err := jwtservice.NewConfiguredService(jwtservice.ConfigServiceInput{
	Algorithm:     "HS256",
	HMACSecretKey: &hmacSecretKey,
})
if err != nil {
	panic(err)
}
```

Para usar un secreto directo sin `viper`:

```go
service, err := jwtservice.New(
	jwtservice.WithHMACSHA256("my-secret"),
)
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

### Operaciones con contexto

La firma, validacion de firma, lectura de claims y validadores pueden recibir
un `context.Context`. Tambien puedes configurar un timeout a nivel del servicio.

```go
import (
	"context"
	"time"

	jwtservice "github.com/PointerByte/QuicksGo/security/auth/jwt"
)

service, err := jwtservice.New(
	jwtservice.WithHMACSHA256("my-secret"),
	jwtservice.WithContextTimeout(2*time.Second),
	jwtservice.WithValidator(func(ctx context.Context, token jwtservice.Token) error {
		return nil
	}),
)
if err != nil {
	panic(err)
}

ctx := context.Background()

token, err := service.CreateWithContext(ctx, map[string]any{"user_id": "42"})
if err != nil {
	panic(err)
}

var claims map[string]any
parsedToken, err := service.Decode(ctx, token, &claims, func(ctx context.Context, token jwtservice.Token) error {
	return nil
})
if err != nil {
	panic(err)
}

_ = parsedToken
```

Usa `ValidateSignatureWithContext(ctx, token)` cuando solo necesitas validar
la firma y el algoritmo del token sin decodificar claims.

### Algoritmos soportados

- `HS256`
- `RS256`
- `PS256`
- `EdDSA`

## Middleware JWT para Gin

`RequireJWT` construye el servicio JWT internamente. Usa
`WithJWTServiceConfig` para pasar el input del servicio configurado,
`WithJWTClaimsFactory` para decodificar en un struct tipado y
`WithJWTValidator` para validaciones posteriores a la firma. Los validadores
reciben el contexto del request.

```go
import (
	"context"

	jwtservice "github.com/PointerByte/QuicksGo/security/auth/jwt"
	"github.com/PointerByte/QuicksGo/security/middlewares"
)

hmacSecretKey := jwtservice.DefaultHMACSecretKey

router.Use(middlewares.RequireJWT(
	middlewares.WithJWTServiceConfig(jwtservice.ConfigServiceInput{
		Algorithm:     "HS256",
		HMACSecretKey: &hmacSecretKey,
	}),
	middlewares.WithJWTClaimsFactory(func() any { return &MyClaims{} }),
	middlewares.WithJWTValidator(func(ctx context.Context, token jwtservice.Token) error {
		return nil
	}),
))
```

Para estrategias JWT personalizadas, reemplaza el constructor con
`WithJWTServiceFactory`.

```go
router.Use(middlewares.RequireJWT(
	middlewares.WithJWTServiceConfig(jwtservice.ConfigServiceInput{
		Algorithm: "CUSTOM",
	}),
	middlewares.WithJWTServiceFactory(func(input jwtservice.ConfigServiceInput) (*jwtservice.Service, error) {
		return jwtservice.New(
			jwtservice.WithCustomStrategy("CUSTOM", signFunc, verifyFunc),
		)
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

El token parseado se guarda con `middlewares.JWTTokenContextKey.String()`.
Si no configuras un claims factory, los claims decodificados se guardan como
`map[string]any`. Puedes cambiar ambas claves con `WithJWTContextKeys`.

## Auth por cookies

El paquete `auth/cookies` reutiliza el servicio JWT y lee el token desde una cookie HTTP.

### Crear un servicio configurado por cookies

```go
import (
	cookiesauth "github.com/PointerByte/QuicksGo/security/auth/cookies"
	jwtservice "github.com/PointerByte/QuicksGo/security/auth/jwt"
	"github.com/spf13/viper"
)

viper.Set("jwt.algorithm", "HS256")
viper.Set("jwt.hmac.secret", "my-secret")
viper.Set("jwt.cookie.name", "session_token")

hmacSecretKey := jwtservice.DefaultHMACSecretKey

service, err := cookiesauth.NewConfiguredService(cookiesauth.ConfigServiceInput{
	CookieNameKey: cookiesauth.DefaultCookieNameKey,
	JWT: jwtservice.ConfigServiceInput{
		Algorithm:     "HS256",
		HMACSecretKey: &hmacSecretKey,
	},
})
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

Para validacion usando el contexto del request, usa `Decode`.

```go
parsedToken, err := service.Decode(r.Context(), r, &claims, func(ctx context.Context, token jwtservice.Token) error {
	return nil
})
if err != nil {
	panic(err)
}

_ = parsedToken
```

## Middleware de cookies para Gin

```go
import (
	"context"

	cookiesauth "github.com/PointerByte/QuicksGo/security/auth/cookies"
	jwtservice "github.com/PointerByte/QuicksGo/security/auth/jwt"
	"github.com/PointerByte/QuicksGo/security/middlewares"
)

hmacSecretKey := jwtservice.DefaultHMACSecretKey

router.Use(middlewares.RequireJWTCookie(
	middlewares.WithJWTCookieServiceConfig(cookiesauth.ConfigServiceInput{
		CookieNameKey: cookiesauth.DefaultCookieNameKey,
		JWT: jwtservice.ConfigServiceInput{
			Algorithm:     "HS256",
			HMACSecretKey: &hmacSecretKey,
		},
	}),
	middlewares.WithJWTCookieClaimsFactory(func() any { return &MyClaims{} }),
	middlewares.WithJWTCookieValidator(func(ctx context.Context, token jwtservice.Token) error {
		return nil
	}),
))
```

Por defecto lee la cookie configurada en `jwt.cookie.name`, o `access_token` cuando esa clave no existe.
Igual que el middleware bearer, guarda el token parseado y los claims
decodificados en el contexto de Gin. Puedes personalizar las claves con
`WithJWTCookieContextKeys`.

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
- `POST /custom/login`
- `GET /custom/api/me`
- `GET /custom/api/admin`

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
go get -u=patch ./...
```

Limpiar cache de compilacion, pruebas y modulos:

```bash
go clean -cache -testcache -modcache
```
