# GoForge Security

`security` provee servicios JWT y middleware Gin para autenticacion basada en
tokens. Usa `viper` para servicios configurados y depende de
`github.com/PointerByte/GoForge/encrypt` para helpers criptograficos.

## Instalacion

```bash
go get github.com/PointerByte/GoForge/security
```

Si tu aplicacion tambien necesita operaciones criptograficas directas, agrega:

```bash
go get github.com/PointerByte/GoForge/encrypt
```

## Paquetes

- `auth/jwt`: creacion JWT, validacion de firma, lectura de claims y estrategias de firma
- `auth/cookies`: validacion JWT desde cookies HTTP
- `middlewares`: middleware Gin para bearer tokens, cookies y headers de seguridad

## Capacidades

- crear JWTs desde claims arbitrarios
- validar firmas y algoritmos de JWT compactos
- decodificar claims en `map[string]any` o structs tipados
- agregar validadores a nivel servicio y por llamada
- usar contextos de request y timeouts a nivel servicio
- proteger rutas Gin mediante bearer token o cookie JWT
- aplicar headers HTTP comunes de seguridad
- conectar estrategias de firma custom

## Configuracion

Este modulo no carga automaticamente `application.yaml`, `application.yml` ni
`application.json`. Carga configuracion en `viper` antes de usar
`NewConfiguredService`, `RequireJWT` o `RequireJWTCookie`.

```yaml
jwt:
  enable: true
  cookie:
    name: access_token
  eddsa:
    private_key: ./certs/jwt/ed25519-key.pem
    public_key: ./certs/jwt/ed25519-public.pem
```

Claves principales:

- `jwt.enable`: cuando esta explicitamente en `false`, el middleware JWT de Gin deja pasar requests
- `jwt.algorithm`: `HS256`, `RS256`, `PS256` o `EdDSA`; es opcional cuando solo hay una estrategia configurada
- `jwt.cookie.name`: nombre de cookie usado por auth con cookies; default `access_token`
- `jwt.hmac.secret`: secreto compartido para `HS256`
- `jwt.rsa.private_key`: valor de llave privada RSA o ruta a archivo PEM
- `jwt.rsa.public_key`: valor de llave publica RSA o ruta a archivo PEM
- `jwt.eddsa.private_key`: valor de llave privada Ed25519 o ruta a archivo PEM
- `jwt.eddsa.public_key`: valor de llave publica Ed25519 o ruta a archivo PEM

Configura una sola estrategia por servicio. Si hay mas de una estrategia bajo
`jwt`, define `jwt.algorithm` para resolver la ambiguedad.

Los archivos `application.yaml` y `application.json` de este modulo son ejemplos
completos: incluyen `hmac`, `rsa` y `eddsa` para documentar las opciones. Como
hay varias estrategias configuradas, tambien incluyen `jwt.algorithm`.

Los inputs configurados reciben nombres de claves de viper, no valores crudos
de secretos. Por ejemplo, `HMACSecretKey` apunta a la clave de viper donde vive
el secreto HS256. Usa `jwtservice.New(jwtservice.WithHMACSHA256("secret"))`
cuando quieras pasar un secreto directamente.

Archivos de ejemplo:

- [application.yaml](./application.yaml)
- [application.json](./application.json)

## Servicio JWT

### Configurado Desde Viper

Con una sola estrategia configurada, `NewConfiguredService` puede inferir el
algoritmo:

```go
viper.Set("jwt.eddsa.private_key", "./certs/jwt/ed25519-key.pem")
viper.Set("jwt.eddsa.public_key", "./certs/jwt/ed25519-public.pem")

service, err := jwtservice.NewConfiguredService(jwtservice.ConfigServiceInput{})
if err != nil {
	panic(err)
}
```

Si configuras varias estrategias, define `jwt.algorithm` o pasalo en
`ConfigServiceInput`:

```go
package main

import (
	jwtservice "github.com/PointerByte/GoForge/security/auth/jwt"
	"github.com/spf13/viper"
)

func main() {
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

	token, err := service.Create(map[string]any{"user_id": "42"})
	if err != nil {
		panic(err)
	}

	var claims map[string]any
	if err := service.Read(token, &claims); err != nil {
		panic(err)
	}
}
```

### Secreto Directo

```go
service, err := jwtservice.New(
	jwtservice.WithHMACSHA256("my-secret"),
)
if err != nil {
	panic(err)
}
```

### Contexto Y Validadores

```go
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

var claims struct {
	UserID string `json:"user_id"`
}

parsedToken, err := service.Decode(ctx, token, &claims)
if err != nil {
	panic(err)
}

_ = parsedToken
```

Usa `ValidateSignatureWithContext(ctx, token)` cuando solo necesites verificar
la estructura, algoritmo y firma del JWT sin decodificar claims.

## Algoritmos Soportados

- `HS256`: HMAC-SHA256
- `RS256`: RSA SHA-256
- `PS256`: RSA-PSS SHA-256
- `EdDSA`: Ed25519

Las llaves RSA y Ed25519 configuradas pueden ser rutas PEM o valores
codificados soportados.

## Middleware Bearer Para Gin

`RequireJWT` lee un bearer token desde el header `Authorization`, lo valida y
guarda el token parseado y los claims en el contexto Gin.

```go
package main

import (
	"context"

	jwtservice "github.com/PointerByte/GoForge/security/auth/jwt"
	"github.com/PointerByte/GoForge/security/middlewares"
	"github.com/gin-gonic/gin"
)

type MyClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

func main() {
	router := gin.New()
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
}
```

Leer valores desde el contexto Gin:

```go
claimsValue, ok := c.Get(middlewares.JWTClaimsContextKey.String())
if !ok {
	return
}

claims := claimsValue.(*MyClaims)
_ = claims
```

El token parseado se guarda con `middlewares.JWTTokenContextKey.String()`. Sin
claims factory, los claims decodificados se guardan como `map[string]any`.
Personaliza las claves con `WithJWTContextKeys`.

## Auth Por Cookies

El paquete `auth/cookies` valida JWTs almacenados en una cookie HTTP.

```go
import (
	cookiesauth "github.com/PointerByte/GoForge/security/auth/cookies"
	jwtservice "github.com/PointerByte/GoForge/security/auth/jwt"
)

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

var claims map[string]any
if err := service.Read(request, &claims); err != nil {
	panic(err)
}
```

Middleware Gin con cookies:

```go
router.Use(middlewares.RequireJWTCookie(
	middlewares.WithJWTCookieServiceConfig(cookiesauth.ConfigServiceInput{
		CookieNameKey: cookiesauth.DefaultCookieNameKey,
		JWT: jwtservice.ConfigServiceInput{
			Algorithm:     "HS256",
			HMACSecretKey: &hmacSecretKey,
		},
	}),
	middlewares.WithJWTCookieClaimsFactory(func() any { return &MyClaims{} }),
))
```

Lee `jwt.cookie.name` desde viper y usa `access_token` como fallback.

## Estrategias Custom

Usa `WithCustomStrategy` directamente o reemplaza la creacion del servicio en
middleware con `WithJWTServiceFactory`.

```go
service, err := jwtservice.New(
	jwtservice.WithCustomStrategy("CUSTOM", signFunc, verifyFunc),
)
if err != nil {
	panic(err)
}

_ = service
```

## Headers De Seguridad

`middlewares.SecurityHeaders()` agrega headers comunes como `X-Frame-Options`,
`Content-Security-Policy`, `Strict-Transport-Security`, `Referrer-Policy`,
`X-Content-Type-Options` y `Permissions-Policy`.

```go
router.Use(middlewares.SecurityHeaders())
```

## Relacion Con `encrypt`

`encrypt` es un modulo separado. `security` lo usa internamente, pero la ruta
publica para criptografia es:

```go
github.com/PointerByte/GoForge/encrypt
```

Usa `encrypt` directamente cuando tu aplicacion necesite AES, hashing, RSA/ECC,
KMS o helpers de firma fuera de auth JWT.

## Ejemplo Ejecutable

Este modulo incluye una app de ejemplo en [main.go](./main.go).

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
go test -cover -covermode=atomic -coverprofile=coverage.out ./...
```
