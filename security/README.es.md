# QuicksGo Security

Biblioteca de seguridad para Go con utilidades de:

- JWT con `viper`
- Middlewares para Gin
- Cifrado simetrico
- RSA
- Firmas digitales

## Instalacion

```bash
go get github.com/PointerByte/QuicksGo/security
```

## Paquetes

- `auth/jwt`: creacion, validacion y lectura de JWT
- `auth/cookies`: validacion y lectura de JWT desde cookies HTTP
- `middlewares`: middlewares para Gin (`RequireJWT`, `RequireJWTCookie`, headers de seguridad)
- `encrypt`: API por repositorios con `context.Context` para AES-GCM, HMAC, hashes, RSA y firmas digitales
- `encrypt/local`: implementacion local con material criptografico exportable
- `encrypt/aws-kms`: implementacion orientada a AWS KMS
- `encrypt/azure-key-vault`: implementacion orientada a Azure Key Vault con fallbacks locales
- `encrypt/gcp-kms`: implementacion orientada a Google Cloud KMS con fallbacks locales

## Configuracion con Viper

Los paquetes JWT y auth por cookies usan `viper` para resolver configuracion.

`security` no carga por si mismo `application.yaml`, `application.yml` ni `application.json`. La aplicacion host debe cargar uno de esos archivos en `viper` antes de crear el servicio JWT o de usar `RequireJWT` / `RequireJWTCookie`.

En este repositorio, por ejemplo, el paquete `server` carga la configuracion desde la raiz de la aplicacion con esta prioridad:

- `application.yml`
- `application.json`

Eso significa que valores como `jwt.enable` o `jwt.algorithm` salen del archivo que tu aplicacion cargo primero, y despues pueden ser sobreescritos por variables de entorno si tu bootstrap lo hace.

Ejemplo cargando `application.yaml` o `application.yml`:

```go
import "github.com/spf13/viper"

func loadConfig() error {
	viper.SetConfigName("application")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	return viper.ReadInConfig()
}
```

Ejemplo cargando `application.json`:

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

Ejemplos incluidos:

- [config.example.yaml](/e:/Proyects/Practices/QuicksGoV2t/security/config.example.yaml)
- [config.example.json](/e:/Proyects/Practices/QuicksGoV2t/security/config.example.json)

## Uso de JWT

### Crear un servicio desde configuracion

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
```

### Validar y leer claims

```go
var claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

err := service.Read(token, &claims)
```

### Algoritmos soportados en JWT

- `HS256`
- `RS256`
- `PS256`
- `EdDSA`

## Middleware JWT para Gin

`RequireJWT` construye internamente el servicio usando `viper`.

```go
router.Use(middlewares.RequireJWT(
	middlewares.WithJWTClaimsFactory(func() any { return &MyClaims{} }),
	middlewares.WithJWTValidator(func(ctx context.Context, token jwtservice.Token) error {
		return nil
	}),
))
```

### Claims tipados

```go
type MyClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}
```

Luego en el handler:

```go
claimsValue, _ := c.Get(middlewares.JWTClaimsContextKey.String())
claims := claimsValue.(*MyClaims)
```

## Auth por cookies

El paquete `auth/cookies` reutiliza el servicio JWT y lee el token desde una cookie HTTP.

### Crear un servicio desde configuracion

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

### Validar claims desde la cookie del request

```go
var claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

err := service.Read(r, &claims)
```

## Middleware JWT por cookies para Gin

`RequireJWTCookie` valida el JWT desde una cookie del request y guarda el token parseado y los claims en el contexto de Gin.

```go
router.Use(middlewares.RequireJWTCookie(
	middlewares.WithJWTCookieClaimsFactory(func() any { return &MyClaims{} }),
))
```

Por defecto lee la cookie configurada en `jwt.cookie.name`, o `access_token` cuando esa clave no esta definida.

## Uso de Encrypt

El modulo de cifrado expone ahora una API por repositorios desde `encrypt`.

`encrypt.NewRepository` ahora recibe el modo del backend de forma explicita.

### Crear un repositorio

```go
import (
	"context"

	"github.com/PointerByte/QuicksGo/security/encrypt"
)

ctx := context.Background()
repository := encrypt.NewRepository(encrypt.Local)
```

### AES-GCM

```go
keyData, err := repository.GeneratesSymetrycKey(ctx, 32)
if err != nil {
	panic(err)
}

additional := "aad"
encrypted, err := repository.EncryptAES(ctx, keyData.Key, "hello", &additional)
if err != nil {
	panic(err)
}

plainText, err := repository.DecryptAES(ctx, keyData.Key, encrypted, &additional)
```

### HMAC

```go
hash := repository.GenerateHMAC(ctx, "secret", "message")
ok := repository.ValidateHMAC(ctx, "secret", "message", hash)
```

### RSA-OAEP con SHA-256

```go
keyData, err := repository.GeneratesRSAKey(ctx, 2048)
if err != nil {
	panic(err)
}

cipherText, err := repository.RSA_OAEP_Encode(ctx, keyData.PublicKey, "hello")
if err != nil {
	panic(err)
}

plainText, err := repository.RSA_OAEP_Decode(ctx, keyData.PrivateKey, cipherText)
```

### Ed25519

```go
keyData, err := repository.GeneratesEd255Key(ctx, 2048)
if err != nil {
	panic(err)
}

signature, err := repository.SignEd25519(ctx, keyData.PrivateKey, "hello")
if err != nil {
	panic(err)
}

err = repository.VerifyEd25519(ctx, keyData.PublicKey, "hello", signature)
```

## Ejemplo ejecutable

El proyecto incluye un ejemplo con Gin en [main.go](/e:/Proyects/Practices/QuicksGoV2t/security/main.go).

Ejecutar:

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

```bash
go test ./...
```

## Comandos utiles

### Actualizar dependencias

Actualiza las dependencias del modulo a versiones mas recientes permitidas.

```bash
go get -u ./...
```

### Limpiar cache de compilacion, pruebas y modulos

Elimina la cache de build, la cache de tests y la cache de modulos descargados.

```bash
go clean -cache -testcache -modcache
```

### Ejecutar pruebas unitarias con coverage

Ejecuta todos los tests del proyecto y genera el archivo `coverage.out`.

```bash
go test -cover -covermode=atomic -coverprofile="coverage.out" ./...
```

### Generar reporte HTML de coverage

Convierte `coverage.out` en un reporte visual HTML.

```bash
go tool cover -html="coverage.out" -o "coverage.html"
```

### Mostrar coverage desde `coverage.out`

Imprime en consola el porcentaje de coverage por funcion.

```bash
go tool cover -func="coverage.out"
```
